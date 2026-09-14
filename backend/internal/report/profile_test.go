package report

import (
	"testing"

	"ai-interview-platform/internal/evaluation"
)

func TestBuildHistoryComparison_NoHistory(t *testing.T) {
	// 首次面试：无历史 → nil
	current := evaluation.Evaluation{TotalScore: 70.75, Dimensions: map[string]float64{"correctness": 70}}
	if got := BuildHistoryComparison(current, nil); got != nil {
		t.Errorf("empty history should return nil, got %+v", got)
	}
}

func TestBuildHistoryComparison_Single(t *testing.T) {
	current := evaluation.Evaluation{
		TotalScore: 70.75,
		Dimensions: map[string]float64{"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
	}
	histories := []evaluation.Evaluation{
		{TotalScore: 65.5, Dimensions: map[string]float64{"correctness": 60, "depth": 55, "logic": 70, "communication": 75}},
	}
	got := BuildHistoryComparison(current, histories)

	if got.ComparedCount != 1 {
		t.Errorf("count = %d, want 1", got.ComparedCount)
	}
	if got.AvgTotalScore != 65.5 {
		t.Errorf("avg_total = %v, want 65.5", got.AvgTotalScore)
	}
	if got.DeltaTotalScore != 5.25 {
		t.Errorf("delta_total = %v, want 5.25", got.DeltaTotalScore)
	}
	if got.AvgDimensions["correctness"] != 60 {
		t.Errorf("avg correctness = %v, want 60", got.AvgDimensions["correctness"])
	}
	if got.DeltaDimensions["correctness"] != 10 {
		t.Errorf("delta correctness = %v, want 10", got.DeltaDimensions["correctness"])
	}
}

func TestBuildHistoryComparison_Multiple(t *testing.T) {
	current := evaluation.Evaluation{
		TotalScore: 70.75,
		Dimensions: map[string]float64{"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
	}
	histories := []evaluation.Evaluation{
		{TotalScore: 60, Dimensions: map[string]float64{"correctness": 50, "depth": 50, "logic": 70, "communication": 70}},
		{TotalScore: 70, Dimensions: map[string]float64{"correctness": 70, "depth": 60, "logic": 80, "communication": 70}},
		{TotalScore: 80, Dimensions: map[string]float64{"correctness": 90, "depth": 70, "logic": 90, "communication": 70}},
	}
	got := BuildHistoryComparison(current, histories)

	if got.ComparedCount != 3 {
		t.Errorf("count = %d, want 3", got.ComparedCount)
	}
	// avg total = (60+70+80)/3 = 70
	if got.AvgTotalScore != 70 {
		t.Errorf("avg_total = %v, want 70", got.AvgTotalScore)
	}
	// delta = 70.75 - 70 = 0.75
	if got.DeltaTotalScore != 0.75 {
		t.Errorf("delta_total = %v, want 0.75", got.DeltaTotalScore)
	}
	// avg correctness = (50+70+90)/3 = 70 → delta 0
	if got.DeltaDimensions["correctness"] != 0 {
		t.Errorf("delta correctness = %v, want 0", got.DeltaDimensions["correctness"])
	}
	// avg depth = (50+60+70)/3 = 60 → delta 0
	if got.DeltaDimensions["depth"] != 0 {
		t.Errorf("delta depth = %v, want 0", got.DeltaDimensions["depth"])
	}
	// avg communication = (70+70+70)/3 = 70 → delta 10
	if got.DeltaDimensions["communication"] != 10 {
		t.Errorf("delta communication = %v, want 10", got.DeltaDimensions["communication"])
	}
}

func TestBuildHistoryComparison_MissingDimInHistory(t *testing.T) {
	// 历史评估缺某维度时，该维度不参与均值（不按 0 分稀释）
	current := evaluation.Evaluation{
		TotalScore: 70,
		Dimensions: map[string]float64{"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
	}
	histories := []evaluation.Evaluation{
		{TotalScore: 60, Dimensions: map[string]float64{"correctness": 50}}, // 缺其他维度
	}
	got := BuildHistoryComparison(current, histories)

	if got.AvgDimensions["correctness"] != 50 {
		t.Errorf("avg correctness = %v, want 50", got.AvgDimensions["correctness"])
	}
	if got.DeltaDimensions["correctness"] != 20 {
		t.Errorf("delta correctness = %v, want 20", got.DeltaDimensions["correctness"])
	}
	if _, ok := got.AvgDimensions["depth"]; ok {
		t.Error("missing dimension in history should not appear in avg")
	}
}

func TestAggregateKnowledgeGaps(t *testing.T) {
	analyses := []*analysisView{
		nil, // nil 安全
		{KnowledgeGaps: []string{"channel 关闭语义", " select 机制 "}},
		{KnowledgeGaps: []string{"channel 关闭语义", "GMP 调度", ""}},
		{KnowledgeGaps: nil},
	}
	got := AggregateKnowledgeGaps(analyses)

	if len(got) != 3 {
		t.Fatalf("gaps = %v, want 3 (dedup + trim)", got)
	}
	if got[0] != "channel 关闭语义" {
		t.Errorf("gap[0] = %q", got[0])
	}
	if got[1] != "select 机制" {
		t.Errorf("gap[1] = %q, want trimmed", got[1])
	}
	if got[2] != "GMP 调度" {
		t.Errorf("gap[2] = %q", got[2])
	}
}

func TestAggregateKnowledgeGaps_Limit(t *testing.T) {
	// 超过上限截断
	var views []*analysisView
	for i := 0; i < 20; i++ {
		views = append(views, &analysisView{KnowledgeGaps: []string{string(rune('a' + i))}})
	}
	got := AggregateKnowledgeGaps(views)
	if len(got) != knowledgeGapLimit {
		t.Errorf("gaps = %d, want %d", len(got), knowledgeGapLimit)
	}
}

func TestAggregateKnowledgeGaps_Empty(t *testing.T) {
	if got := AggregateKnowledgeGaps(nil); got != nil {
		t.Errorf("nil input should return nil, got %v", got)
	}
}
