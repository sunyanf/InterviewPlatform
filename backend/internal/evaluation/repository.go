package evaluation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository 评估数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建评估 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Upsert 写入评估结果（session_id 唯一，重跑覆盖）
func (r *Repository) Upsert(ctx context.Context, e *Evaluation) error {
	e.ID = uuid.New().String()
	now := time.Now()
	e.CreatedAt = now
	e.UpdatedAt = now

	dimensionsJSON, _ := json.Marshal(e.Dimensions)
	evidenceJSON, _ := json.Marshal(e.Evidence)
	if e.Evidence == nil {
		evidenceJSON = []byte("[]")
	}
	recsJSON, _ := json.Marshal(e.Recommendations)
	if e.Recommendations == nil {
		recsJSON = []byte("[]")
	}
	rubricJSON, _ := json.Marshal(e.Rubric)

	return r.db.QueryRow(ctx,
		`INSERT INTO evaluations (id, session_id, user_id, dimensions, evidence, recommendations, rubric, total_score, prompt_version, model, created_at, updated_at)
		 VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8, $9, $10, $11, $12)
		 ON CONFLICT (session_id) DO UPDATE SET
		   user_id = EXCLUDED.user_id,
		   dimensions = EXCLUDED.dimensions,
		   evidence = EXCLUDED.evidence,
		   recommendations = EXCLUDED.recommendations,
		   rubric = EXCLUDED.rubric,
		   total_score = EXCLUDED.total_score,
		   prompt_version = EXCLUDED.prompt_version,
		   model = EXCLUDED.model,
		   updated_at = EXCLUDED.updated_at
		 RETURNING id, created_at, updated_at`,
		e.ID, e.SessionID, e.UserID, string(dimensionsJSON), string(evidenceJSON), string(recsJSON),
		string(rubricJSON), e.TotalScore, e.PromptVersion, e.Model, e.CreatedAt, e.UpdatedAt,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

// GetBySessionID 按会话查询评估
func (r *Repository) GetBySessionID(ctx context.Context, sessionID string) (*Evaluation, error) {
	var e Evaluation
	var dimensionsJSON, evidenceJSON, recsJSON, rubricJSON string

	err := r.db.QueryRow(ctx,
		`SELECT id, session_id, user_id, dimensions::text, evidence::text, recommendations::text, rubric::text,
		        total_score, prompt_version, model, created_at, updated_at
		 FROM evaluations WHERE session_id = $1`, sessionID,
	).Scan(&e.ID, &e.SessionID, &e.UserID, &dimensionsJSON, &evidenceJSON, &recsJSON, &rubricJSON,
		&e.TotalScore, &e.PromptVersion, &e.Model, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(dimensionsJSON), &e.Dimensions)
	_ = json.Unmarshal([]byte(evidenceJSON), &e.Evidence)
	_ = json.Unmarshal([]byte(recsJSON), &e.Recommendations)
	_ = json.Unmarshal([]byte(rubricJSON), &e.Rubric)
	return &e, nil
}

// ListByUserID 查询用户的评估列表
func (r *Repository) ListByUserID(ctx context.Context, userID string) ([]Evaluation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, session_id, user_id, dimensions::text, evidence::text, recommendations::text, rubric::text,
		        total_score, prompt_version, model, created_at, updated_at
		 FROM evaluations WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	return scanEvaluations(rows)
}

// ListByUserAndType 查询用户指定面试类型的历史评估（排除指定会话，按时间倒序）
// 用于报告的历史对比（确定性计算），MVP 按 interview_type 对齐（同岗位场次不足时对比更有代表性的是类型）
func (r *Repository) ListByUserAndType(ctx context.Context, userID, interviewType string, excludeSessionID string, limit int) ([]Evaluation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT e.id, e.session_id, e.user_id, e.dimensions::text, e.evidence::text, e.recommendations::text, e.rubric::text,
		        e.total_score, e.prompt_version, e.model, e.created_at, e.updated_at
		 FROM evaluations e
		 JOIN interview_sessions s ON s.id = e.session_id
		 WHERE e.user_id = $1 AND s.interview_type = $2 AND e.session_id <> $3
		 ORDER BY e.created_at DESC
		 LIMIT $4`, userID, interviewType, excludeSessionID, limit)
	if err != nil {
		return nil, err
	}
	return scanEvaluations(rows)
}

// scanEvaluations 通用行扫描
func scanEvaluations(rows pgx.Rows) ([]Evaluation, error) {
	defer rows.Close()

	var list []Evaluation
	for rows.Next() {
		var e Evaluation
		var dimensionsJSON, evidenceJSON, recsJSON, rubricJSON string
		if err := rows.Scan(&e.ID, &e.SessionID, &e.UserID, &dimensionsJSON, &evidenceJSON, &recsJSON, &rubricJSON,
			&e.TotalScore, &e.PromptVersion, &e.Model, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(dimensionsJSON), &e.Dimensions)
		_ = json.Unmarshal([]byte(evidenceJSON), &e.Evidence)
		_ = json.Unmarshal([]byte(recsJSON), &e.Recommendations)
		_ = json.Unmarshal([]byte(rubricJSON), &e.Rubric)
		list = append(list, e)
	}
	return list, rows.Err()
}
