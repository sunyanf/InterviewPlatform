// Command evalrunner 运行评估器 Eval 回归（docs/EVALUATION.md #5）。
//
// 默认（mock provider，CI 可直接运行）：
//
//	go run ./cmd/evalrunner
//
//	执行 golden 数据集结构化契约回归 + adversarial 契约异常回归。
//
// 真实 Provider 评分一致性回归（阶段 B 配置密钥后）：
//
//	LLM_PROVIDER=openai LLM_API_KEY=... LLM_MODEL=... \
//	go run ./cmd/evalrunner -agreement -tolerance 15 -min-agreement 0.8 \
//	  -out ../evals/results/evaluator-2026-09-16.json
//
// 退出码：0 全部通过；1 存在契约失败，或 agreement 模式一致率低于阈值；2 参数/初始化错误。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/evalrun"
	"ai-interview-platform/pkg/llm"
)

func main() {
	goldenPath := flag.String("golden", filepath.Join("..", "evals", "datasets", "evaluator", "golden.jsonl"), "golden 数据集 JSONL 路径")
	advPath := flag.String("adversarial", filepath.Join("..", "evals", "datasets", "evaluator", "adversarial.jsonl"), "adversarial 数据集 JSONL 路径")
	out := flag.String("out", "", "报告 JSON 输出路径（可选）")
	agreement := flag.Bool("agreement", false, "启用人工标注评分一致性对比（应配合真实 Provider）")
	tolerance := flag.Float64("tolerance", 15, "单维度允许偏差（分）")
	minAgreement := flag.Float64("min-agreement", 0.8, "agreement 模式最低容差通过率")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(2)
	}
	provider, err := llm.NewProvider(cfg.LLM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init llm provider: %v\n", err)
		os.Exit(2)
	}

	// mock 输出固定，与答案质量无关，开启一致性对比没有意义
	if *agreement && provider.Name() == "mock" {
		fmt.Fprintln(os.Stderr, "WARN: -agreement ignored for mock provider (mock scores do not depend on answers); running contract mode only")
		*agreement = false
	}

	golden, err := evalrun.LoadGolden(*goldenPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load golden: %v\n", err)
		os.Exit(2)
	}
	adversarial, err := evalrun.LoadAdversarial(*advPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load adversarial: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	runner := evalrun.New(provider)
	goldenReport, err := runner.RunGolden(ctx, golden, evalrun.Options{
		CheckAgreement: *agreement,
		Tolerance:      *tolerance,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "run golden: %v\n", err)
		os.Exit(2)
	}
	advReport, err := runner.RunAdversarial(ctx, adversarial)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run adversarial: %v\n", err)
		os.Exit(2)
	}

	printReport(goldenReport)
	printReport(advReport)

	if *out != "" {
		if err := writeReports(*out, goldenReport, advReport); err != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", err)
			os.Exit(2)
		}
		fmt.Printf("\nreport written to %s\n", *out)
	}

	failed := goldenReport.Failed + advReport.Failed
	if *agreement && goldenReport.AgreementRate >= 0 && goldenReport.AgreementRate < *minAgreement {
		fmt.Fprintf(os.Stderr, "\nagreement rate %.2f below threshold %.2f\n", goldenReport.AgreementRate, *minAgreement)
		failed++
	}
	if failed > 0 {
		os.Exit(1)
	}
}

func printReport(r *evalrun.Report) {
	fmt.Printf("\n=== dataset=%s mode=%s provider=%s model=%s ===\n",
		r.Dataset, r.Mode, r.Provider, r.Model)
	fmt.Printf("total=%d passed=%d failed=%d\n", r.Total, r.Passed, r.Failed)
	if r.AgreementRate >= 0 {
		fmt.Printf("agreement_rate=%.2f mean_abs_error=%.2f (tolerance ±%.0f)\n",
			r.AgreementRate, r.MeanAbsError, r.Tolerance)
	}
	for _, res := range r.Results {
		if res.OK {
			continue
		}
		fmt.Printf("  FAIL %s: %v\n", res.CaseID, res.Issues)
	}
}

func writeReports(path string, reports ...*evalrun.Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"reports":      reports,
	})
}
