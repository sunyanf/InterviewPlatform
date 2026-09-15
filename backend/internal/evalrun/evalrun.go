// Package evalrun 提供评估器 Eval 回归能力：
//   - golden 数据集契约回归（任意 Provider，默认 mock）：验证 agent.Evaluate 的结构化输出契约；
//   - golden 评分一致性（真实 Provider）：维度分与人工标注对比，输出容差通过率与 MAE；
//   - adversarial 契约异常回归（脚本化 Provider）：验证空输出/错类型/缺字段等异常被正确接受或拒绝。
//
// 对应 docs/EVALUATION.md #5。LLM 不产出总分，runner 中所有总分均由确定性公式重算。
package evalrun

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"

	"ai-interview-platform/internal/agent"
	"ai-interview-platform/internal/evaluation"
	"ai-interview-platform/pkg/llm"
)

// Case golden 数据集中的人工标注样本
type Case struct {
	CaseID             string             `json:"case_id"`
	Dataset            string             `json:"dataset"`
	Description        string             `json:"description"`
	Question           string             `json:"question"`
	Answer             string             `json:"answer"`
	ExpectedDimensions map[string]float64 `json:"expected_dimensions"`
	ExpectedTotal      float64            `json:"expected_total"`
}

// AdversarialCase 契约异常样本：scenario 对应脚本化 Provider 的固定输出
type AdversarialCase struct {
	CaseID      string `json:"case_id"`
	Dataset     string `json:"dataset"`
	Description string `json:"description"`
	Scenario    string `json:"scenario"`
	ExpectValid bool   `json:"expect_valid"`
}

// LoadGolden 从 JSONL 文件加载 golden 样本
func LoadGolden(path string) ([]Case, error) {
	var cases []Case
	if err := loadJSONL(path, &cases); err != nil {
		return nil, err
	}
	return cases, nil
}

// LoadAdversarial 从 JSONL 文件加载契约异常样本
func LoadAdversarial(path string) ([]AdversarialCase, error) {
	var cases []AdversarialCase
	if err := loadJSONL(path, &cases); err != nil {
		return nil, err
	}
	return cases, nil
}

func loadJSONL(path string, dst any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open dataset %s: %w", path, err)
	}
	defer f.Close()

	switch out := dst.(type) {
	case *[]Case:
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := bytesTrim(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var c Case
			if err := json.Unmarshal(line, &c); err != nil {
				return fmt.Errorf("invalid golden jsonl line: %w", err)
			}
			*out = append(*out, c)
		}
		return scanner.Err()
	case *[]AdversarialCase:
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := bytesTrim(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var c AdversarialCase
			if err := json.Unmarshal(line, &c); err != nil {
				return fmt.Errorf("invalid adversarial jsonl line: %w", err)
			}
			*out = append(*out, c)
		}
		return scanner.Err()
	default:
		return fmt.Errorf("loadJSONL: unsupported destination type %T", dst)
	}
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// ScriptedProvider 按固定内容应答的 Provider，用于 adversarial 契约回归
type ScriptedProvider struct {
	name    string
	content string
}

// NewScriptedProvider 创建脚本化 Provider
func NewScriptedProvider(content string) *ScriptedProvider {
	return &ScriptedProvider{name: "scripted", content: content}
}

// Chat 返回固定内容
func (p *ScriptedProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: p.content, Model: p.name}, nil
}

// Name 返回 Provider 名称
func (p *ScriptedProvider) Name() string { return p.name }

// ScriptedContent 返回 adversarial 场景名对应的模型输出内容；未知场景返回 false
func ScriptedContent(scenario string) (string, bool) {
	c, ok := scripts[scenario]
	return c, ok
}

var scripts = map[string]string{
	"valid_basic": `{
		"dimensions": {"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
		"evidence": [{"question_seq": 1, "issue": "缺少关闭语义", "evidence": "候选人只说明 channel 用于通信", "reference": ""}],
		"recommendations": ["学习 channel 关闭语义", "先结论后展开"]
	}`,
	"all_zero": `{
		"dimensions": {"correctness": 0, "depth": 0, "logic": 0, "communication": 0},
		"evidence": [],
		"recommendations": ["从基础概念开始系统学习"]
	}`,
	"all_one_hundred": `{
		"dimensions": {"correctness": 100, "depth": 100, "logic": 100, "communication": 100},
		"evidence": [{"question_seq": 1, "issue": "无明显问题", "evidence": "回答覆盖全部要点并给出量化结果", "reference": "rubric"}],
		"recommendations": ["保持当前答题结构"]
	}`,
	"fenced_json": "```json\n" + `{
		"dimensions": {"correctness": 66, "depth": 55, "logic": 60, "communication": 70},
		"evidence": [],
		"recommendations": ["补充项目细节"]
	}` + "\n```",
	"empty_string":      "",
	"plain_text":        "我认为这个候选人表现还可以，具体分数你们自己定吧。",
	"empty_object":      `{}`,
	"missing_dimension": `{"dimensions": {"correctness": 70, "depth": 60, "logic": 75}, "evidence": [], "recommendations": []}`,
	"string_score":      `{"dimensions": {"correctness": "高", "depth": 60, "logic": 75, "communication": 80}, "evidence": [], "recommendations": []}`,
	"out_of_range": `{
		"dimensions": {"correctness": 150, "depth": -20, "logic": 75, "communication": 80},
		"evidence": [],
		"recommendations": []
	}`,
	"unknown_dimensions": `{
		"dimensions": {"correctness": 70, "depth": 60, "logic": 75, "communication": 80, "attitude": 99, "charisma": 88},
		"evidence": [],
		"recommendations": []
	}`,
	"empty_collections": `{
		"dimensions": {"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
		"evidence": [],
		"recommendations": []
	}`,
	"null_literal": `null`,
	"decimal_scores": `{
		"dimensions": {"correctness": 70.4, "depth": 60.6, "logic": 75.5, "communication": 80},
		"evidence": [],
		"recommendations": []
	}`,
	"blank_evidence_items": `{
		"dimensions": {"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
		"evidence": [
			{"question_seq": 1, "issue": "有效", "evidence": "回答原文片段", "reference": ""},
			{"question_seq": 2, "issue": "无效", "evidence": "   ", "reference": ""}
		],
		"recommendations": ["", "有效建议"]
	}`,
	"json_with_prose": `好的，以下是我的评估结果：
{"dimensions": {"correctness": 70, "depth": 60, "logic": 75, "communication": 80}, "evidence": [], "recommendations": []}
希望对你有帮助！`,
	"null_dimensions": `{"dimensions": null, "evidence": [], "recommendations": []}`,
}

// Options golden 回归选项
type Options struct {
	// CheckAgreement 是否做人工标注一致性对比（仅真实 Provider 有意义；mock 输出固定，不反映答案质量）
	CheckAgreement bool
	// Tolerance 单维度允许偏差（默认 15）
	Tolerance float64
}

// CaseResult 单样本结果
type CaseResult struct {
	CaseID          string             `json:"case_id"`
	OK              bool               `json:"ok"`
	Issues          []string           `json:"issues,omitempty"`
	Dimensions      map[string]float64 `json:"dimensions,omitempty"`
	TotalScore      float64            `json:"total_score,omitempty"`
	ExpectedTotal   float64            `json:"expected_total,omitempty"`
	WithinTolerance bool               `json:"within_tolerance,omitempty"`
	DimMAE          float64            `json:"dim_mae,omitempty"`
}

// Report 一次回归的完整报告
type Report struct {
	Provider      string       `json:"provider"`
	Model         string       `json:"model"`
	PromptVersion string       `json:"prompt_version"`
	Dataset       string       `json:"dataset"`
	Mode          string       `json:"mode"` // contract / agreement
	Tolerance     float64      `json:"tolerance,omitempty"`
	Total         int          `json:"total"`
	Passed        int          `json:"passed"`
	Failed        int          `json:"failed"`
	AgreementRate float64      `json:"agreement_rate,omitempty"` // -1 表示不适用
	MeanAbsError  float64      `json:"mean_abs_error,omitempty"` // -1 表示不适用
	Results       []CaseResult `json:"results"`
}

// Runner Eval 回归执行器
type Runner struct {
	provider llm.Provider
}

// New 创建回归执行器
func New(provider llm.Provider) *Runner {
	return &Runner{provider: provider}
}

// RunGolden 对 golden 数据集跑评估器：契约必检；opts.CheckAgreement 时额外做评分一致性对比
func (r *Runner) RunGolden(ctx context.Context, cases []Case, opts Options) (*Report, error) {
	if len(cases) == 0 {
		return nil, fmt.Errorf("golden dataset is empty")
	}
	if opts.Tolerance <= 0 {
		opts.Tolerance = 15
	}

	ag := agent.New(r.provider, slog.Default())
	report := &Report{
		Provider:      r.provider.Name(),
		Dataset:       cases[0].Dataset,
		Mode:          "contract",
		Tolerance:     opts.Tolerance,
		AgreementRate: -1,
		MeanAbsError:  -1,
	}
	if opts.CheckAgreement {
		report.Mode = "agreement"
	}

	var dimErrSum float64
	var dimCount int
	var agreeCases int

	for _, c := range cases {
		res := CaseResult{CaseID: c.CaseID, OK: true}
		out, err := ag.Evaluate(ctx, agent.EvaluateInput{
			JobTitle:      "后端工程师",
			InterviewType: "technical",
			QA: []agent.EvaluatedQA{{
				Seq: 1, Question: c.Question, QuestionType: "technical", AnswerText: c.Answer,
			}},
		})
		if err != nil {
			res.OK = false
			res.Issues = append(res.Issues, "evaluate error: "+err.Error())
			report.Results = append(report.Results, res)
			continue
		}

		report.Model = firstNonEmpty(report.Model, out.Model)
		report.PromptVersion = firstNonEmpty(report.PromptVersion, out.PromptVersion)
		res.Dimensions = out.Dimensions
		res.TotalScore = evaluation.CalculateTotalScore(out.Dimensions, evaluation.DefaultRubric)
		res.ExpectedTotal = c.ExpectedTotal

		for _, issue := range checkContract(out) {
			res.OK = false
			res.Issues = append(res.Issues, issue)
		}

		if opts.CheckAgreement {
			within := true
			for _, dim := range evaluation.DefaultRubric.Dimensions {
				got := out.Dimensions[dim.Name]
				want, ok := c.ExpectedDimensions[dim.Name]
				if !ok {
					res.OK = false
					res.Issues = append(res.Issues, fmt.Sprintf("case missing expected dimension %q", dim.Name))
					within = false
					continue
				}
				diff := math.Abs(got - want)
				dimErrSum += diff
				dimCount++
				if diff > opts.Tolerance {
					within = false
					res.Issues = append(res.Issues, fmt.Sprintf("dimension %q = %v, want %v (±%v)", dim.Name, got, want, opts.Tolerance))
				}
			}
			res.WithinTolerance = within
			res.DimMAE = round2(dimMAE(out.Dimensions, c.ExpectedDimensions))
			if within {
				agreeCases++
			}
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
	if opts.CheckAgreement && dimCount > 0 {
		report.AgreementRate = round2(float64(agreeCases) / float64(report.Total))
		report.MeanAbsError = round2(dimErrSum / float64(dimCount))
	}
	return report, nil
}

// RunAdversarial 用脚本化 Provider 跑契约异常集：agent.Evaluate 的接受/拒绝必须与 expect_valid 一致
func (r *Runner) RunAdversarial(ctx context.Context, cases []AdversarialCase) (*Report, error) {
	if len(cases) == 0 {
		return nil, fmt.Errorf("adversarial dataset is empty")
	}
	report := &Report{
		Provider:      "scripted",
		Dataset:       cases[0].Dataset,
		Mode:          "contract",
		AgreementRate: -1,
		MeanAbsError:  -1,
	}

	for _, c := range cases {
		res := CaseResult{CaseID: c.CaseID, OK: true}
		content, ok := ScriptedContent(c.Scenario)
		if !ok {
			res.OK = false
			res.Issues = []string{"unknown scenario: " + c.Scenario}
			report.Results = append(report.Results, res)
			continue
		}

		ag := agent.New(NewScriptedProvider(content), slog.Default())
		out, err := ag.Evaluate(ctx, agent.EvaluateInput{
			QA: []agent.EvaluatedQA{{Seq: 1, Question: "Q", AnswerText: "A"}},
		})
		gotValid := err == nil
		if gotValid != c.ExpectValid {
			res.OK = false
			res.Issues = append(res.Issues, fmt.Sprintf("scenario %q: expect valid=%v, got valid=%v (err=%v)",
				c.Scenario, c.ExpectValid, gotValid, err))
		}
		if gotValid {
			res.Dimensions = out.Dimensions
			for _, issue := range checkContract(out) {
				res.OK = false
				res.Issues = append(res.Issues, issue)
			}
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

// checkContract 对 agent 已归一化的输出做 runner 级独立复核（防御式契约检查）
func checkContract(out *agent.EvaluationOutput) []string {
	var issues []string
	if out == nil {
		return []string{"nil evaluation output"}
	}
	if len(out.Dimensions) != len(evaluation.DefaultRubric.Dimensions) {
		issues = append(issues, fmt.Sprintf("dimensions count = %d, want %d",
			len(out.Dimensions), len(evaluation.DefaultRubric.Dimensions)))
	}
	for _, dim := range evaluation.DefaultRubric.Dimensions {
		v, ok := out.Dimensions[dim.Name]
		if !ok {
			issues = append(issues, "missing dimension "+dim.Name)
			continue
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			issues = append(issues, fmt.Sprintf("dimension %q not finite: %v", dim.Name, v))
		}
		if v < 0 || v > 100 {
			issues = append(issues, fmt.Sprintf("dimension %q out of range: %v", dim.Name, v))
		}
		if v != math.Trunc(v) {
			issues = append(issues, fmt.Sprintf("dimension %q not integer after normalization: %v", dim.Name, v))
		}
	}
	for i, e := range out.Evidence {
		if strings.TrimSpace(e.Evidence) == "" {
			issues = append(issues, fmt.Sprintf("evidence[%d] has blank evidence text", i))
		}
	}
	if strings.TrimSpace(out.PromptVersion) == "" {
		issues = append(issues, "empty prompt_version")
	}
	if strings.TrimSpace(out.Model) == "" {
		issues = append(issues, "empty model")
	}
	return issues
}

func dimMAE(got, want map[string]float64) float64 {
	var sum float64
	var n int
	for name, w := range want {
		if g, ok := got[name]; ok {
			sum += math.Abs(g - w)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
