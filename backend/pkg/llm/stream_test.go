package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// collectStream 收集流式 channel 的全部内容或错误
func collectStream(ch <-chan StreamChunk) (string, error) {
	var sb strings.Builder
	for c := range ch {
		if c.Err != nil {
			return sb.String(), c.Err
		}
		sb.WriteString(c.Content)
	}
	return sb.String(), nil
}

func TestMockStreamChat_ReassemblesFullContent(t *testing.T) {
	p := NewMockProvider()
	// 默认分支返回确定性的 mock 文本
	ch, err := p.StreamChat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "任意问题"}},
	})
	if err != nil {
		t.Fatalf("stream start: %v", err)
	}
	got, err := collectStream(ch)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "任意问题"}},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if got != resp.Content {
		t.Fatalf("stream content mismatch:\n got=%q\nwant=%q", got, resp.Content)
	}
	if !strings.Contains(got, "mock response") {
		t.Fatalf("unexpected mock content: %q", got)
	}
}

func TestMockStreamChat_ContextCancel(t *testing.T) {
	p := NewMockProvider()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	ch, err := p.StreamChat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "任意问题"}},
	})
	if err != nil {
		return // 建立前取消也是合法行为
	}
	// channel 必须关闭且最终退出，不能泄漏 goroutine
	deadline := time.After(2 * time.Second)
	for range ch {
		select {
		case <-deadline:
			t.Fatal("stream did not stop after context cancel")
		default:
		}
	}
}

func TestOpenAIStreamChat_ParsesSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), "Bearer") {
			t.Errorf("missing auth header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		for _, delta := range []string{"你好", "，面试官", "在此"} {
			fmt.Fprintf(w, `data: {"choices":[{"delta":{"content":%q}}]}`+"\n\n", delta)
			fl.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(OpenAIConfig{APIKey: "k", BaseURL: srv.URL})
	ch, err := p.StreamChat(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("stream start: %v", err)
	}
	got, err := collectStream(ch)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got != "你好，面试官在此" {
		t.Fatalf("got %q", got)
	}
}

func TestOpenAIStreamChat_4xxFailsImmediately(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(OpenAIConfig{APIKey: "bad", BaseURL: srv.URL})
	if _, err := p.StreamChat(context.Background(), ChatRequest{}); err == nil {
		t.Fatal("expected error on 401")
	}
	if calls != 1 {
		t.Fatalf("4xx must not be retried, calls=%d", calls)
	}
}

func TestOpenAIStreamChat_RetriesOn429(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(OpenAIConfig{APIKey: "k", BaseURL: srv.URL})
	ch, err := p.StreamChat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("stream start after retry: %v", err)
	}
	got, err := collectStream(ch)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got != "ok" || calls != 2 {
		t.Fatalf("got=%q calls=%d", got, calls)
	}
}

// 编译期保证两个 Provider 均实现 Streamer 可选接口
var (
	_ Streamer = (*MockProvider)(nil)
	_ Streamer = (*OpenAIProvider)(nil)
)
