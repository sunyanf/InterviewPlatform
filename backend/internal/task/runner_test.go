package task

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBackoffForAttempt(t *testing.T) {
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{0, 10 * time.Second},
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{3, 40 * time.Second},
		{4, 80 * time.Second},
		{7, 120 * time.Second}, // 640s 被截断到 2min 上限
		{-3, 10 * time.Second},
	}
	for _, c := range cases {
		if got := backoffForAttempt(c.attempts); got != c.want {
			t.Errorf("backoffForAttempt(%d) = %v, want %v", c.attempts, got, c.want)
		}
	}
}

func TestConfigWithDefaults(t *testing.T) {
	got := Config{}.withDefaults()
	if got.Workers != 2 || got.PollInterval != 2*time.Second ||
		got.LeaseTimeout != 5*time.Minute || got.ShutdownTimeout != 20*time.Second {
		t.Fatalf("unexpected defaults: %+v", got)
	}

	custom := Config{
		Workers: 5, PollInterval: time.Second,
		LeaseTimeout: time.Minute, ShutdownTimeout: 3 * time.Second,
	}.withDefaults()
	if custom.Workers != 5 || custom.PollInterval != time.Second ||
		custom.LeaseTimeout != time.Minute || custom.ShutdownTimeout != 3*time.Second {
		t.Fatalf("custom values were overwritten: %+v", custom)
	}
}

// fakeStore 内存假任务存储，记录终态写入
type fakeStore struct {
	mu         sync.Mutex
	tasks      []*Task // ClaimNext 依次返回，nil 表示空轮询
	idx        int
	succeeded  []string
	failed     []failedCall
	reapCalled int
}

type failedCall struct {
	id      string
	message string
	backoff time.Duration
}

func (f *fakeStore) ClaimNext(_ context.Context, _ string) (*Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.idx >= len(f.tasks) {
		return nil, nil
	}
	tk := f.tasks[f.idx]
	tk.Status = StatusRunning
	tk.Attempts++
	f.idx++
	return tk, nil
}

func (f *fakeStore) MarkSucceeded(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.succeeded = append(f.succeeded, id)
	return nil
}

func (f *fakeStore) MarkFailed(_ context.Context, id, msg string, backoff time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, failedCall{id: id, message: msg, backoff: backoff})
	return nil
}

func (f *fakeStore) ReapStale(_ context.Context, _ time.Duration, _ int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reapCalled++
	return 0, nil
}

// 执行成功：payload 正确解码并标记 succeeded
func TestExecuteSuccess(t *testing.T) {
	store := &fakeStore{}
	r := NewRunner(store, Config{}, "w-test", testLogger())

	var got ResumeParsePayload
	r.Register(TypeResumeParse, func(_ context.Context, raw json.RawMessage) error {
		return json.Unmarshal(raw, &got)
	})

	payload, _ := json.Marshal(ResumeParsePayload{UserID: "u1", ResumeID: "rs1"})
	r.execute(context.Background(), &Task{ID: "t1", Type: TypeResumeParse, Payload: payload})

	if got.UserID != "u1" || got.ResumeID != "rs1" {
		t.Fatalf("handler payload not decoded: %+v", got)
	}
	if len(store.succeeded) != 1 || store.succeeded[0] != "t1" {
		t.Fatalf("expected t1 succeeded, got %+v", store.succeeded)
	}
	if len(store.failed) != 0 {
		t.Fatalf("unexpected failures: %+v", store.failed)
	}
}

// 执行失败：handler error 透传到 MarkFailed，退避按 attempts 计算
func TestExecuteHandlerError(t *testing.T) {
	store := &fakeStore{}
	r := NewRunner(store, Config{}, "w-test", testLogger())
	handlerErr := errors.New("llm timeout")
	r.Register(TypeSessionEvaluation, func(_ context.Context, _ json.RawMessage) error {
		return handlerErr
	})

	tk := &Task{ID: "t2", Type: TypeSessionEvaluation, Attempts: 2, Payload: json.RawMessage(`{}`)}
	r.execute(context.Background(), tk)

	if len(store.failed) != 1 {
		t.Fatalf("expected 1 failed call, got %d", len(store.failed))
	}
	call := store.failed[0]
	if call.id != "t2" || call.message != "llm timeout" || call.backoff != 20*time.Second {
		t.Fatalf("unexpected failed call: %+v", call)
	}
	if len(store.succeeded) != 0 {
		t.Fatalf("task should not succeed")
	}
}

// 未注册的任务类型：按 ErrUnknownHandler 标记失败
func TestExecuteUnknownType(t *testing.T) {
	store := &fakeStore{}
	r := NewRunner(store, Config{}, "w-test", testLogger())

	r.execute(context.Background(), &Task{ID: "t3", Type: "not_registered", Attempts: 1, Payload: json.RawMessage(`{}`)})

	if len(store.failed) != 1 || store.failed[0].message != ErrUnknownHandler.Error() {
		t.Fatalf("expected unknown handler failure, got %+v", store.failed)
	}
}

// Start/Shutdown 生命周期：worker 能领取并执行任务，关闭后干净退出
func TestRunnerStartShutdown(t *testing.T) {
	payload, _ := json.Marshal(SessionTaskPayload{UserID: "u1", SessionID: "s1"})
	store := &fakeStore{tasks: []*Task{
		{ID: "t4", Type: TypeReportGeneration, Payload: payload},
	}}
	r := NewRunner(store, Config{Workers: 1, PollInterval: 20 * time.Millisecond}, "w-test", testLogger())
	r.Register(TypeReportGeneration, func(_ context.Context, raw json.RawMessage) error {
		var p SessionTaskPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if p.SessionID != "s1" {
			return errors.New("wrong session")
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)

	deadline := time.After(2 * time.Second)
	for {
		store.mu.Lock()
		done := len(store.succeeded) == 1
		store.mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("task was not executed before deadline")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	cancel()
	r.Shutdown() // 应快速返回而不阻塞（在途任务已结束）
}
