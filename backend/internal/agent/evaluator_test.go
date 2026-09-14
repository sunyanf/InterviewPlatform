package agent

import (
	"context"
	"log/slog"
	"testing"
)

func TestEvaluate_HappyPath(t *testing.T) {
	stub := &stubProvider{content: `{
		"dimensions": {"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
		"evidence": [{"question_seq": 1, "issue": "缺少关闭语义", "evidence": "候选人只说明了 channel 用于通信", "reference": ""}],
		"recommendations": ["深入学习 channel 关闭语义", "回答先给结论"]
	}`}
	a := &Agent{llm: stub, log: slog.Default()}

	out, err := a.Evaluate(context.Background(), EvaluateInput{
		JobTitle: "Go 工程师",
		QA:       []EvaluatedQA{{Seq: 1, Question: "Q1", AnswerText: "A1"}},
	})
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}
	if out.Dimensions["correctness"] != 70 || out.Dimensions["communication"] != 80 {
		t.Errorf("dimensions = %v", out.Dimensions)
	}
	if len(out.Evidence) != 1 || out.Evidence[0].QuestionSeq != 1 {
		t.Errorf("evidence = %v", out.Evidence)
	}
	if out.PromptVersion != PromptEvaluator {
		t.Errorf("prompt_version = %q", out.PromptVersion)
	}
	if out.Model != "stub" {
		t.Errorf("model = %q", out.Model)
	}
}

func TestEvaluate_Normalization(t *testing.T) {
	stub := &stubProvider{content: `{
		"dimensions": {"correctness": 150, "depth": -5, "logic": 75.4, "communication": 80, "unknown_dim": 90},
		"evidence": [
			{"question_seq": 1, "issue": "有效证据", "evidence": "回答原文片段", "reference": ""},
			{"question_seq": 2, "issue": "无证据内容", "evidence": "   ", "reference": ""}
		],
		"recommendations": ["有效建议", "", "  "]
	}`}
	a := &Agent{llm: stub, log: slog.Default()}

	out, err := a.Evaluate(context.Background(), EvaluateInput{
		QA: []EvaluatedQA{{Seq: 1, Question: "Q1", AnswerText: "A1"}},
	})
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}
	// 越界 clamp
	if out.Dimensions["correctness"] != 100 {
		t.Errorf("correctness = %v, want 100 (clamped)", out.Dimensions["correctness"])
	}
	if out.Dimensions["depth"] != 0 {
		t.Errorf("depth = %v, want 0 (clamped)", out.Dimensions["depth"])
	}
	// 四舍五入取整
	if out.Dimensions["logic"] != 75 {
		t.Errorf("logic = %v, want 75 (rounded)", out.Dimensions["logic"])
	}
	// 白名单外维度剔除
	if _, ok := out.Dimensions["unknown_dim"]; ok {
		t.Error("unknown dimension should be removed")
	}
	// 无证据内容的 evidence 剔除
	if len(out.Evidence) != 1 {
		t.Errorf("evidence = %v, want 1 valid item", out.Evidence)
	}
	// 空建议剔除
	if len(out.Recommendations) != 1 {
		t.Errorf("recommendations = %v, want 1 valid item", out.Recommendations)
	}
}

func TestEvaluate_MissingDimension(t *testing.T) {
	// 缺维度 → 评估无效（可重跑），不得静默用默认分
	stub := &stubProvider{content: `{"dimensions": {"correctness": 70, "depth": 60, "communication": 80}}`}
	a := &Agent{llm: stub, log: slog.Default()}

	if _, err := a.Evaluate(context.Background(), EvaluateInput{
		QA: []EvaluatedQA{{Seq: 1, Question: "Q1", AnswerText: "A1"}},
	}); err == nil {
		t.Error("missing dimension should fail validation")
	}
}

func TestEvaluate_WrongFieldType(t *testing.T) {
	// 维度分为字符串 → Schema 校验失败
	stub := &stubProvider{content: `{"dimensions": {"correctness": "high", "depth": 60, "logic": 75, "communication": 80}}`}
	a := &Agent{llm: stub, log: slog.Default()}

	if _, err := a.Evaluate(context.Background(), EvaluateInput{
		QA: []EvaluatedQA{{Seq: 1, Question: "Q1", AnswerText: "A1"}},
	}); err == nil {
		t.Error("string dimension score should fail schema validation")
	}
}

func TestEvaluate_EmptyQA(t *testing.T) {
	a := &Agent{llm: &stubProvider{}, log: slog.Default()}
	if _, err := a.Evaluate(context.Background(), EvaluateInput{}); err == nil {
		t.Error("empty QA should fail")
	}
}
