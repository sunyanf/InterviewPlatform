package evalrun

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"ai-interview-platform/internal/agent"
)

// plannerPromptVersion 出题规划 Prompt 版本（interview.question_planner.v4）
const plannerPromptVersion = agent.PromptQuestionPlanner

// PlannerCase 出题防护（planner_guard）数据集样本：
// 简历技能可包含夸张的量级描述，用于验证 planner 不把它当事实前提命题（红队 A2）。
type PlannerCase struct {
	CaseID          string   `json:"case_id"`
	Dataset         string   `json:"dataset"`
	Description     string   `json:"description"`
	JobTitle        string   `json:"job_title"`
	JobSkills       []string `json:"job_skills"`
	JobRequirements []string `json:"job_requirements"`
	// ResumeSkills 简历技能（可包含「主导亿级网关」等未经验证的夸张描述）
	ResumeSkills []string `json:"resume_skills"`
	// InterviewType 面试类型（technical / behavioral / mixed）
	InterviewType string `json:"interview_type"`
	// Count 出题数量
	Count int `json:"count"`
	// ForbiddenQuestionSubstrings 所有题干中不得出现的子串（夸张量级描述本身）
	ForbiddenQuestionSubstrings []string `json:"forbidden_question_substrings"`
}

// LoadPlanner 从 JSONL 文件加载 planner_guard 数据集样本
func LoadPlanner(path string) ([]PlannerCase, error) {
	var cases []PlannerCase
	if err := loadJSONL(path, &cases); err != nil {
		return nil, err
	}
	return cases, nil
}

// RunPlanner 对 planner_guard 数据集跑出题防护契约回归：
// 验证 agent.PlanQuestions 在简历含未经验证的夸张量级描述时，题干不把它当作事实前提，
// 也不在题干中复述这些描述（prompt 版本 + 题干禁用子串检查）。
func (r *Runner) RunPlanner(ctx context.Context, cases []PlannerCase) (*Report, error) {
	if len(cases) == 0 {
		return nil, fmt.Errorf("planner dataset is empty")
	}

	ag := agent.New(r.provider, slog.Default())
	report := &Report{
		Provider:      r.provider.Name(),
		Dataset:       cases[0].Dataset,
		Mode:          "planner_contract",
		PromptVersion: plannerPromptVersion,
		AgreementRate: -1,
		MeanAbsError:  -1,
	}

	for _, c := range cases {
		res := CaseResult{CaseID: c.CaseID, OK: true}
		questions, err := ag.PlanQuestions(ctx, agent.PlanQuestionsInput{
			JobTitle:        c.JobTitle,
			JobSkills:       c.JobSkills,
			JobRequirements: c.JobRequirements,
			ResumeSkills:    c.ResumeSkills,
			InterviewType:   c.InterviewType,
			Count:           c.Count,
		})
		if err != nil {
			res.OK = false
			res.Issues = append(res.Issues, "plan questions error: "+err.Error())
			report.Results = append(report.Results, res)
			continue
		}

		res.QuestionsCount = len(questions)
		for _, issue := range checkPlannerContract(questions, c.ForbiddenQuestionSubstrings) {
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

// checkPlannerContract 出题防护契约复核（runner 级独立检查）：
// 至少产出 1 题，且所有题干不得包含禁用子串。
func checkPlannerContract(questions []agent.PlannedQuestion, forbidden []string) []string {
	var issues []string
	if len(questions) == 0 {
		return []string{"no questions generated"}
	}
	for i, q := range questions {
		for _, s := range forbidden {
			if strings.Contains(q.Question, s) {
				issues = append(issues, fmt.Sprintf("question[%d] contains forbidden substring %q", i, s))
			}
		}
	}
	return issues
}
