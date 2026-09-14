package agent

import (
	"context"
	"log/slog"
	"testing"
)

func TestLearningPlan_HappyPath(t *testing.T) {
	stub := &stubProvider{content: `{
		"strengths": ["概念表述准确"],
		"weaknesses": ["channel 关闭语义不足"],
		"focus_areas": [{"topic": "Go 并发深入", "reason": "对应 correctness 不足", "suggestions": ["阅读官方文档", "练习题"]}],
		"next_training": {"focus": "Go 并发", "suggested_question_type": "Technical", "suggested_difficulty": "hard", "suggested_topics": ["GMP"]}
	}`}
	a := &Agent{llm: stub, log: slog.Default()}

	out, err := a.LearningPlan(context.Background(), LearningPlannerInput{
		JobTitle:   "Go 工程师",
		Dimensions: map[string]float64{"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
	})
	if err != nil {
		t.Fatalf("LearningPlan error: %v", err)
	}
	if len(out.Strengths) != 1 || len(out.Weaknesses) != 1 || len(out.FocusAreas) != 1 {
		t.Errorf("output = %+v", out)
	}
	// 难度归一化为小写
	if out.NextTraining.SuggestedDifficulty != "hard" {
		t.Errorf("difficulty = %q", out.NextTraining.SuggestedDifficulty)
	}
	// question_type 归一化为小写
	if out.NextTraining.SuggestedQuestionType != "technical" {
		t.Errorf("question_type = %q", out.NextTraining.SuggestedQuestionType)
	}
	if out.PromptVersion != PromptLearningPlanner {
		t.Errorf("prompt_version = %q", out.PromptVersion)
	}
}

func TestLearningPlan_Normalization(t *testing.T) {
	stub := &stubProvider{content: `{
		"strengths": ["有效优点", "", "   "],
		"weaknesses": ["有效不足"],
		"focus_areas": [
			{"topic": "有效重点", "reason": "", "suggestions": ["建议1", ""]},
			{"topic": "   ", "reason": "空 topic 剔除", "suggestions": []}
		],
		"next_training": {"focus": "", "suggested_question_type": "", "suggested_difficulty": "insane", "suggested_topics": ["topic1", ""]}
	}`}
	a := &Agent{llm: stub, log: slog.Default()}

	out, err := a.LearningPlan(context.Background(), LearningPlannerInput{
		Dimensions: map[string]float64{"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
	})
	if err != nil {
		t.Fatalf("LearningPlan error: %v", err)
	}
	// 空建议剔除
	if len(out.Strengths) != 1 {
		t.Errorf("strengths = %v, want 1", out.Strengths)
	}
	if len(out.FocusAreas) != 1 || out.FocusAreas[0].Topic != "有效重点" {
		t.Errorf("focus_areas = %+v", out.FocusAreas)
	}
	if len(out.FocusAreas[0].Suggestions) != 1 {
		t.Errorf("suggestions = %v, want 1", out.FocusAreas[0].Suggestions)
	}
	// 非法难度默认 medium
	if out.NextTraining.SuggestedDifficulty != "medium" {
		t.Errorf("difficulty = %q, want medium (invalid default)", out.NextTraining.SuggestedDifficulty)
	}
	if len(out.NextTraining.SuggestedTopics) != 1 {
		t.Errorf("topics = %v, want 1", out.NextTraining.SuggestedTopics)
	}
}

func TestLearningPlan_MissingRequired(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"missing strengths", `{"weaknesses":["w"],"focus_areas":[{"topic":"t"}]}`},
		{"missing weaknesses", `{"strengths":["s"],"focus_areas":[{"topic":"t"}]}`},
		{"missing focus_areas", `{"strengths":["s"],"weaknesses":["w"]}`},
		{"empty strengths after clean", `{"strengths":["  "],"weaknesses":["w"],"focus_areas":[{"topic":"t"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &Agent{llm: &stubProvider{content: tc.content}, log: slog.Default()}
			if _, err := a.LearningPlan(context.Background(), LearningPlannerInput{
				Dimensions: map[string]float64{"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
			}); err == nil {
				t.Errorf("%s should fail validation", tc.name)
			}
		})
	}
}

func TestLearningPlan_NoDimensions(t *testing.T) {
	a := &Agent{llm: &stubProvider{}, log: slog.Default()}
	if _, err := a.LearningPlan(context.Background(), LearningPlannerInput{}); err == nil {
		t.Error("empty dimensions should fail")
	}
}

func TestFormatDimensions(t *testing.T) {
	got := formatDimensions(map[string]float64{"correctness": 70, "depth": 60, "logic": 75.4, "communication": 80})
	want := "correctness=70, depth=60, logic=75, communication=80"
	if got != want {
		t.Errorf("format = %q, want %q", got, want)
	}
}
