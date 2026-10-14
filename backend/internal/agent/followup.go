package agent

import (
	"context"
	"fmt"
	"strings"

	"ai-interview-platform/pkg/promptguard"
)

// DecideFollowUp 追问决策（interview.follow_up.v2）
// LLM 只提出建议；是否创建追问问题由业务代码决定（ADR-0004）
func (a *Agent) DecideFollowUp(ctx context.Context, in FollowUpInput) (*FollowUpDecision, error) {
	analysisPart := "（无分析结果）"
	if in.Analysis != nil {
		analysisPart = fmt.Sprintf(
			"正确点：%s\n错误点：%s\n缺失点：%s\n知识缺口：%s",
			strings.Join(in.Analysis.CorrectPoints, "、"),
			strings.Join(in.Analysis.WrongPoints, "、"),
			strings.Join(in.Analysis.MissingPoints, "、"),
			strings.Join(in.Analysis.KnowledgeGaps, "、"),
		)
	}

	// 问题/参考要点由不可信输入衍生，候选人回答为用户原文：均作为数据区注入
	questionSection := promptguard.Wrap("question", fmt.Sprintf(
		"问题：%s\n参考答案要点：\n%s", in.Question, joinPoints(in.ExpectedPoints)))

	prompt := fmt.Sprintf(`你是一个技术面试官。请根据以下信息判断是否需要追问。

%s
%s

回答分析：
%s

要求：
1. 只返回 JSON，不要包含任何解释文字
2. JSON 结构：{"should_follow_up":false,"reason":"","question":"","target_gap":""}
3. 当回答明显浮于表面、存在错误未澄清、或知识缺口值得深挖时，才 should_follow_up 为 true
4. should_follow_up 为 true 时，question 给出具体的追问问题，target_gap 指出追问目标
5. 不需要追问时，question 和 target_gap 填空字符串`,
		questionSection, promptguard.Wrap("candidate_answer", in.AnswerText), analysisPart)

	content, err := a.chat(ctx,
		buildSystemPrompt("你是一个专业的技术面试官，只输出 JSON。", PromptFollowUp),
		prompt, 0.3)
	if err != nil {
		return nil, err
	}

	// Schema 校验
	var decision FollowUpDecision
	if err := unmarshal(content, &decision); err != nil {
		return nil, err
	}

	// 业务校验 + 归一化
	decision.Reason = strings.TrimSpace(decision.Reason)
	decision.Question = strings.TrimSpace(decision.Question)
	decision.TargetGap = strings.TrimSpace(decision.TargetGap)
	// 建议追问但未给出问题时，视为不追问
	if decision.ShouldFollowUp && decision.Question == "" {
		decision.ShouldFollowUp = false
	}

	return &decision, nil
}
