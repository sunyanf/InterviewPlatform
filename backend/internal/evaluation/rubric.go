package evaluation

import "math"

// Dimension 评估维度
type Dimension struct {
	Name   string  `json:"name"`
	Label  string  `json:"label"`
	Weight float64 `json:"weight"`
}

// Rubric 评分规则（快照随评估落库，便于追溯与未来按岗位差异化）
type Rubric struct {
	Dimensions []Dimension `json:"dimensions"`
}

// DefaultRubric 默认技术岗 Rubric（映射 docs/EVALUATION.md 技术示例，维度名对齐 AI_DATA_CONTRACTS.md #5）
// correctness=技术正确性、depth=技术深度（含项目经验）、logic=逻辑思维（含系统思维）、communication=表达
var DefaultRubric = Rubric{
	Dimensions: []Dimension{
		{Name: "correctness", Label: "技术正确性", Weight: 0.30},
		{Name: "depth", Label: "技术深度", Weight: 0.25},
		{Name: "logic", Label: "逻辑思维", Weight: 0.25},
		{Name: "communication", Label: "表达", Weight: 0.20},
	},
}

// CalculateTotalScore 确定性总分计算：Σ 维度分 × 权重（AGENTS.md #15，LLM 不算总分）
// 结果四舍五入保留 2 位小数；权重之和不等于 1 时按实际权重求和（调用方负责 Rubric 合法性）
func CalculateTotalScore(dimensions map[string]float64, rubric Rubric) float64 {
	var total float64
	for _, dim := range rubric.Dimensions {
		score, ok := dimensions[dim.Name]
		if !ok {
			continue
		}
		total += score * dim.Weight
	}
	return math.Round(total*100) / 100
}

// ValidateRubric 校验 Rubric 合法性：维度名非空、权重在 (0,1]、权重之和为 1
func ValidateRubric(r Rubric) bool {
	if len(r.Dimensions) == 0 {
		return false
	}
	var sum float64
	for _, d := range r.Dimensions {
		if d.Name == "" || d.Weight <= 0 || d.Weight > 1 {
			return false
		}
		sum += d.Weight
	}
	return math.Abs(sum-1) < 1e-9
}
