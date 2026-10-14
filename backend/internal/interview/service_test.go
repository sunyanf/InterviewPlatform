package interview

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"ai-interview-platform/internal/agent"
	apperrors "ai-interview-platform/pkg/errors"
)

// discardLogger 测试用静默 logger
func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeRepo 内存版面试 repo（实现 interviewRepo），仅记录 CreateAnswer 是否被调用
type fakeRepo struct {
	sess              *Session
	createAnswerCalls int
	// CoachQuestion 用例的可控数据
	questions []Question
	jobTitle  string
	// 示范结果持久化存储（question_id → 示范结果），nil 表示空表
	coachings map[string]*agent.AnswerCoaching
	// 置非 nil 时模拟 UpsertCoaching 落库失败
	upsertCoachingErr error
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
func (f *fakeRepo) GetJobTitle(ctx context.Context, jobID string) (string, error) {
	return f.jobTitle, nil
}
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
	return f.questions, nil
}
func (f *fakeRepo) GetQuestionByID(ctx context.Context, id string) (*Question, error) {
	return nil, nil
}
func (f *fakeRepo) CreateAnswer(ctx context.Context, a *Answer) error {
	f.createAnswerCalls++
	return nil
}
func (f *fakeRepo) CountAnswered(ctx context.Context, sessionID string) (int, error) { return 0, nil }
func (f *fakeRepo) GetCoachingByQuestionID(ctx context.Context, questionID string) (*agent.AnswerCoaching, error) {
	if f.coachings == nil {
		return nil, nil
	}
	return f.coachings[questionID], nil
}
func (f *fakeRepo) UpsertCoaching(ctx context.Context, sessionID, questionID string, c *agent.AnswerCoaching) error {
	if f.upsertCoachingErr != nil {
		return f.upsertCoachingErr
	}
	if f.coachings == nil {
		f.coachings = map[string]*agent.AnswerCoaching{}
	}
	f.coachings[questionID] = c
	return nil
}

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

// fakeAgent 实现 AgentService，用于 CoachQuestion 用例
type fakeAgent struct {
	coaching  *agent.AnswerCoaching
	err       error
	lastInput agent.AnswerCoachInput
	// CoachAnswer 调用计数（缓存命中用例断言为 0）
	coachCalls int
}

func (f *fakeAgent) PlanQuestions(ctx context.Context, in agent.PlanQuestionsInput) ([]agent.PlannedQuestion, error) {
	return nil, nil
}
func (f *fakeAgent) AnalyzeAnswer(ctx context.Context, in agent.AnalyzeAnswerInput) (*agent.AnswerAnalysis, error) {
	return nil, nil
}
func (f *fakeAgent) DecideFollowUp(ctx context.Context, in agent.FollowUpInput) (*agent.FollowUpDecision, error) {
	return nil, nil
}
func (f *fakeAgent) OpeningMessage(ctx context.Context, in agent.OpeningInput) (string, error) {
	return "", nil
}
func (f *fakeAgent) CoachAnswer(ctx context.Context, in agent.AnswerCoachInput) (*agent.AnswerCoaching, error) {
	f.coachCalls++
	f.lastInput = in
	return f.coaching, f.err
}

// completedSession 已结束且属于 u1 的会话
func completedSession() *Session {
	return &Session{ID: "s1", UserID: "u1", JobID: "j1", Status: StatusCompleted}
}

// answeredQuestion 属于 s1 且已作答的问题
func answeredQuestion() Question {
	return Question{
		ID: "q1", SessionID: "s1", Question: "Q1", Difficulty: "medium",
		ExpectedPoints: []string{"要点一"},
		Answer:         &Answer{TextContent: "我的回答"},
	}
}

// assertAppErr 断言错误码与状态码
func assertAppErr(t *testing.T, err error, code string, status int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s %d, got nil", code, status)
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != code || ae.Status != status {
		t.Fatalf("got %v, want %s %d", err, code, status)
	}
}

func TestCoachQuestion_CrossUserForbidden(t *testing.T) {
	repo := &fakeRepo{sess: &Session{ID: "s1", UserID: "u2", Status: StatusCompleted}}
	svc := &Service{repo: repo, log: discardLogger()}

	_, err := svc.CoachQuestion(context.Background(), "u1", "s1", "q1")
	assertAppErr(t, err, "FORBIDDEN", 403)
}

func TestCoachQuestion_SessionNotCompleted(t *testing.T) {
	repo := &fakeRepo{sess: &Session{ID: "s1", UserID: "u1", Status: StatusRunning}}
	svc := &Service{repo: repo, log: discardLogger()}

	_, err := svc.CoachQuestion(context.Background(), "u1", "s1", "q1")
	assertAppErr(t, err, "SESSION_NOT_COMPLETED", 400)
}

func TestCoachQuestion_QuestionNotInSession(t *testing.T) {
	repo := &fakeRepo{
		sess:      completedSession(),
		questions: []Question{{ID: "q-other", SessionID: "s1", Answer: &Answer{TextContent: "A"}}},
	}
	svc := &Service{repo: repo, log: discardLogger()}

	_, err := svc.CoachQuestion(context.Background(), "u1", "s1", "q1")
	assertAppErr(t, err, "QUESTION_NOT_IN_SESSION", 400)
}

func TestCoachQuestion_AnswerRequired(t *testing.T) {
	// 未作答（Answer 为 nil）
	repo := &fakeRepo{sess: completedSession(), questions: []Question{{ID: "q1", SessionID: "s1"}}}
	svc := &Service{repo: repo, log: discardLogger()}
	_, err := svc.CoachQuestion(context.Background(), "u1", "s1", "q1")
	assertAppErr(t, err, "ANSWER_REQUIRED", 400)

	// 已作答但内容为空白
	repo2 := &fakeRepo{
		sess:      completedSession(),
		questions: []Question{{ID: "q1", SessionID: "s1", Answer: &Answer{TextContent: "   "}}},
	}
	svc2 := &Service{repo: repo2, log: discardLogger()}
	_, err = svc2.CoachQuestion(context.Background(), "u1", "s1", "q1")
	assertAppErr(t, err, "ANSWER_REQUIRED", 400)
}

func TestCoachQuestion_MissingIDs(t *testing.T) {
	svc := &Service{repo: &fakeRepo{}, log: discardLogger()}
	_, err := svc.CoachQuestion(context.Background(), "u1", "s1", "")
	assertAppErr(t, err, "BAD_REQUEST", 400)
}

func TestCoachQuestion_HappyPath(t *testing.T) {
	coaching := &agent.AnswerCoaching{ImprovedAnswer: "改良示范", KeyChanges: []string{"更聚焦"}, PromptVersion: "interview.answer_coach.v1", Model: "mock"}
	repo := &fakeRepo{sess: completedSession(), questions: []Question{answeredQuestion()}, jobTitle: "Go 工程师"}
	fa := &fakeAgent{coaching: coaching}
	svc := &Service{repo: repo, agent: fa, log: discardLogger()}

	got, err := svc.CoachQuestion(context.Background(), "u1", "s1", "q1")
	if err != nil {
		t.Fatalf("CoachQuestion error: %v", err)
	}
	if got != coaching {
		t.Errorf("returned coaching mismatch")
	}
	// 输入组装正确
	if fa.lastInput.JobTitle != "Go 工程师" || fa.lastInput.Question != "Q1" || fa.lastInput.AnswerText != "我的回答" {
		t.Errorf("input mapping wrong: %+v", fa.lastInput)
	}
}

func TestCoachQuestion_ContentReplaced(t *testing.T) {
	coaching := &agent.AnswerCoaching{
		ImprovedAnswer: "你可以代考，保证通过",
		KeyChanges:     []string{"正常改进", "帮你办证"},
	}
	repo := &fakeRepo{sess: completedSession(), questions: []Question{answeredQuestion()}}
	fa := &fakeAgent{coaching: coaching}
	svc := &Service{repo: repo, agent: fa, log: discardLogger()}

	got, err := svc.CoachQuestion(context.Background(), "u1", "s1", "q1")
	if err != nil {
		t.Fatalf("CoachQuestion should still succeed: %v", err)
	}
	if got.ImprovedAnswer != coachNeutralPlaceholder {
		t.Errorf("improved_answer = %q, want placeholder", got.ImprovedAnswer)
	}
	if got.KeyChanges[0] != "正常改进" || got.KeyChanges[1] != coachNeutralPlaceholder {
		t.Errorf("key_changes = %v", got.KeyChanges)
	}
}

func TestCoachQuestion_AgentError(t *testing.T) {
	repo := &fakeRepo{sess: completedSession(), questions: []Question{answeredQuestion()}}
	fa := &fakeAgent{err: errors.New("llm timeout")}
	svc := &Service{repo: repo, agent: fa, log: discardLogger()}

	_, err := svc.CoachQuestion(context.Background(), "u1", "s1", "q1")
	assertAppErr(t, err, "COACHING_FAILED", 500)
}

// TestCoachQuestion_CacheHit 命中持久化示范 → 直接返回缓存内容，且不调用 Agent（省成本）
func TestCoachQuestion_CacheHit(t *testing.T) {
	cached := &agent.AnswerCoaching{
		ImprovedAnswer: "缓存中的示范",
		KeyChanges:     []string{"更聚焦"},
		PromptVersion:  "interview.answer_coach.v1",
		Model:          "mock",
	}
	repo := &fakeRepo{
		sess:      completedSession(),
		questions: []Question{answeredQuestion()},
		coachings: map[string]*agent.AnswerCoaching{"q1": cached},
	}
	fa := &fakeAgent{coaching: &agent.AnswerCoaching{ImprovedAnswer: "不应被生成"}}
	svc := &Service{repo: repo, agent: fa, log: discardLogger()}

	got, err := svc.CoachQuestion(context.Background(), "u1", "s1", "q1")
	if err != nil {
		t.Fatalf("CoachQuestion error: %v", err)
	}
	if fa.coachCalls != 0 {
		t.Errorf("缓存命中时不应调用 Agent，实际调用 %d 次", fa.coachCalls)
	}
	if got != cached {
		t.Errorf("应原样返回持久化结果: got %+v, want %+v", got, cached)
	}
}

// TestCoachQuestion_GeneratedAndPersisted 未命中缓存 → 生成 → 已落库
func TestCoachQuestion_GeneratedAndPersisted(t *testing.T) {
	coaching := &agent.AnswerCoaching{
		ImprovedAnswer: "新生成的示范",
		KeyChanges:     []string{"更具体"},
		PromptVersion:  "interview.answer_coach.v1",
		Model:          "mock",
	}
	repo := &fakeRepo{sess: completedSession(), questions: []Question{answeredQuestion()}, jobTitle: "Go 工程师"}
	fa := &fakeAgent{coaching: coaching}
	svc := &Service{repo: repo, agent: fa, log: discardLogger()}

	got, err := svc.CoachQuestion(context.Background(), "u1", "s1", "q1")
	if err != nil {
		t.Fatalf("CoachQuestion error: %v", err)
	}
	if fa.coachCalls != 1 {
		t.Errorf("未命中缓存时应调用 Agent 1 次，实际 %d 次", fa.coachCalls)
	}
	if saved := repo.coachings["q1"]; saved != coaching {
		t.Errorf("生成结果未正确落库: saved=%+v, want %+v", saved, coaching)
	}
	if got != coaching {
		t.Errorf("returned coaching mismatch")
	}
}

// TestCoachQuestion_UpsertError 落库失败 → 500 INTERNAL_ERROR（下次点击可重试）
func TestCoachQuestion_UpsertError(t *testing.T) {
	repo := &fakeRepo{
		sess:              completedSession(),
		questions:         []Question{answeredQuestion()},
		upsertCoachingErr: errors.New("db down"),
	}
	fa := &fakeAgent{coaching: &agent.AnswerCoaching{ImprovedAnswer: "示范"}}
	svc := &Service{repo: repo, agent: fa, log: discardLogger()}

	_, err := svc.CoachQuestion(context.Background(), "u1", "s1", "q1")
	assertAppErr(t, err, "INTERNAL_ERROR", 500)
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
