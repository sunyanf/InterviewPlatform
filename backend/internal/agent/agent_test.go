package agent

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"ai-interview-platform/pkg/llm"
)

// stubProvider 测试桩：返回预设内容
type stubProvider struct {
	content string
	err     error
	// lastCtx 记录最近一次调用的 ctx（用于超时断言）
	lastCtx context.Context
}

func (s *stubProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	s.lastCtx = ctx
	if s.err != nil {
		return nil, s.err
	}
	return &llm.ChatResponse{Content: s.content, Model: "stub-model"}, nil
}

func (s *stubProvider) Name() string { return "stub" }

func newTestAgent(content string) *Agent {
	return New(&stubProvider{content: content}, slog.Default())
}

func TestPlanQuestions_Normalization(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantCount int
		wantType  string
		wantDiff  string
	}{
		{
			name:      "正常输出",
			content:   `{"questions":[{"question":"什么是 goroutine?","type":"technical","difficulty":"medium","expected_points":["a","b"]}]}`,
			wantCount: 1, wantType: "technical", wantDiff: "medium",
		},
		{
			name:      "markdown 包裹",
			content:   "```json\n{\"questions\":[{\"question\":\"Q1\",\"type\":\"project\",\"difficulty\":\"hard\",\"expected_points\":[]}]}\n```",
			wantCount: 1, wantType: "project", wantDiff: "hard",
		},
		{
			name:      "非法 type 归一化为 technical",
			content:   `{"questions":[{"question":"Q1","type":"unknown","difficulty":"easy","expected_points":[]}]}`,
			wantCount: 1, wantType: "technical", wantDiff: "easy",
		},
		{
			name:      "非法 difficulty 归一化为 medium",
			content:   `{"questions":[{"question":"Q1","type":"technical","difficulty":"extreme","expected_points":[]}]}`,
			wantCount: 1, wantType: "technical", wantDiff: "medium",
		},
		{
			name:      "空问题被过滤",
			content:   `{"questions":[{"question":"  ","type":"technical","difficulty":"easy"},{"question":"Q2","type":"technical","difficulty":"easy","expected_points":[]}]}`,
			wantCount: 1, wantType: "technical", wantDiff: "easy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := newTestAgent(tt.content)
			got, err := agent.PlanQuestions(context.Background(), PlanQuestionsInput{
				JobTitle: "Go 工程师", Count: 1, InterviewType: "technical",
			})
			if err != nil {
				t.Fatalf("PlanQuestions error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d questions, want %d", len(got), tt.wantCount)
			}
			if got[0].Type != tt.wantType {
				t.Errorf("type = %q, want %q", got[0].Type, tt.wantType)
			}
			if got[0].Difficulty != tt.wantDiff {
				t.Errorf("difficulty = %q, want %q", got[0].Difficulty, tt.wantDiff)
			}
			if got[0].ExpectedPoints == nil {
				t.Error("expected_points should never be nil")
			}
		})
	}
}

func TestPlanQuestions_InvalidInput(t *testing.T) {
	agent := newTestAgent(`{"questions":[]}`)

	// 题数越界
	if _, err := agent.PlanQuestions(context.Background(), PlanQuestionsInput{Count: 0}); err == nil {
		t.Error("count=0 should fail")
	}
	if _, err := agent.PlanQuestions(context.Background(), PlanQuestionsInput{Count: 99}); err == nil {
		t.Error("count=99 should fail")
	}
	// 非法 JSON
	badAgent := newTestAgent("not json")
	if _, err := badAgent.PlanQuestions(context.Background(), PlanQuestionsInput{Count: 1}); err == nil {
		t.Error("invalid json should fail")
	}
}

func TestAnalyzeAnswer(t *testing.T) {
	agent := newTestAgent(`{"claims":["goroutine 是协程"],"correct_points":["正确"],"wrong_points":[],"missing_points":["channel"],"knowledge_gaps":[]}`)

	got, err := agent.AnalyzeAnswer(context.Background(), AnalyzeAnswerInput{
		Question: "Q", ExpectedPoints: []string{"a"}, AnswerText: "A", Difficulty: "medium",
	})
	if err != nil {
		t.Fatalf("AnalyzeAnswer error: %v", err)
	}
	if len(got.Claims) != 1 || got.Claims[0] != "goroutine 是协程" {
		t.Errorf("claims = %v", got.Claims)
	}
	if got.PromptVersion != PromptAnswerAnalyzer {
		t.Errorf("prompt_version = %q, want %q", got.PromptVersion, PromptAnswerAnalyzer)
	}
	if got.Model != "stub-model" {
		t.Errorf("model = %q, want stub-model", got.Model)
	}
}

func TestAnalyzeAnswer_NullArraysNormalized(t *testing.T) {
	agent := newTestAgent(`{"claims":null,"correct_points":null,"wrong_points":null,"missing_points":null,"knowledge_gaps":null}`)

	got, err := agent.AnalyzeAnswer(context.Background(), AnalyzeAnswerInput{AnswerText: "A"})
	if err != nil {
		t.Fatalf("AnalyzeAnswer error: %v", err)
	}
	for name, arr := range map[string][]string{
		"claims": got.Claims, "correct_points": got.CorrectPoints,
		"wrong_points": got.WrongPoints, "missing_points": got.MissingPoints,
		"knowledge_gaps": got.KnowledgeGaps,
	} {
		if arr == nil {
			t.Errorf("%s should be non-nil empty slice", name)
		}
	}
}

func TestDecideFollowUp_EmptyQuestionMeansNo(t *testing.T) {
	agent := newTestAgent(`{"should_follow_up":true,"reason":"回答太浅","question":"","target_gap":""}`)

	got, err := agent.DecideFollowUp(context.Background(), FollowUpInput{Question: "Q", AnswerText: "A"})
	if err != nil {
		t.Fatalf("DecideFollowUp error: %v", err)
	}
	if got.ShouldFollowUp {
		t.Error("should_follow_up should be false when question is empty")
	}
}

func TestDecideFollowUp_Normal(t *testing.T) {
	agent := newTestAgent(`{"should_follow_up":true,"reason":"需要深挖","question":"追问内容","target_gap":"并发模型"}`)

	got, err := agent.DecideFollowUp(context.Background(), FollowUpInput{Question: "Q", AnswerText: "A"})
	if err != nil {
		t.Fatalf("DecideFollowUp error: %v", err)
	}
	if !got.ShouldFollowUp || got.Question != "追问内容" {
		t.Errorf("decision = %+v", got)
	}
}

func TestOpeningMessage(t *testing.T) {
	agent := newTestAgent("你好，欢迎参加本次面试。我们先从第一道题开始。")

	got, err := agent.OpeningMessage(context.Background(), OpeningInput{JobTitle: "Go", InterviewType: "technical", FirstQuestion: "Q1"})
	if err != nil {
		t.Fatalf("OpeningMessage error: %v", err)
	}
	if got == "" {
		t.Error("opening should not be empty")
	}

	// 空开场白报错
	emptyAgent := newTestAgent("   ")
	if _, err := emptyAgent.OpeningMessage(context.Background(), OpeningInput{}); err == nil {
		t.Error("empty opening should fail")
	}
}

func TestLLMError_Propagates(t *testing.T) {
	agent := New(&stubProvider{err: errors.New("timeout")}, slog.Default())
	if _, err := agent.PlanQuestions(context.Background(), PlanQuestionsInput{Count: 1}); err == nil {
		t.Error("llm error should propagate")
	}
}

func TestChat_DefaultTimeoutApplied(t *testing.T) {
	// 上游 ctx 无 deadline 时，agent.chat 应施加默认超时（保护同步链路）
	stub := &stubProvider{content: `{"questions":[]}`}
	agent := New(stub, slog.Default())
	_, _ = agent.PlanQuestions(context.Background(), PlanQuestionsInput{Count: 1})

	if stub.lastCtx == nil {
		t.Fatal("stub should capture ctx")
	}
	_, ok := stub.lastCtx.Deadline()
	if !ok {
		t.Error("chat should apply default timeout when caller has no deadline")
	}
}

func TestPlanQuestions_EmptyOutput(t *testing.T) {
	// 模型输出为空 → Schema 校验必须拦截，不得静默降级
	agent := newTestAgent("")
	if _, err := agent.PlanQuestions(context.Background(), PlanQuestionsInput{Count: 1}); err == nil {
		t.Error("empty output should fail schema validation")
	}
}

func TestPlanQuestions_WrongFieldType(t *testing.T) {
	// expected_points 为字符串而非数组 → 整体校验失败（不落库、不静默）
	agent := newTestAgent(`{"questions":[{"question":"Q1","type":"technical","difficulty":"easy","expected_points":"not-an-array"}]}`)
	if _, err := agent.PlanQuestions(context.Background(), PlanQuestionsInput{Count: 1}); err == nil {
		t.Error("wrong field type should fail schema validation")
	}
}
