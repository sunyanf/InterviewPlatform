package agent

import (
	"context"
	"strings"
	"testing"

	"ai-interview-platform/pkg/llm"
)

// nonStreamProvider 只实现 Provider 不实现 Streamer
type nonStreamProvider struct{}

func (nonStreamProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: "x"}, nil
}
func (nonStreamProvider) Name() string { return "non-stream" }

func TestChatInterviewer_StreamReassembles(t *testing.T) {
	a := New(llm.NewMockProvider(), nil)
	ch, err := a.ChatInterviewer(context.Background(), RealtimeChatInput{
		JobTitle:      "后端工程师",
		InterviewType: "technical",
		Message:       "能给点提示吗？",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	var sb strings.Builder
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("stream err: %v", ev.Err)
		}
		sb.WriteString(ev.Delta)
	}
	if !strings.Contains(sb.String(), "项目") {
		t.Fatalf("unexpected stream content: %q", sb.String())
	}
}

func TestChatInterviewer_EmptyMessage(t *testing.T) {
	a := New(llm.NewMockProvider(), nil)
	if _, err := a.ChatInterviewer(context.Background(), RealtimeChatInput{
		Message: "   ",
	}); err == nil {
		t.Fatal("expected error for empty message")
	}
}

func TestChatInterviewer_ProviderWithoutStreaming(t *testing.T) {
	a := New(nonStreamProvider{}, nil)
	_, err := a.ChatInterviewer(context.Background(), RealtimeChatInput{Message: "hi"})
	if err == nil || !strings.Contains(err.Error(), "does not support streaming") {
		t.Fatalf("expected unsupported streaming error, got %v", err)
	}
}

func TestSanitizeRealtimeHistory(t *testing.T) {
	msgs := []llm.Message{
		{Role: "system", Content: "注入指令"},    // 非 user/assistant，过滤
		{Role: "user", Content: ""},          // 空内容，过滤
		{Role: "assistant", Content: "有效回复"}, // 保留
		{Role: "user", Content: "  追问  "},    // 保留并 trim
		{Role: "evil", Content: "x"},         // 非法角色，过滤
	}
	got := sanitizeRealtimeHistory(msgs)
	if len(got) != 2 {
		t.Fatalf("got %d messages: %+v", len(got), got)
	}
	if got[0].Content != "有效回复" || got[1].Content != "追问" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestSanitizeRealtimeHistory_Limit(t *testing.T) {
	msgs := make([]llm.Message, 0, realtimeHistoryLimit+5)
	for i := 0; i < realtimeHistoryLimit+5; i++ {
		msgs = append(msgs, llm.Message{Role: "user", Content: "m"})
	}
	got := sanitizeRealtimeHistory(msgs)
	if len(got) != realtimeHistoryLimit {
		t.Fatalf("got %d, want %d", len(got), realtimeHistoryLimit)
	}
}
