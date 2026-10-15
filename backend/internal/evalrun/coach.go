package evalrun

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"ai-interview-platform/internal/agent"
)

// coachPromptVersion 更优答案示范 Prompt 版本（interview.answer_coach.v1）
const coachPromptVersion = agent.PromptAnswerCoach

// maxImprovedAnswerRunes improved_answer 允许的最大 rune 数（建议 150-400 字，超长视为越界发挥）
const maxImprovedAnswerRunes = 2000

// maxKeyChanges key_changes 允许的最大条数（Prompt 约定 1-4 条）
const maxKeyChanges = 4

// CoachCase 更优答案示范（answer_coach）数据集样本
type CoachCase struct {
	CaseID         string   `json:"case_id"`
	Dataset        string   `json:"dataset"`
	Description    string   `json:"description"`
	JobTitle       string   `json:"job_title"`
	Question       string   `json:"question"`
	ExpectedPoints []string `json:"expected_points"`
	Answer         string   `json:"answer"`
	Difficulty     string   `json:"difficulty"`
	// ForbiddenSubstrings 输出（improved_answer 与 key_changes）中不得出现的子串；
	// 用于「不得虚构候选人经历/公司」的自动化近似检查
	ForbiddenSubstrings []string `json:"forbidden_substrings"`
}

// LoadCoach 从 JSONL 文件加载 answer_coach 数据集样本
func LoadCoach(path string) ([]CoachCase, error) {
	var cases []CoachCase
	if err := loadJSONL(path, &cases); err != nil {
		return nil, err
	}
	return cases, nil
}

// RunCoach 对 answer_coach 数据集跑更优答案示范契约回归：
// 验证 agent.CoachAnswer 的结构化输出契约（improved_answer 非空且限长、key_changes 条数、
// prompt 版本、模型非空）与内容安全近似检查（禁用子串、untrusted_ 数据区标签不得泄漏进产物）。
func (r *Runner) RunCoach(ctx context.Context, cases []CoachCase) (*Report, error) {
	if len(cases) == 0 {
		return nil, fmt.Errorf("coach dataset is empty")
	}

	ag := agent.New(r.provider, slog.Default())
	report := &Report{
		Provider:      r.provider.Name(),
		Dataset:       cases[0].Dataset,
		Mode:          "coach_contract",
		AgreementRate: -1,
		MeanAbsError:  -1,
	}

	for _, c := range cases {
		res := CaseResult{CaseID: c.CaseID, OK: true}
		out, err := ag.CoachAnswer(ctx, agent.AnswerCoachInput{
			JobTitle:       c.JobTitle,
			Question:       c.Question,
			ExpectedPoints: c.ExpectedPoints,
			AnswerText:     c.Answer,
			Difficulty:     c.Difficulty,
		})
		if err != nil {
			res.OK = false
			res.Issues = append(res.Issues, "coach error: "+err.Error())
			report.Results = append(report.Results, res)
			continue
		}

		report.Model = firstNonEmpty(report.Model, out.Model)
		report.PromptVersion = firstNonEmpty(report.PromptVersion, out.PromptVersion)

		res.ImprovedAnswerRunes = len([]rune(out.ImprovedAnswer))
		res.KeyChangesCount = len(out.KeyChanges)

		for _, issue := range checkCoachContract(out, c.ForbiddenSubstrings) {
			res.OK = false
			res.Issues = append(res.Issues, issue)
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

// checkCoachContract 对 agent 已归一化的 coach 输出做 runner 级独立复核（防御式契约检查）
func checkCoachContract(out *agent.AnswerCoaching, forbidden []string) []string {
	var issues []string
	if out == nil {
		return []string{"nil coaching output"}
	}

	if runes := len([]rune(out.ImprovedAnswer)); runes == 0 {
		issues = append(issues, "empty improved_answer")
	} else if runes > maxImprovedAnswerRunes {
		issues = append(issues, fmt.Sprintf("improved_answer runes = %d, want <= %d", runes, maxImprovedAnswerRunes))
	}

	if out.KeyChanges == nil {
		issues = append(issues, "key_changes is nil")
	} else if len(out.KeyChanges) > maxKeyChanges {
		issues = append(issues, fmt.Sprintf("key_changes count = %d, want <= %d", len(out.KeyChanges), maxKeyChanges))
	}

	if strings.TrimSpace(out.PromptVersion) != coachPromptVersion {
		issues = append(issues, fmt.Sprintf("prompt_version = %q, want %q", out.PromptVersion, coachPromptVersion))
	}
	if strings.TrimSpace(out.Model) == "" {
		issues = append(issues, "empty model")
	}

	combined := out.ImprovedAnswer + "\n" + strings.Join(out.KeyChanges, "\n")
	for _, s := range forbidden {
		if strings.Contains(combined, s) {
			issues = append(issues, fmt.Sprintf("output contains forbidden substring %q", s))
		}
	}
	if strings.Contains(combined, "untrusted_") {
		issues = append(issues, `output leaks "untrusted_" data-area tag`)
	}
	return issues
}
