package llm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ai-interview-platform/pkg/metrics"
)

type fakeProvider struct {
	resp *ChatResponse
	err  error
}

func (f *fakeProvider) Chat(_ context.Context, _ ChatRequest) (*ChatResponse, error) {
	return f.resp, f.err
}
func (f *fakeProvider) Name() string { return "fake" }

func TestMeteredProvider_RecordsTokensAndCost(t *testing.T) {
	inner := &fakeProvider{resp: &ChatResponse{
		Content: "ok", Model: "test-model", InputTokens: 1000, OutputTokens: 500,
	}}
	m := NewMeteredProvider(inner, 2.0, 8.0) // 每 1K：输入 $2、输出 $8

	resp, err := m.Chat(context.Background(), ChatRequest{Model: "request-model"})
	if err != nil || resp.Content != "ok" {
		t.Fatalf("chat failed: %v %v", resp, err)
	}

	out := metrics.Default.Expose()
	for _, want := range []string{
		`llm_requests_total{provider="fake",model="request-model",status="success"}`,
		`llm_tokens_total{provider="fake",model="test-model",direction="input"} 1000`,
		`llm_tokens_total{provider="fake",model="test-model",direction="output"} 500`,
		`llm_cost_total{provider="fake",model="test-model"} 6`, // 1000*0.002 + 500*0.008
		`llm_duration_seconds_bucket{provider="fake",model="request-model",le="+Inf"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics missing %s in:\n%s", want, out)
		}
	}
}

func TestMeteredProvider_ErrorStatus(t *testing.T) {
	boom := errors.New("boom")
	m := NewMeteredProvider(&fakeProvider{err: boom}, 0, 0)

	if _, err := m.Chat(context.Background(), ChatRequest{Model: "m"}); !errors.Is(err, boom) {
		t.Fatalf("want wrapped error to propagate, got %v", err)
	}
	out := metrics.Default.Expose()
	if !strings.Contains(out, `llm_requests_total{provider="fake",model="m",status="error"}`) {
		t.Errorf("error status not recorded:\n%s", out)
	}
	if strings.Contains(out, `llm_tokens_total{provider="fake",model="m"`) {
		t.Errorf("tokens must not be recorded on error:\n%s", out)
	}
}

func TestMeteredProvider_StreamPassthrough(t *testing.T) {
	// 不实现 Streamer 的 Provider → 明确报错
	m := NewMeteredProvider(&fakeProvider{}, 0, 0)
	if _, err := m.StreamChat(context.Background(), ChatRequest{}); !errors.Is(err, ErrStreamingNotSupported) {
		t.Fatalf("want ErrStreamingNotSupported, got %v", err)
	}
}
