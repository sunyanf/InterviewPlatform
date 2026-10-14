package agent

import (
	"context"
	"fmt"
	"strings"

	"ai-interview-platform/pkg/llm"
	"ai-interview-platform/pkg/promptguard"
)

// AnswerCoachInput 更优答案示范输入
type AnswerCoachInput struct {
	JobTitle       string
	Question       string
	ExpectedPoints []string
	AnswerText     string
	Difficulty     string
	// Analysis 可选：已生成的回答分析（命中/遗漏/错误要点），用于指导改进
	Analysis *AnswerAnalysis
}

// AnswerCoaching 更优答案示范结果（AI 数据 Contract）
type AnswerCoaching struct {
	ImprovedAnswer string   `json:"improved_answer"`
	KeyChanges     []string `json:"key_changes"`
	PromptVersion  string   `json:"prompt_version"`
	Model          string   `json:"model"`
}

// CoachAnswer 更优答案示范（interview.answer_coach.v1）
// 基于问题、参考要点与候选人原回答组织一份「回答改良示范」；只产文本，
// 不计算分数、不改变任何会话状态（AGENTS.md #9、#10）。
func (a *Agent) CoachAnswer(ctx context.Context, in AnswerCoachInput) (*AnswerCoaching, error) {
	// 原回答分析摘要（可选，作为数据区注入）
	analysisSection := ""
	if s := formatAnswerAnalysis(in.Analysis); s != "" {
		analysisSection = "\n原回答分析（供参考）：\n" + promptguard.Wrap("answer_analysis", s)
	}

	// 岗位/问题/参考要点/候选人回答/分析均属不可信输入：以数据区包裹（P0-5）
	prompt := fmt.Sprintf(`请针对下面这道面试题，为候选人给出一份「回答改良示范」。

岗位背景：
%s
难度：%s

题目：
%s

参考答案要点：
%s

候选人原回答：
%s
%s

要求：
1. 只返回 JSON，不要包含任何解释文字
2. JSON 结构：{"improved_answer":"","key_changes":[""]}
3. improved_answer 是一份「回答改良示范」，不是这道题的标准答案：演示在给定问题与参考要点下如何更有条理、更完整地组织一段口语化回答（建议 150-400 字）
4. 只依据给定的问题与参考要点组织内容，不得虚构候选人的具体经历、公司、项目名称或数字；需要举例时使用通用化、不指向特定个人的表述
5. key_changes 给 1-4 条，每条一句话，说明这份示范相比候选人原回答的主要改进点
6. 全程使用中文`,
		promptguard.Wrap("job_context", in.JobTitle), in.Difficulty,
		promptguard.Wrap("question", in.Question),
		promptguard.Wrap("expected_points", joinPoints(in.ExpectedPoints)),
		promptguard.Wrap("candidate_answer", in.AnswerText),
		analysisSection)

	resp, err := a.llm.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: buildSystemPrompt(
				"你是一名资深面试教练，擅长针对具体面试题给出结构清晰、真实可信的回答改良示范，只输出 JSON。",
				PromptAnswerCoach)},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.5,
	})
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	// Schema 校验
	var out AnswerCoaching
	if err := unmarshal(cleanJSON(resp.Content), &out); err != nil {
		return nil, err
	}

	// 业务校验 + 归一化（AGENTS.md #11）
	out.ImprovedAnswer = strings.TrimSpace(out.ImprovedAnswer)
	if out.ImprovedAnswer == "" {
		return nil, fmt.Errorf("answer coach output missing improved_answer")
	}
	// key_changes：trim、去空、去重；允许为空数组但不得为 nil
	out.KeyChanges = dedupeStrings(cleanStringList(out.KeyChanges))

	// 记录元信息（可追踪：prompt 版本 + 模型）
	out.PromptVersion = PromptAnswerCoach
	out.Model = resp.Model

	return &out, nil
}

// formatAnswerAnalysis 把回答分析要点格式化为文本，无有效内容时返回空串
func formatAnswerAnalysis(a *AnswerAnalysis) string {
	if a == nil {
		return ""
	}
	var sb strings.Builder
	writeSection := func(label string, items []string) {
		if len(items) == 0 {
			return
		}
		sb.WriteString(label + "：\n")
		for _, it := range items {
			sb.WriteString("- " + it + "\n")
		}
	}
	writeSection("关键论断", a.Claims)
	writeSection("命中要点", a.CorrectPoints)
	writeSection("错误要点", a.WrongPoints)
	writeSection("遗漏要点", a.MissingPoints)
	writeSection("知识缺口", a.KnowledgeGaps)
	return strings.TrimSpace(sb.String())
}

// dedupeStrings 去重（保序），始终返回非 nil 切片
func dedupeStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, s := range items {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
