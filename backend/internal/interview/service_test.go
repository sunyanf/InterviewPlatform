package interview

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	apperrors "ai-interview-platform/pkg/errors"
)

// discardLogger 测试用静默 logger
func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeRepo 内存版面试 repo（实现 interviewRepo），仅记录 CreateAnswer 是否被调用
type fakeRepo struct {
	sess              *Session
	createAnswerCalls int
}

func (f *fakeRepo) Create(ctx context.Context, s *Session) error { return nil }
func (f *fakeRepo) GetByID(ctx context.Context, id string) (*Session, error) {
	return f.sess, nil
}
func (f *fakeRepo) ListByUserID(ctx context.Context, userID string) ([]Session, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateStatus(ctx context.Context, id, status string, startedAt, endedAt *time.Time) error {
	return nil
}
func (f *fakeRepo) GetJobTitle(ctx context.Context, jobID string) (string, error) { return "", nil }
func (f *fakeRepo) CreateQuestions(ctx context.Context, sessionID string, questions []Question) error {
	return nil
}
func (f *fakeRepo) GetMaxSeq(ctx context.Context, sessionID string) (int, error) { return 0, nil }
func (f *fakeRepo) UpdateSessionMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	return nil
}
func (f *fakeRepo) UpdateAnswerAnalysis(ctx context.Context, answerID string, analysis any) error {
	return nil
}
func (f *fakeRepo) ListQuestionsBySession(ctx context.Context, sessionID string) ([]Question, error) {
	return nil, nil
}
func (f *fakeRepo) GetQuestionByID(ctx context.Context, id string) (*Question, error) {
	return nil, nil
}
func (f *fakeRepo) CreateAnswer(ctx context.Context, a *Answer) error {
	f.createAnswerCalls++
	return nil
}
func (f *fakeRepo) CountAnswered(ctx context.Context, sessionID string) (int, error) { return 0, nil }

// TestSubmitAnswer_ContentRejectedBeforePersist 回答含违禁词 → 400 CONTENT_REJECTED 且未落库
func TestSubmitAnswer_ContentRejectedBeforePersist(t *testing.T) {
	repo := &fakeRepo{sess: &Session{ID: "s1", UserID: "u1", Status: StatusRunning}}
	svc := &Service{repo: repo, log: discardLogger()}

	_, err := svc.SubmitAnswer(context.Background(), "u1", "s1", SubmitAnswerRequest{
		QuestionID: "q1", TextContent: "我可以帮你代考，保证通过",
	})
	if err == nil {
		t.Fatal("expected CONTENT_REJECTED, got nil")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "CONTENT_REJECTED" || ae.Status != 400 {
		t.Fatalf("got %v, want CONTENT_REJECTED 400", err)
	}
	if repo.createAnswerCalls != 0 {
		t.Fatalf("回答命中被拒时不得落库，CreateAnswer 调用 %d 次", repo.createAnswerCalls)
	}
}

// TestSubmitAnswer_EmptyAnswerUnchanged 空白回答仍走既有 EMPTY_ANSWER 校验（审核插在其后，不改变原行为）
func TestSubmitAnswer_EmptyAnswerUnchanged(t *testing.T) {
	repo := &fakeRepo{sess: &Session{ID: "s1", UserID: "u1", Status: StatusRunning}}
	svc := &Service{repo: repo, log: discardLogger()}

	_, err := svc.SubmitAnswer(context.Background(), "u1", "s1", SubmitAnswerRequest{
		QuestionID: "q1", TextContent: "   ",
	})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "EMPTY_ANSWER" {
		t.Fatalf("got %v, want EMPTY_ANSWER", err)
	}
	if repo.createAnswerCalls != 0 {
		t.Fatalf("空回答不得落库，CreateAnswer 调用 %d 次", repo.createAnswerCalls)
	}
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		// 合法转换
		{"INIT 到 RUNNING", StatusInit, StatusRunning, true},
		{"INIT 到 READY", StatusInit, StatusReady, true},
		{"INIT 到 FAILED", StatusInit, StatusFailed, true},
		{"READY 到 RUNNING", StatusReady, StatusRunning, true},
		{"RUNNING 到 COMPLETED", StatusRunning, StatusCompleted, true},
		{"RUNNING 到 PAUSED", StatusRunning, StatusPaused, true},
		{"RUNNING 到 FINISHING", StatusRunning, StatusFinishing, true},
		{"PAUSED 到 RUNNING", StatusPaused, StatusRunning, true},
		{"FINISHING 到 EVALUATING", StatusFinishing, StatusEvaluating, true},
		{"FINISHING 到 COMPLETED", StatusFinishing, StatusCompleted, true},
		{"EVALUATING 到 COMPLETED", StatusEvaluating, StatusCompleted, true},

		// 非法转换
		{"INIT 到 COMPLETED", StatusInit, StatusCompleted, false},
		{"COMPLETED 到 RUNNING", StatusCompleted, StatusRunning, false},
		{"COMPLETED 到 COMPLETED", StatusCompleted, StatusCompleted, false},
		{"RUNNING 到 INIT", StatusRunning, StatusInit, false},
		{"FAILED 到 RUNNING", StatusFailed, StatusRunning, false},
		{"PAUSED 到 COMPLETED", StatusPaused, StatusCompleted, false},
		{"未知状态 到 RUNNING", "UNKNOWN", StatusRunning, false},
		{"RUNNING 到 未知状态", StatusRunning, "UNKNOWN", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("canTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
