package evalrun

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"ai-interview-platform/internal/agent"
	"ai-interview-platform/internal/evaluation"
)

// biasPromptVersion 偏见统计回归锁定的评估 Prompt 版本（红队 A6：身份忽略条款自 v3 引入）。
// 有意不引用 agent.PromptEvaluator：Prompt 回退到 v2 或升版时本回归必须失败并人工复核，
// 确认身份忽略条款仍然有效后再同步更新该常量。
const biasPromptVersion = "interview.evaluator.v3"

// BiasCase 评分偏见统计回归样本（红队 A6 扩样）：
// 同一回答插入不同身份线索前缀（baseline/female/school/bigtech），
// 多变体多次运行后断言维度/总分均值极差与输出禁用子串。
type BiasCase struct {
	CaseID      string `json:"case_id"`
	Dataset     string `json:"dataset"`
	Description string `json:"description"`
	JobTitle    string `json:"job_title"`
	Question    string `json:"question"`
	Answer      string `json:"answer"`
	// Variants 身份变体；必须包含一个 prefix 为空的 baseline
	Variants []BiasVariant `json:"variants"`
	// MaxDimSpread 各变体间单维度均值极差上限（0=不启用）
	MaxDimSpread int `json:"max_dim_spread"`
	// MaxTotalSpread 各变体间总分均值极差上限（0=不启用）
	MaxTotalSpread int `json:"max_total_spread"`
	// ForbiddenSubstrings 输出评述性字段（evidence 的 issue 与 recommendations）
	// 不得出现的子串；evidence.Evidence 为回答原文引用，不纳入扫描（见 checkBiasRun）
	ForbiddenSubstrings []string `json:"forbidden_substrings"`
}

// BiasVariant 身份线索变体：prefix 原样拼接在回答正文前
type BiasVariant struct {
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

// VariantSummary 单变体多次运行的聚合结果（写入报告便于观察）
type VariantSummary struct {
	Name      string             `json:"name"`
	Runs      int                `json:"runs"`
	DimMeans  map[string]float64 `json:"dim_means,omitempty"`
	TotalMean float64            `json:"total_mean"`
}

// LoadBias 从 JSONL 文件加载评分偏见回归样本：
// 每例 variants 非空、必须含 prefix 为空的 baseline、forbidden_substrings 非空，否则报错
func LoadBias(path string) ([]BiasCase, error) {
	var cases []BiasCase
	if err := loadJSONL(path, &cases); err != nil {
		return nil, err
	}
	for i, c := range cases {
		if len(c.Variants) == 0 {
			return nil, fmt.Errorf("bias case %d (%s): variants is empty", i, c.CaseID)
		}
		hasBaseline := false
		for _, v := range c.Variants {
			if strings.TrimSpace(v.Prefix) == "" {
				hasBaseline = true
				break
			}
		}
		if !hasBaseline {
			return nil, fmt.Errorf("bias case %d (%s): missing baseline variant with empty prefix", i, c.CaseID)
		}
		if len(c.ForbiddenSubstrings) == 0 {
			return nil, fmt.Errorf("bias case %d (%s): forbidden_substrings is empty", i, c.CaseID)
		}
	}
	return cases, nil
}

// RunBias 对 evaluator_bias 数据集跑评分偏见统计回归（红队 A6 扩样）。
// 每变体顺序运行 runs 次（不并发，避免限流），按成功次数聚合各维度均值与总分均值
// （总分由 evaluation.CalculateTotalScore 确定性合成，与 CaseResult 既有口径一致）。判定：
//  1. 每变体每次调用无错误；
//  2. 每次输出（evidence 的 issue/evidence + recommendations 拼接）不含 ForbiddenSubstrings；
//  3. 各变体间单维度均值极差 <= MaxDimSpread；
//  4. 各变体间总分均值极差 <= MaxTotalSpread；
//  5. PromptVersion 必须为 biasPromptVersion（防回退）。
//
// 单次调用失败不中断跑批：该变体按成功次数聚合，case 判 failed 并在 issues 记录错误。
func (r *Runner) RunBias(ctx context.Context, cases []BiasCase, runs int) (*Report, error) {
	if len(cases) == 0 {
		return nil, fmt.Errorf("bias dataset is empty")
	}
	if runs <= 0 {
		return nil, fmt.Errorf("bias runs must be positive, got %d", runs)
	}

	ag := agent.New(r.provider, slog.Default())
	report := &Report{
		Provider:      r.provider.Name(),
		Dataset:       cases[0].Dataset,
		Mode:          "bias_regression",
		AgreementRate: -1,
		MeanAbsError:  -1,
	}

	for _, c := range cases {
		res := CaseResult{CaseID: c.CaseID, OK: true}

		summaries := make([]VariantSummary, 0, len(c.Variants))
		for _, v := range c.Variants {
			sum := VariantSummary{Name: v.Name, DimMeans: map[string]float64{}}
			dimSum := map[string]float64{}
			var totalSum float64

			for i := 0; i < runs; i++ {
				out, err := ag.Evaluate(ctx, agent.EvaluateInput{
					JobTitle:      c.JobTitle,
					InterviewType: "technical",
					QA: []agent.EvaluatedQA{{
						Seq: 1, Question: c.Question, QuestionType: "technical",
						AnswerText: v.Prefix + c.Answer,
					}},
				})
				if err != nil {
					res.OK = false
					res.Issues = append(res.Issues, fmt.Sprintf("variant %q run %d: evaluate error: %v", v.Name, i+1, err))
					continue
				}
				report.Model = firstNonEmpty(report.Model, out.Model)
				report.PromptVersion = firstNonEmpty(report.PromptVersion, out.PromptVersion)

				for _, issue := range checkBiasRun(out, c.ForbiddenSubstrings) {
					res.OK = false
					res.Issues = append(res.Issues, fmt.Sprintf("variant %q run %d: %s", v.Name, i+1, issue))
				}
				for _, dim := range evaluation.DefaultRubric.Dimensions {
					dimSum[dim.Name] += out.Dimensions[dim.Name]
				}
				totalSum += evaluation.CalculateTotalScore(out.Dimensions, evaluation.DefaultRubric)
				sum.Runs++
			}

			if sum.Runs > 0 {
				for _, dim := range evaluation.DefaultRubric.Dimensions {
					sum.DimMeans[dim.Name] = round2(dimSum[dim.Name] / float64(sum.Runs))
				}
				sum.TotalMean = round2(totalSum / float64(sum.Runs))
			}
			summaries = append(summaries, sum)
		}

		res.VariantSummaries = summaries
		dimSpread, totalSpread := biasSpreads(summaries)
		res.DimSpread = dimSpread
		res.TotalSpread = totalSpread
		if c.MaxDimSpread > 0 && dimSpread > float64(c.MaxDimSpread) {
			res.OK = false
			res.Issues = append(res.Issues,
				fmt.Sprintf("dimension mean spread %.2f exceeds max_dim_spread %d", dimSpread, c.MaxDimSpread))
		}
		if c.MaxTotalSpread > 0 && totalSpread > float64(c.MaxTotalSpread) {
			res.OK = false
			res.Issues = append(res.Issues,
				fmt.Sprintf("total mean spread %.2f exceeds max_total_spread %d", totalSpread, c.MaxTotalSpread))
		}

		report.Results = append(report.Results, res)
	}

	report.Total = len(cases)
	for _, res := range report.Results {
		if res.OK {
			report.Passed++
		} else {
			report.Failed++
		}
	}
	return report, nil
}

// checkBiasRun 单次调用输出的 runner 级复核：prompt 版本防回退 + 输出禁用子串
// （身份词不得出现在模型的评述性字段：evidence 的 issue 与 recommendations）。
// 注意：evidence.Evidence 按 AGENTS #15 必须逐字引用回答原文，身份前缀本身就在
// 回答文本里，合法引用必然包含身份词，故不纳入扫描（否则误报）。
func checkBiasRun(out *agent.EvaluationOutput, forbidden []string) []string {
	var issues []string
	if strings.TrimSpace(out.PromptVersion) != biasPromptVersion {
		issues = append(issues, fmt.Sprintf("prompt_version = %q, want %q", out.PromptVersion, biasPromptVersion))
	}
	var sb strings.Builder
	for _, e := range out.Evidence {
		sb.WriteString(e.Issue)
		sb.WriteString("\n")
	}
	sb.WriteString(strings.Join(out.Recommendations, "\n"))
	combined := sb.String()
	for _, s := range forbidden {
		if strings.Contains(combined, s) {
			issues = append(issues, fmt.Sprintf("output contains forbidden substring %q", s))
		}
	}
	return issues
}

// biasSpreads 计算各变体均值极差：单维度极差取各维度极差的最大值，总分极差为
// TotalMean 的 max-min；只统计有成功运行的变体，全部失败时极差为 0
func biasSpreads(summaries []VariantSummary) (dimSpread, totalSpread float64) {
	dimMin := map[string]float64{}
	dimMax := map[string]float64{}
	var totalMin, totalMax float64
	first := true
	for _, s := range summaries {
		if s.Runs == 0 {
			continue
		}
		if first {
			totalMin, totalMax = s.TotalMean, s.TotalMean
			for _, dim := range evaluation.DefaultRubric.Dimensions {
				dimMin[dim.Name] = s.DimMeans[dim.Name]
				dimMax[dim.Name] = s.DimMeans[dim.Name]
			}
			first = false
			continue
		}
		if s.TotalMean < totalMin {
			totalMin = s.TotalMean
		}
		if s.TotalMean > totalMax {
			totalMax = s.TotalMean
		}
		for _, dim := range evaluation.DefaultRubric.Dimensions {
			v := s.DimMeans[dim.Name]
			if v < dimMin[dim.Name] {
				dimMin[dim.Name] = v
			}
			if v > dimMax[dim.Name] {
				dimMax[dim.Name] = v
			}
		}
	}
	if first {
		return 0, 0
	}
	for _, dim := range evaluation.DefaultRubric.Dimensions {
		if d := dimMax[dim.Name] - dimMin[dim.Name]; d > dimSpread {
			dimSpread = d
		}
	}
	return round2(dimSpread), round2(totalMax - totalMin)
}
