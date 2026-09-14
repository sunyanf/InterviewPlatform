package evaluation

import (
	"testing"
)

func TestCalculateTotalScore(t *testing.T) {
	dims := map[string]float64{"correctness": 70, "depth": 60, "logic": 75, "communication": 80}
	// 70*0.30 + 60*0.25 + 75*0.25 + 80*0.20 = 21 + 15 + 18.75 + 16 = 70.75
	got := CalculateTotalScore(dims, DefaultRubric)
	if got != 70.75 {
		t.Errorf("total = %v, want 70.75", got)
	}
}

func TestCalculateTotalScore_Rounding(t *testing.T) {
	dims := map[string]float64{"correctness": 33.333, "depth": 66.666, "logic": 50, "communication": 50}
	// 33.333*0.3 + 66.666*0.25 + 50*0.25 + 50*0.2 = 9.9999 + 16.6665 + 12.5 + 10 = 49.1664 → 49.17
	got := CalculateTotalScore(dims, DefaultRubric)
	if got != 49.17 {
		t.Errorf("total = %v, want 49.17", got)
	}
}

func TestCalculateTotalScore_MissingDimensionSkipped(t *testing.T) {
	// 缺维度时跳过（调用方负责校验维度完整性），不 panic
	dims := map[string]float64{"correctness": 100}
	got := CalculateTotalScore(dims, DefaultRubric)
	if got != 30 {
		t.Errorf("total = %v, want 30", got)
	}
}

func TestCalculateTotalScore_Zero(t *testing.T) {
	dims := map[string]float64{"correctness": 0, "depth": 0, "logic": 0, "communication": 0}
	if got := CalculateTotalScore(dims, DefaultRubric); got != 0 {
		t.Errorf("total = %v, want 0", got)
	}
}

func TestValidateRubric(t *testing.T) {
	if !ValidateRubric(DefaultRubric) {
		t.Error("default rubric should be valid")
	}

	// 权重之和不为 1
	bad := Rubric{Dimensions: []Dimension{
		{Name: "a", Weight: 0.5}, {Name: "b", Weight: 0.4},
	}}
	if ValidateRubric(bad) {
		t.Error("weights not summing to 1 should be invalid")
	}

	// 空维度
	if ValidateRubric(Rubric{}) {
		t.Error("empty rubric should be invalid")
	}

	// 非法权重
	if ValidateRubric(Rubric{Dimensions: []Dimension{{Name: "a", Weight: 1.5}}}) {
		t.Error("weight > 1 should be invalid")
	}

	// 空维度名
	if ValidateRubric(Rubric{Dimensions: []Dimension{{Name: "", Weight: 1}}}) {
		t.Error("empty name should be invalid")
	}
}
