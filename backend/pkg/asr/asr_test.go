package asr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMockASR_Transcribe(t *testing.T) {
	m := NewMockASR()
	tr, err := m.Transcribe(context.Background(), []byte("fake-audio"), "answer.wav", TranscribeOptions{Language: "zh"})
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if tr.Text == "" {
		t.Error("mock transcript should not be empty")
	}
	if m.Name() != "mock" {
		t.Errorf("name = %q", m.Name())
	}
}

func TestMockASR_EmptyAudio(t *testing.T) {
	m := NewMockASR()
	if _, err := m.Transcribe(context.Background(), nil, "answer.wav", TranscribeOptions{}); err == nil {
		t.Error("empty audio should fail")
	}
}

func TestOpenAIASR_Transcribe(t *testing.T) {
	var gotContentType, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"转写文本","language":"zh","duration":12.5}`))
	}))
	defer srv.Close()

	p := NewOpenAIASR(ASRConfig{APIKey: "test-key", BaseURL: srv.URL, Model: "whisper-1"})
	tr, err := p.Transcribe(context.Background(), []byte("fake-audio"), "answer.wav", TranscribeOptions{Language: "zh"})
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if tr.Text != "转写文本" {
		t.Errorf("text = %q", tr.Text)
	}
	if tr.DurationMs != 12500 {
		t.Errorf("duration_ms = %d, want 12500", tr.DurationMs)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotContentType == "" {
		t.Error("multipart content type should be set")
	}
}

func TestOpenAIASR_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid key"))
	}))
	defer srv.Close()

	p := NewOpenAIASR(ASRConfig{APIKey: "bad", BaseURL: srv.URL, Model: "whisper-1"})
	if _, err := p.Transcribe(context.Background(), []byte("fake"), "a.wav", TranscribeOptions{}); err == nil {
		t.Error("api error should fail")
	}
}
