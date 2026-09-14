package agent

import (
	"context"
	"fmt"
	"strings"
)

// PlanQuestions 出题规划（interview.question_planner.v1）
// LLM 只生成题目内容，输出经 Schema 校验 + 业务校验归一化
func (a *Agent) PlanQuestions(ctx context.Context, in PlanQuestionsInput) ([]PlannedQuestion, error) {
	if in.Count < 1 || in.Count > 20 {
		return nil, fmt.Errorf("invalid question count: %d", in.Count)
	}

	// 知识库参考资料（可选，由业务层 RAG 检索后传入）
	knowledgeSection := ""
	if len(in.Knowledge) > 0 {
		var refs strings.Builder
		for i, k := range in.Knowledge {
			refs.WriteString(fmt.Sprintf("[%d] %s\n", i+1, k))
		}
		knowledgeSection = fmt.Sprintf(`
知识库参考资料（出题依据，必须基于以下资料出题，不得编造资料中不存在的知识点）：
%s
`, refs.String())
	}

	prompt := fmt.Sprintf(`你是一个技术面试官。请根据以下信息生成 %d 道面试题。

岗位：%s
岗位技能要求：%s
岗位任职要求：%s
候选人技能：%s
面试类型：%s
%s
要求：
1. 只返回 JSON，不要包含任何解释文字
2. JSON 结构：{"questions":[{"question":"","type":"","difficulty":"","expected_points":[""]}]}
3. type 只能是 technical、behavioral、project 之一
4. difficulty 只能是 easy、medium、hard 之一
5. expected_points 是该题的参考答案要点，2-5 条
6. 问题应循序渐进，覆盖岗位核心技能`,
		in.Count, in.JobTitle, strings.Join(in.JobSkills, "、"),
		strings.Join(in.JobRequirements, "；"), strings.Join(in.ResumeSkills, "、"), in.InterviewType,
		knowledgeSection)

	content, err := a.chat(ctx,
		buildSystemPrompt("你是一个专业的技术面试官，只输出 JSON。", PromptQuestionPlanner),
		prompt, 0.7)
	if err != nil {
		return nil, err
	}

	// Schema 校验
	var parsed struct {
		Questions []PlannedQuestion `json:"questions"`
	}
	if err := unmarshal(content, &parsed); err != nil {
		return nil, err
	}

	// 业务校验 + 归一化
	var questions []PlannedQuestion
	for _, q := range parsed.Questions {
		q.Question = strings.TrimSpace(q.Question)
		if q.Question == "" {
			continue
		}
		if !validQuestionTypes[q.Type] || q.Type == "follow_up" {
			q.Type = "technical"
		}
		if !validDifficulties[q.Difficulty] {
			q.Difficulty = "medium"
		}
		if q.ExpectedPoints == nil {
			q.ExpectedPoints = []string{}
		}
		questions = append(questions, q)
	}

	return questions, nil
}
