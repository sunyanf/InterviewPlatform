package agent

import "fmt"

// Prompt 版本注册表（见 docs/agent/PROMPT_REGISTRY.md）
// 命名规则：领域.能力.版本
// Prompt 变更必须：更新版本号 → 记录原因 → 必要时重跑 Eval Dataset
const (
	// PromptQuestionPlanner 出题规划
	// 用途：根据岗位、简历、面试类型生成结构化面试题
	// 输入：岗位标题/技能/任职要求、候选人技能、面试类型、题数
	// 输出 Schema：{"questions":[{"question","type","difficulty","expected_points"}]}
	// 系统约束：type/difficulty 枚举校验，业务代码归一化，不决定任何会话状态
	PromptQuestionPlanner = "interview.question_planner.v1"

	// PromptInterviewer 面试官开场
	// 用途：生成面试开场白
	// 输入：岗位标题、面试类型、第一道题
	// 输出：自然语言文本（非 JSON）
	// 系统约束：只产出对话内容，不改变会话状态
	PromptInterviewer = "interview.interviewer.v1"

	// PromptFollowUp 追问决策
	// 用途：根据问题、参考要点、回答内容判断是否追问
	// 输入：问题、expected_points、回答文本、回答分析结果
	// 输出 Schema：{"should_follow_up","reason","question","target_gap"}
	// 系统约束：只提出建议，是否创建追问问题由业务代码决定
	PromptFollowUp = "interview.follow_up.v1"

	// PromptAnswerAnalyzer 回答分析
	// 用途：基于证据分析回答内容
	// 输入：问题、expected_points、回答文本
	// 输出 Schema：{"claims","correct_points","wrong_points","missing_points","knowledge_gaps"}
	// 系统约束：分析必须基于回答原文与参考要点（Evidence 原则），不做最终评分
	PromptAnswerAnalyzer = "interview.answer_analyzer.v1"
)

// buildSystemPrompt 构造带版本号的 system prompt
func buildSystemPrompt(role, promptVersion string) string {
	return fmt.Sprintf("%s [prompt_version: %s]", role, promptVersion)
}
