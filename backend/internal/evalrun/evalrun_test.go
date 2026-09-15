package evalrun

import (
	"context"
	"path/filepath"
	"testing"

	"ai-interview-platform/pkg/llm"
)

// TestRunGolden_MockContract 用 mock provider 跑全量 golden 数据集：
// 所有样本必须通过结构化输出契约（4 维度/区间/整数/证据文本/prompt 版本）。
// 这是 CI 中防 prompt 与解析链路退化的默认回归；真实 provider 的评分一致性用 CLI -agreement。
func TestRunGolden_MockContract(t *testing.T) {
	cases, err := LoadGolden(filepath.Join("..", "..", "..", "evals", "datasets", "evaluator", "golden.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 20 {
		t.Fatalf("golden dataset too small: %d cases, want >= 20", len(cases))
	}

	r := New(llm.NewMockProvider())
	report, err := r.RunGolden(context.Background(), cases, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed != 0 {
		for _, res := range report.Results {
			if !res.OK {
				t.Errorf("case %s failed: %v", res.CaseID, res.Issues)
			}
		}
	}
	if report.Passed != len(cases) {
		t.Errorf("passed = %d, want %d", report.Passed, len(cases))
	}
	if report.Mode != "contract" {
		t.Errorf("mode = %q, want contract", report.Mode)
	}
	for _, res := range report.Results {
		if res.TotalScore < 0 || res.TotalScore > 100 {
			t.Errorf("case %s total %v out of range", res.CaseID, res.TotalScore)
		}
	}
}

// TestRunAdversarial 脚本化异常输出的接受/拒绝必须与数据集标注完全一致
func TestRunAdversarial(t *testing.T) {
	cases, err := LoadAdversarial(filepath.Join("..", "..", "..", "evals", "datasets", "evaluator", "adversarial.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("adversarial dataset is empty")
	}

	r := New(llm.NewMockProvider()) // provider 不参与，每个场景内部使用 ScriptedProvider
	report, err := r.RunAdversarial(context.Background(), cases)
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed != 0 {
		for _, res := range report.Results {
			if !res.OK {
				t.Errorf("case %s: %v", res.CaseID, res.Issues)
			}
		}
	}
}

// TestAdversarialScenariosCovered 数据集中引用的每个场景在脚本表中都有实现
func TestAdversarialScenariosCovered(t *testing.T) {
	cases, err := LoadAdversarial(filepath.Join("..", "..", "..", "evals", "datasets", "evaluator", "adversarial.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range cases {
		if _, ok := ScriptedContent(c.Scenario); !ok {
			t.Errorf("case %s references unimplemented scenario %q", c.CaseID, c.Scenario)
		}
		if seen[c.Scenario] {
			t.Errorf("scenario %q duplicated in dataset", c.Scenario)
		}
		seen[c.Scenario] = true
	}
}
