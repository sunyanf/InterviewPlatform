package agent

import (
	"context"
	"fmt"
	"strings"

	"ai-interview-platform/pkg/llm"
)

// realtimeHistoryLimit 实时对话携带的最大历史消息条数（防止上下文滥用）
const realtimeHistoryLimit = 20

// RealtimeChatInput 实时面试官流式对话输入
type RealtimeChatInput struct {
	JobTitle      string        // 岗位标题（会话上下文）
	InterviewType string        // 面试类型（technical / behavioral / mixed）
	History       []llm.Message // 本连接内已发生的对话（user / assistant）
	Message       string        // 本轮候选人发言
}

// RealtimeChatEvent 实时对话流式事件；channel 关闭表示结束，Err 非空为终端错误
type RealtimeChatEvent struct {
	Delta string
	Err   error
}

// ChatInterviewer 以面试官身份进行实时流式对话（realtime.interviewer.v1）。
// 只产出对话内容：不改变任何会话状态、不持久化（AGENTS.md #10）。
// Provider 不支持流式能力时返回错误，由调用方决定降级策略。
func (a *Agent) ChatInterviewer(ctx context.Context, in RealtimeChatInput) (<-chan RealtimeChatEvent, error) {
	if strings.TrimSpace(in.Message) == "" {
		return nil, fmt.Errorf("empty chat message")
	}

	streamer, ok := a.llm.(llm.Streamer)
	if !ok {
		return nil, fmt.Errorf("llm provider %s does not support streaming", a.llm.Name())
	}

	system := buildSystemPrompt(fmt.Sprintf(
		"你是一个正在进行%s岗位面试（类型：%s）的专业技术面试官。"+
			"用简短自然的口语与候选人实时交流，可就候选人的回答即时追问或给予提示；"+
			"每次回复控制在 150 字以内，直接输出对话内容，不要 JSON，不要复述候选人的话。",
		in.JobTitle, in.InterviewType,
	), PromptRealtimeInterviewer)

	msgs := make([]llm.Message, 0, len(in.History)+2)
	msgs = append(msgs, llm.Message{Role: "system", Content: system})
	msgs = append(msgs, sanitizeRealtimeHistory(in.History)...)
	msgs = append(msgs, llm.Message{Role: "user", Content: strings.TrimSpace(in.Message)})

	// 上游无 deadline 时施加默认超时，与 agent.chat 保持一致的同步链路保护
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, chatTimeout)
	}

	chunks, err := streamer.StreamChat(ctx, llm.ChatRequest{
		Messages:    msgs,
		Temperature: 0.6,
	})
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("llm stream chat: %w", err)
	}

	out := make(chan RealtimeChatEvent)
	go func() {
		if cancel != nil {
			defer cancel()
		}
		defer close(out)
		for c := range chunks {
			if c.Err != nil {
				sendRealtimeEvent(ctx, out, RealtimeChatEvent{Err: c.Err})
				return
			}
			if !sendRealtimeEvent(ctx, out, RealtimeChatEvent{Delta: c.Content}) {
				return
			}
		}
	}()
	return out, nil
}

// sanitizeRealtimeHistory 清洗客户端传入的对话历史：
// 仅保留 user/assistant 角色的非空内容，截断到最近 realtimeHistoryLimit 条
func sanitizeRealtimeHistory(history []llm.Message) []llm.Message {
	cleaned := make([]llm.Message, 0, len(history))
	for _, m := range history {
		role := strings.TrimSpace(m.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		cleaned = append(cleaned, llm.Message{Role: role, Content: content})
	}
	if len(cleaned) > realtimeHistoryLimit {
		cleaned = cleaned[len(cleaned)-realtimeHistoryLimit:]
	}
	return cleaned
}

// sendRealtimeEvent 发送事件并尊重 ctx 取消；返回 false 表示应终止
func sendRealtimeEvent(ctx context.Context, out chan<- RealtimeChatEvent, ev RealtimeChatEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case out <- ev:
		return true
	}
}
