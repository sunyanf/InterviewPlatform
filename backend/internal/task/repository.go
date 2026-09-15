package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository 异步任务数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建任务 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// taskColumns 查询列顺序（scanTask 共用）
const taskColumns = `id, task_type, payload, status, priority, attempts, max_attempts,
	run_after, locked_by, locked_at, COALESCE(last_error,''), COALESCE(idempotency_key,''),
	created_at, updated_at`

func scanTask(row pgx.Row) (*Task, error) {
	var t Task
	err := row.Scan(
		&t.ID, &t.Type, &t.Payload, &t.Status, &t.Priority, &t.Attempts, &t.MaxAttempts,
		&t.RunAfter, &t.LockedBy, &t.LockedAt, &t.LastError, &t.IdempotencyKey,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Enqueue 创建任务。
// 返回 (task, created)：幂等键命中已存在的 pending/running 任务时 created=false，返回原任务。
func (r *Repository) Enqueue(ctx context.Context, req EnqueueRequest) (*Task, bool, error) {
	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, false, err
	}
	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	now := time.Now()
	t := &Task{
		ID:             uuid.New().String(),
		Type:           req.Type,
		Payload:        payload,
		Status:         StatusPending,
		Priority:       req.Priority,
		Attempts:       0,
		MaxAttempts:    maxAttempts,
		RunAfter:       now,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// ON CONFLICT 匹配 0011 迁移中的 partial unique index；冲突时不插入
	row := r.db.QueryRow(ctx,
		`INSERT INTO tasks (id, task_type, payload, status, priority, attempts, max_attempts,
			run_after, idempotency_key, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,0,$6,$7,$8,$9,$9)
		 ON CONFLICT (idempotency_key)
		 WHERE idempotency_key <> '' AND status IN ('pending','running')
		 DO NOTHING
		 RETURNING `+taskColumns,
		t.ID, t.Type, t.Payload, t.Status, t.Priority, t.MaxAttempts,
		t.RunAfter, t.IdempotencyKey, now,
	)
	created, err := scanTask(row)
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	// 幂等冲突：取回已有的 pending/running 任务
	if req.IdempotencyKey == "" {
		// 无幂等键不应出现 DO NOTHING 冲突，属于数据库异常
		return nil, false, err
	}
	existing, gerr := r.getByIdempotencyKey(ctx, req.IdempotencyKey)
	if gerr != nil {
		return nil, false, gerr
	}
	return existing, false, nil
}

func (r *Repository) getByIdempotencyKey(ctx context.Context, key string) (*Task, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+taskColumns+` FROM tasks
		 WHERE idempotency_key = $1 AND status IN ('pending','running')
		 ORDER BY created_at DESC LIMIT 1`, key)
	t, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

// GetByID 按 ID 查询任务
func (r *Repository) GetByID(ctx context.Context, id string) (*Task, error) {
	row := r.db.QueryRow(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = $1`, id)
	t, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

// ClaimNext 认领一个到期的 pending 任务：置 running、attempts+1、加锁。
// FOR UPDATE SKIP LOCKED 保证多 worker/多实例互不抢占；无任务时返回 (nil, nil)。
func (r *Repository) ClaimNext(ctx context.Context, workerID string) (*Task, error) {
	row := r.db.QueryRow(ctx,
		`UPDATE tasks SET status = $1, attempts = attempts + 1, locked_by = $2,
			locked_at = NOW(), updated_at = NOW()
		 WHERE id = (
			SELECT id FROM tasks
			WHERE status = $3 AND run_after <= NOW()
			ORDER BY priority DESC, created_at
			LIMIT 1 FOR UPDATE SKIP LOCKED
		 )
		 RETURNING `+taskColumns,
		StatusRunning, workerID, StatusPending)
	t, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

// MarkSucceeded 任务成功落终态
func (r *Repository) MarkSucceeded(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE tasks SET status = $2, locked_by = '', locked_at = NULL,
			last_error = '', updated_at = NOW()
		 WHERE id = $1`, id, StatusSucceeded)
	return err
}

// MarkFailed 记录失败：未达上限按退避重新排队，达上限置 failed。
// backoff 由调用方按 attempts 计算（纯策略在 retryPolicy，便于单测）。
func (r *Repository) MarkFailed(ctx context.Context, id, errMsg string, backoff time.Duration) error {
	_, err := r.db.Exec(ctx,
		`UPDATE tasks SET
			status = CASE WHEN attempts >= max_attempts THEN $2 ELSE $3 END,
			run_after = CASE WHEN attempts >= max_attempts THEN run_after ELSE NOW() + $4 END,
			last_error = $5, locked_by = '', locked_at = NULL, updated_at = NOW()
		 WHERE id = $1`,
		id, StatusFailed, StatusPending, backoff, truncateError(errMsg))
	return err
}

// ReapStale 回收租约超时的 running 任务（worker 崩溃/实例重启遗留）：
// 重新置 pending 到期立即可被认领；重试次数已在 ClaimNext 累加，不重复计数。
func (r *Repository) ReapStale(ctx context.Context, lease time.Duration, limit int) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE tasks SET status = $1, locked_by = '', locked_at = NULL, updated_at = NOW()
		 WHERE id IN (
			SELECT id FROM tasks
			WHERE status = $2 AND locked_at < $3
			ORDER BY locked_at LIMIT $4
		 )`,
		StatusPending, StatusRunning, time.Now().Add(-lease), limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// truncateError 限制 last_error 长度，避免超长模型报错撑爆字段
func truncateError(s string) string {
	const max = 2000
	if len(s) > max {
		return s[:max]
	}
	return s
}
