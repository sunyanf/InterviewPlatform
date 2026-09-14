package agent

import (
	"context"
	"fmt"
	"strings"

	"ai-interview-platform/pkg/llm"
)

// OpeningMessage 面试官开场白（interview.interviewer.v1）
// 只产出对话内容，不改变会话状态
func (a *Agent) OpeningMessage(ctx context.Context, in OpeningInput) (string, error) {
	prompt := fmt.Sprintf(`你是一个友好的技术面试官。请生成一段面试开场白。

岗位：%s
面试类型：%s
第一道题：%s

要求：
1. 直接返回开场白文本，不要 JSON，不要任何解释
2. 简短自然，包含问候、岗位面试说明，并引出第一道题
3. 100 字以内`, in.JobTitle, in.InterviewType, in.FirstQuestion)

	resp, err := a.llm.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: buildSystemPrompt("你是一个专业的技术面试官。", PromptInterviewer)},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.6,
	})
	if err != nil {
		return "", fmt.Errorf("llm chat: %w", err)
	}

	opening := strings.TrimSpace(resp.Content)
	if opening == "" {
		return "", fmt.Errorf("empty opening message")
	}
	return opening, nil
}
