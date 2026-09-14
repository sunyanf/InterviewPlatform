package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer 返回指向 httptest 服务器的 Provider
func newTestServer(t *testing.T, handler http.HandlerFunc) (*OpenAIProvider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	p := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   "test-model",
	})
	return p, srv
}

func okBody() string {
	return `{"choices":[{"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`
}

func TestChat_RetryOnServerError(t *testing.T) {
	calls := 0
	p, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server busy"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(okBody()))
	})

	resp, err := p.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat error after retries: %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("content = %q", resp.Content)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (initial + 2 retries)", calls)
	}
}

func TestChat_RetryOnRateLimit(t *testing.T) {
	calls := 0
	p, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(okBody()))
	})

	if _, err := p.Chat(context.Background(), ChatRequest{}); err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestChat_NoRetryOnBadRequest(t *testing.T) {
	calls := 0
	p, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid request"))
	})

	if _, err := p.Chat(context.Background(), ChatRequest{}); err == nil {
		t.Error("400 should fail")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 4xx)", calls)
	}
}

func TestChat_RetriesExhausted(t *testing.T) {
	calls := 0
	p, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := p.Chat(context.Background(), ChatRequest{}); err == nil {
		t.Error("persistent 500 should fail")
	}
	if calls != 1+maxRetries {
		t.Errorf("calls = %d, want %d", calls, 1+maxRetries)
	}
}

func TestChat_EmptyChoices(t *testing.T) {
	p, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	})

	if _, err := p.Chat(context.Background(), ChatRequest{}); err == nil {
		t.Error("empty choices should fail")
	}
}
