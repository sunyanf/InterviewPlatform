package report

import (
	"io"
	"log/slog"
	"testing"

	"ai-interview-platform/internal/agent"
)

// TestSanitizeLearningPlan_ReplacesRejectedAndKeepsOthers 命中条目替换为中性占位，其余条目保留
func TestSanitizeLearningPlan_ReplacesRejectedAndKeepsOthers(t *testing.T) {
	svc := &Service{log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	out := &agent.LearningPlanOutput{
		Strengths:  []string{"结构化表达清晰，项目数据具体", "可以帮你代考"},
		Weaknesses: []string{"深度不足"},
		FocusAreas: []agent.FocusArea{
			{
				Topic:       "系统设计",
				Reason:      "根因分析待加强",
				Suggestions: []string{"多练习故障复盘", "研究赌博网站引流手法"},
			},
		},
		NextTraining: agent.NextTraining{
			Focus:           "加强并发模型",
			SuggestedTopics: []string{"GMP 调度", "代考技巧"},
		},
	}

	svc.sanitizeLearningPlan(out, "s1")

	if out.Strengths[0] != "结构化表达清晰，项目数据具体" {
		t.Errorf("合规 strength 应保留，got %q", out.Strengths[0])
	}
	if out.Strengths[1] != neutralPlaceholder {
		t.Errorf("命中 strength 应替换为占位，got %q", out.Strengths[1])
	}
	if out.Weaknesses[0] != "深度不足" {
		t.Errorf("合规 weakness 应保留，got %q", out.Weaknesses[0])
	}
	if out.FocusAreas[0].Topic != "系统设计" || out.FocusAreas[0].Reason != "根因分析待加强" {
		t.Errorf("合规 focus_area 文本应保留，got %+v", out.FocusAreas[0])
	}
	if out.FocusAreas[0].Suggestions[0] != "多练习故障复盘" {
		t.Errorf("合规 suggestion 应保留，got %q", out.FocusAreas[0].Suggestions[0])
	}
	if out.FocusAreas[0].Suggestions[1] != neutralPlaceholder {
		t.Errorf("命中 suggestion 应替换为占位，got %q", out.FocusAreas[0].Suggestions[1])
	}
	if out.NextTraining.Focus != "加强并发模型" {
		t.Errorf("合规 next_training.focus 应保留，got %q", out.NextTraining.Focus)
	}
	if out.NextTraining.SuggestedTopics[0] != "GMP 调度" {
		t.Errorf("合规 topic 应保留，got %q", out.NextTraining.SuggestedTopics[0])
	}
	if out.NextTraining.SuggestedTopics[1] != neutralPlaceholder {
		t.Errorf("命中 topic 应替换为占位，got %q", out.NextTraining.SuggestedTopics[1])
	}
}

// TestSanitizeLearningPlan_EmptyUntouched 空/零长度数组保持原样，不产生占位
func TestSanitizeLearningPlan_EmptyUntouched(t *testing.T) {
	svc := &Service{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	out := &agent.LearningPlanOutput{}

	svc.sanitizeLearningPlan(out, "s1")

	if len(out.Strengths) != 0 || len(out.Weaknesses) != 0 || len(out.FocusAreas) != 0 {
		t.Fatalf("空数组应保持原样，got %+v", out)
	}
	if out.NextTraining.Focus != "" || len(out.NextTraining.SuggestedTopics) != 0 {
		t.Fatalf("空 next_training 应保持原样，got %+v", out.NextTraining)
	}
}

// TestSanitizeLearningPlan_AllClean 全部合规时不做任何替换
func TestSanitizeLearningPlan_AllClean(t *testing.T) {
	svc := &Service{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	out := &agent.LearningPlanOutput{
		Strengths:    []string{"表达清晰"},
		Weaknesses:   []string{"深度不够"},
		FocusAreas:   []agent.FocusArea{{Topic: "系统设计", Reason: "缺乏深度", Suggestions: []string{"多练习"}}},
		NextTraining: agent.NextTraining{Focus: "复盘", SuggestedTopics: []string{"CAP"}},
	}

	svc.sanitizeLearningPlan(out, "s1")

	if out.Strengths[0] != "表达清晰" || out.Weaknesses[0] != "深度不够" {
		t.Fatalf("合规内容不应被替换：%+v", out)
	}
	if out.FocusAreas[0].Topic != "系统设计" || out.FocusAreas[0].Suggestions[0] != "多练习" {
		t.Fatalf("合规内容不应被替换：%+v", out.FocusAreas)
	}
	if out.NextTraining.Focus != "复盘" || out.NextTraining.SuggestedTopics[0] != "CAP" {
		t.Fatalf("合规内容不应被替换：%+v", out.NextTraining)
	}
}
