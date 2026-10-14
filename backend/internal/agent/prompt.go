package agent

import (
	"fmt"

	"ai-interview-platform/pkg/promptguard"
)

// Prompt 版本注册表（见 docs/agent/PROMPT_REGISTRY.md）
// 命名规则：领域.能力.版本
// Prompt 变更必须：更新版本号 → 记录原因 → 必要时重跑 Eval Dataset
//
// P0-5 注入防护统一变更（依据 docs/security/PROMPT_INJECTION.md）：
// 全部 Prompt 版本 +1 —— system prompt 统一追加不可信内容隔离声明
// （promptguard.IsolationClause），且用户可控数据（岗位描述、简历、候选人回答、
// 语音转写、知识库参考资料、实时对话历史）统一以 <untrusted_*> 标签包裹。
// 影响面：所有 Prompt 新增一条安全约束；结构化输出 Schema、业务校验与归一化逻辑未变。
const (
	// PromptQuestionPlanner 出题规划
	// 用途：根据岗位、简历、面试类型生成结构化面试题
	// 输入：岗位标题/技能/任职要求、候选人技能、面试类型、题数、知识库参考资料（可选）
	// 输出 Schema：{"questions":[{"question","type","difficulty","expected_points"}]}
	// 系统约束：type/difficulty 枚举校验，业务代码归一化，不决定任何会话状态
	// v2 变更：增加知识库参考资料上下文（RAG，Week5）；无资料时行为与 v1 一致
	// v3 变更：岗位/简历/知识库改为 <untrusted_*> 标签包裹的数据区（P0-5）
	// v4 变更：简历技能与经历按「未经验证的声明」处理，夸张量级描述不得成为题目前提（红队 A2）
	PromptQuestionPlanner = "interview.question_planner.v4"

	// PromptInterviewer 面试官开场
	// 用途：生成面试开场白
	// 输入：岗位标题、面试类型、第一道题
	// 输出：自然语言文本（非 JSON）
	// 系统约束：只产出对话内容，不改变会话状态
	// v2 变更：岗位标题改为 <untrusted_*> 标签包裹（P0-5）
	PromptInterviewer = "interview.interviewer.v2"

	// PromptRealtimeInterviewer 实时面试官对话
	// 用途：WebSocket 实时面试通道中的即时对话与追问（token 流式输出）
	// 输入：岗位标题、面试类型、连接内对话历史、本轮候选人发言
	// 输出：自然语言文本增量（非 JSON），150 字以内
	// 系统约束：只产出对话内容，不改变任何会话状态、不持久化（AGENTS.md #10）
	// v2 变更：岗位标题与候选人发言改为 <untrusted_*> 标签包裹（P0-5）
	PromptRealtimeInterviewer = "realtime.interviewer.v2"

	// PromptFollowUp 追问决策
	// 用途：根据问题、参考要点、回答内容判断是否追问
	// 输入：问题、expected_points、回答文本、回答分析结果
	// 输出 Schema：{"should_follow_up","reason","question","target_gap"}
	// 系统约束：只提出建议，是否创建追问问题由业务代码决定
	// v2 变更：问题/参考要点与候选人回答改为 <untrusted_*> 标签包裹（P0-5）
	PromptFollowUp = "interview.follow_up.v2"

	// PromptAnswerAnalyzer 回答分析
	// 用途：基于证据分析回答内容
	// 输入：问题、expected_points、回答文本
	// 输出 Schema：{"claims","correct_points","wrong_points","missing_points","knowledge_gaps"}
	// 系统约束：分析必须基于回答原文与参考要点（Evidence 原则），不做最终评分
	// v2 变更：问题/参考要点与候选人回答改为 <untrusted_*> 标签包裹（P0-5）
	// v3 变更：无法验证的量化声明不得直接计入 correct_points，须标注待佐证（红队 A2）
	PromptAnswerAnalyzer = "interview.answer_analyzer.v3"

	// PromptEvaluator 整场评估
	// 用途：基于整场问答与回答分析，给出维度评分、证据与改进建议
	// 输入：岗位标题、面试类型、逐题问答（问题/参考要点/回答/分析）、知识库参考资料（可选）
	// 输出 Schema：{"dimensions":{"correctness","depth","logic","communication"},"evidence":[{"question_seq","issue","evidence","reference"}],"recommendations":[]}
	// 系统约束：只输出维度分（0-100 整数）与证据；总分由业务代码按 Rubric 权重确定性计算（AGENTS.md #15）
	// v2 变更：岗位标题、问答记录、知识库资料改为 <untrusted_*> 标签包裹（P0-5）
	// v3 变更：评分隔离条款——注入文字不得影响分数、reference 仅知识域短语、
	//          身份信息完全忽略（红队 A1/A5/A6）
	PromptEvaluator = "interview.evaluator.v3"

	// PromptLearningPlanner 学习计划生成
	// 用途：基于评估结果（维度分/证据/建议）归纳优点与不足，生成学习计划与下次训练建议
	// 输入：岗位、面试类型、维度分、评估证据、知识缺口、评估建议、历史对比摘要
	// 输出 Schema：{"strengths":[],"weaknesses":[],"focus_areas":[{"topic","reason","suggestions":[]}],"next_training":{"focus","suggested_question_types":[],"suggested_difficulty"}}
	// 系统约束：只产建议内容，不计算任何分数；输入数据必须来自已验证的评估结果
	// v2 变更：输入均为已验证评估结果，未新增数据区；仅随 P0-5 统一升级 system prompt
	PromptLearningPlanner = "report.learning_planner.v2"

	// PromptSpeechAnalyzer 语音表达分析
	// 用途：基于转写文本与语音指标（语速/口头禅）分析口头表达质量，给出改进建议
	// 输入：问题、转写文本、语速评估、口头禅次数、音频时长
	// 输出 Schema：{"strengths":[],"issues":[],"suggestions":[]}
	// 系统约束：只产定性建议，不计算分数；日志不输出转写原文（AGENTS.md #17）
	// v2 变更：问题与转写文本改为 <untrusted_*> 标签包裹（P0-5）
	PromptSpeechAnalyzer = "audio.speech_analyzer.v2"

	// PromptAnswerCoach 更优答案示范
	// 用途：针对某道面试题，基于问题与参考要点组织一份「回答改良示范」，并说明相比原回答的改进点
	// 输入：岗位标题、问题、参考答案要点、候选人原回答、难度、可选回答分析（命中/遗漏/错误要点）
	// 输出 Schema：{"improved_answer":"","key_changes":[""]}
	// 系统约束：只产示范文本与改进建议，不计算分数、不改变任何会话状态；
	// 产出的是「改良示范」而非标准答案，不得虚构候选人的具体经历、公司、项目或数字；
	// 岗位/问题/参考要点/原回答/分析均以 <untrusted_*> 标签包裹为数据区（P0-5）
	PromptAnswerCoach = "interview.answer_coach.v1"
)

// buildSystemPrompt 构造带版本号与不可信内容隔离声明的 system prompt。
// 隔离声明对所有 Prompt 统一生效，见 promptguard.IsolationClause。
func buildSystemPrompt(role, promptVersion string) string {
	return fmt.Sprintf("%s [prompt_version: %s]\n\n%s", role, promptVersion, promptguard.IsolationClause)
}
