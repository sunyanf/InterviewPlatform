package evaluation

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// goldenCase eval 数据集条目（见 evals/datasets/evaluator/golden.jsonl）
type goldenCase struct {
	CaseID             string             `json:"case_id"`
	Dataset            string             `json:"dataset"`
	Description        string             `json:"description"`
	Question           string             `json:"question"`
	Answer             string             `json:"answer"`
	ExpectedDimensions map[string]float64 `json:"expected_dimensions"`
	ExpectedTotal      float64            `json:"expected_total"`
}

// TestEvalGolden 确定性评估回归：golden 数据集的维度分经过总分公式后必须与期望一致。
// 修改 Rubric 权重导致本测试失败时，说明预期行为变化，必须同步更新 golden 数据集并说明原因（AGENTS.md #24）。
// LLM 评分一致性回归（真实 Provider）依赖 eval runner，见 docs/EVALUATION.md #5。
func TestEvalGolden(t *testing.T) {
	if !ValidateRubric(DefaultRubric) {
		t.Fatal("default rubric is invalid")
	}

	path := filepath.Join("..", "..", "..", "evals", "datasets", "evaluator", "golden.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open golden dataset: %v", err)
	}
	defer f.Close()

	cases := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var c goldenCase
		if err := json.Unmarshal(line, &c); err != nil {
			t.Fatalf("case %d: invalid json: %v", cases, err)
		}
		cases++

		// 维度名必须在 Rubric 内（保证 golden 与 Rubric 对齐）
		for name := range c.ExpectedDimensions {
			found := false
			for _, d := range DefaultRubric.Dimensions {
				if d.Name == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("case %s: dimension %q not in default rubric", c.CaseID, name)
			}
		}

		// 确定性总分回归
		total := CalculateTotalScore(c.ExpectedDimensions, DefaultRubric)
		if math.Abs(total-c.ExpectedTotal) > 0.005 {
			t.Errorf("case %s: total = %v, want %v", c.CaseID, total, c.ExpectedTotal)
		}

		// 总分必须在 0-100 区间
		if total < 0 || total > 100 {
			t.Errorf("case %s: total %v out of range [0,100]", c.CaseID, total)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan golden dataset: %v", err)
	}
	if cases < 3 {
		t.Fatalf("golden dataset too small: %d cases", cases)
	}
}
