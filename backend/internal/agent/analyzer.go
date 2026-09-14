package agent

import (
	"context"
	"fmt"
	"strings"

	"ai-interview-platform/pkg/llm"
)

// AnalyzeAnswer 回答分析（interview.answer_analyzer.v1）
// 分析基于问题、参考要点与回答原文（Evidence 原则），输出经 Schema 校验
func (a *Agent) AnalyzeAnswer(ctx context.Context, in AnalyzeAnswerInput) (*AnswerAnalysis, error) {
	prompt := fmt.Sprintf(`请分析候选人对下面问题的回答。

问题：%s
参考答案要点：
%s
难度：%s
候选人回答：
%s

要求：
1. 只返回 JSON，不要包含任何解释文字
2. JSON 结构：{"claims":[""],"correct_points":[""],"wrong_points":[""],"missing_points":[""],"knowledge_gaps":[""]}
3. claims：候选人回答中的关键论断
4. correct_points：与参考要点一致的回答点
5. wrong_points：与参考要点矛盾或错误的回答点
6. missing_points：候选人未覆盖的参考要点
7. knowledge_gaps：暴露出的知识缺口
8. 所有分析必须基于回答原文，不要凭空推测`,
		in.Question, joinPoints(in.ExpectedPoints), in.Difficulty, in.AnswerText)

	resp, err := a.llm.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: buildSystemPrompt("你是一个严谨的技术面试分析助手，只输出 JSON。", PromptAnswerAnalyzer)},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	// Schema 校验
	var analysis AnswerAnalysis
	if err := unmarshal(cleanJSON(resp.Content), &analysis); err != nil {
		return nil, err
	}

	// 业务校验 + 归一化
	normalizeStrings(&analysis.Claims)
	normalizeStrings(&analysis.CorrectPoints)
	normalizeStrings(&analysis.WrongPoints)
	normalizeStrings(&analysis.MissingPoints)
	normalizeStrings(&analysis.KnowledgeGaps)

	// 记录元信息（可追踪：prompt 版本 + 模型）
	analysis.PromptVersion = PromptAnswerAnalyzer
	analysis.Model = resp.Model

	return &analysis, nil
}

func joinPoints(points []string) string {
	if len(points) == 0 {
		return "（无）"
	}
	var sb strings.Builder
	for i, p := range points {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, p))
	}
	return sb.String()
}

func normalizeStrings(s *[]string) {
	if s == nil || *s == nil {
		*s = []string{}
	}
}
