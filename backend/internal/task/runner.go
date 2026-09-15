package task

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// JobHandler 任务处理器：由业务模块注册，payload 为入队时的 JSON。
// 返回 error 时 worker 按退避策略重试；处理器内部必须自行保证业务幂等（评估/报告均为 Upsert）。
type JobHandler func(ctx context.Context, payload json.RawMessage) error

// Config worker 运行参数
type Config struct {
	Workers         int           // 并发 worker 数（bounded worker pool）
	PollInterval    time.Duration // 无任务时的轮询间隔
	LeaseTimeout    time.Duration // 单任务执行租约：超时会被其他 worker 回收重领
	ShutdownTimeout time.Duration // 关闭时等待在途任务的最长时间
}

func (c Config) withDefaults() Config {
	if c.Workers <= 0 {
		c.Workers = 2
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 2 * time.Second
	}
	if c.LeaseTimeout <= 0 {
		c.LeaseTimeout = 5 * time.Minute
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 20 * time.Second
	}
	return c
}

// ErrUnknownHandler 任务类型未注册处理器（属于配置错误，重试到上限后置 failed）
var ErrUnknownHandler = errors.New("no handler registered for task type")

// taskStore runner 需要的任务存储能力（*Repository 实现；测试可替换为 fake）
type taskStore interface {
	ClaimNext(ctx context.Context, workerID string) (*Task, error)
	MarkSucceeded(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id, errMsg string, backoff time.Duration) error
	ReapStale(ctx context.Context, lease time.Duration, limit int) (int64, error)
}

// Runner 轮询 tasks 表并分发执行。
// 生命周期：Start 派生内部 ctx；Shutdown 停止领取新任务并等待在途任务（有界）。
type Runner struct {
	repo     taskStore
	handlers map[string]JobHandler
	cfg      Config
	workerID string
	log      *slog.Logger

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// NewRunner 创建任务运行器
func NewRunner(repo taskStore, cfg Config, workerID string, log *slog.Logger) *Runner {
	return &Runner{
		repo:     repo,
		handlers: make(map[string]JobHandler),
		cfg:      cfg.withDefaults(),
		workerID: workerID,
		log:      log,
	}
}

// Register 注册任务处理器（必须在 Start 前完成）
func (r *Runner) Register(taskType string, h JobHandler) {
	r.handlers[taskType] = h
}

// Start 启动 worker pool 与僵死任务回收器
func (r *Runner) Start(base context.Context) {
	ctx, cancel := context.WithCancel(base)
	r.cancel = cancel

	for i := 0; i < r.cfg.Workers; i++ {
		r.wg.Add(1)
		go r.worker(ctx, i)
	}
	r.wg.Add(1)
	go r.reaper(ctx)

	r.log.Info("task runner started",
		"worker_id", r.workerID, "workers", r.cfg.Workers,
		"poll_interval", r.cfg.PollInterval.String(), "lease_timeout", r.cfg.LeaseTimeout.String())
}

// Shutdown 停止领取新任务，等待在途任务完成或超时
func (r *Runner) Shutdown() {
	if r.cancel == nil {
		return
	}
	r.cancel()
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		r.log.Info("task runner stopped gracefully")
	case <-time.After(r.cfg.ShutdownTimeout):
		r.log.Warn("task runner shutdown timeout, some tasks may still be running",
			"wait", r.cfg.ShutdownTimeout.String())
	}
}

// worker 领取-执行循环
func (r *Runner) worker(ctx context.Context, idx int) {
	defer r.wg.Done()
	for {
		// 先响应关闭，避免关闭后再领取新任务
		select {
		case <-ctx.Done():
			return
		default:
		}

		t, err := r.repo.ClaimNext(ctx, r.workerID)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				r.log.Error("claim task failed", "worker", idx, "error", err)
			}
			if !sleep(ctx, r.cfg.PollInterval) {
				return
			}
			continue
		}
		if t == nil {
			if !sleep(ctx, r.cfg.PollInterval) {
				return
			}
			continue
		}
		r.execute(ctx, t)
	}
}

// execute 执行单个任务。
// 任务 ctx 有意不继承 runner 的取消 ctx：优雅关闭时在途任务可继续执行到自己的租约上限，
// ShutdownTimeout 仅决定主流程等待多久；未完成任务的锁会在租约超时后由 reaper 回收。
func (r *Runner) execute(_ context.Context, t *Task) {
	h, ok := r.handlers[t.Type]
	if !ok {
		r.log.Error("unknown task type", "task_id", t.ID, "type", t.Type)
		_ = r.repo.MarkFailed(context.Background(), t.ID, ErrUnknownHandler.Error(),
			backoffForAttempt(t.Attempts))
		return
	}

	jobCtx, cancel := context.WithTimeout(context.Background(), r.cfg.LeaseTimeout)
	defer cancel()

	if err := h(jobCtx, t.Payload); err != nil {
		r.log.Error("task failed", "task_id", t.ID, "type", t.Type,
			"attempts", t.Attempts, "error", err)
		if merr := r.repo.MarkFailed(context.Background(), t.ID, err.Error(),
			backoffForAttempt(t.Attempts)); merr != nil {
			r.log.Error("mark task failed error", "task_id", t.ID, "error", merr)
		}
		return
	}

	if err := r.repo.MarkSucceeded(context.Background(), t.ID); err != nil {
		r.log.Error("mark task succeeded error", "task_id", t.ID, "error", err)
		return
	}
	r.log.Info("task succeeded", "task_id", t.ID, "type", t.Type, "attempts", t.Attempts)
}

// reaper 定期回收租约超时的 running 任务（worker 崩溃遗留）
func (r *Runner) reaper(ctx context.Context) {
	defer r.wg.Done()
	interval := 30 * r.cfg.PollInterval
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := r.repo.ReapStale(ctx, r.cfg.LeaseTimeout, r.cfg.Workers*2)
			if err != nil && !errors.Is(err, context.Canceled) {
				r.log.Error("reap stale tasks failed", "error", err)
				continue
			}
			if n > 0 {
				r.log.Warn("reaped stale tasks", "count", n, "lease", r.cfg.LeaseTimeout.String())
			}
		}
	}
}

// backoffForAttempt 指数退避：第 N 次尝试失败后等待 base*2^(N-1)，上限 2 分钟
func backoffForAttempt(attempts int) time.Duration {
	const base = 10 * time.Second
	const capBackoff = 2 * time.Minute
	if attempts < 1 {
		attempts = 1
	}
	d := base << (attempts - 1)
	if d <= 0 || d > capBackoff {
		return capBackoff
	}
	return d
}

// sleep 可被 ctx 取消的等待；返回 false 表示已取消
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
