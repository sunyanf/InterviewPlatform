package tts

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-interview-platform/internal/config"
)

func TestMockTTS_ValidDeterministicWAV(t *testing.T) {
	p := NewMockTTS()
	if p.Name() != "mock" {
		t.Fatalf("name = %s", p.Name())
	}

	s1, err := p.Synthesize(context.Background(), "你好面试官", SynthesizeOptions{})
	if err != nil {
		t.Fatalf("synth: %v", err)
	}
	s2, _ := p.Synthesize(context.Background(), "不同的文本", SynthesizeOptions{Voice: "nova", Format: "mp3"})

	if s1.Format != "wav" || s1.ContentType != "audio/wav" {
		t.Fatalf("unexpected format: %s/%s", s1.Format, s1.ContentType)
	}
	// WAV 头校验
	if string(s1.Audio[0:4]) != "RIFF" || string(s1.Audio[8:12]) != "WAVE" {
		t.Fatal("missing RIFF/WAVE header")
	}
	if len(s1.Audio) != 44+int(mockSampleRate*mockDurationS)*2 {
		t.Fatalf("wav size = %d", len(s1.Audio))
	}
	// 确定性：任意输入输出字节一致
	if string(s1.Audio) != string(s2.Audio) {
		t.Fatal("mock output must be deterministic regardless of input/options")
	}
}

func TestMockTTS_EmptyText(t *testing.T) {
	p := NewMockTTS()
	if _, err := p.Synthesize(context.Background(), "  ", SynthesizeOptions{}); err == nil {
		t.Fatal("expected error on empty text")
	}
}

func TestOpenAITTS_SendsCorrectRequest(t *testing.T) {
	var gotReq openAISpeechRequest
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotReq); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("FAKE-MP3-AUDIO"))
	}))
	defer srv.Close()

	p := NewOpenAITTS(OpenAIConfig{APIKey: "k", BaseURL: srv.URL, Model: "tts-1", Voice: "alloy", Format: "mp3"})
	s, err := p.Synthesize(context.Background(), "请做个自我介绍", SynthesizeOptions{})
	if err != nil {
		t.Fatalf("synth: %v", err)
	}
	if gotAuth != "Bearer k" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotReq.Model != "tts-1" || gotReq.Input != "请做个自我介绍" || gotReq.Voice != "alloy" || gotReq.ResponseFormat != "mp3" {
		t.Fatalf("request = %+v", gotReq)
	}
	if string(s.Audio) != "FAKE-MP3-AUDIO" || s.Format != "mp3" {
		t.Fatalf("speech = %+v", s)
	}
}

func TestOpenAITTS_OptionsOverride(t *testing.T) {
	var gotReq openAISpeechRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotReq)
		_, _ = w.Write([]byte("OK"))
	}))
	defer srv.Close()

	p := NewOpenAITTS(OpenAIConfig{APIKey: "k", BaseURL: srv.URL, Model: "tts-1", Voice: "alloy", Format: "mp3"})
	_, err := p.Synthesize(context.Background(), "hi", SynthesizeOptions{Voice: "nova", Format: "wav", Speed: 1.25})
	if err != nil {
		t.Fatalf("synth: %v", err)
	}
	if gotReq.Voice != "nova" || gotReq.ResponseFormat != "wav" || gotReq.Speed != 1.25 {
		t.Fatalf("override failed: %+v", gotReq)
	}
}

func TestOpenAITTS_InvalidVoiceAndFormat(t *testing.T) {
	p := NewOpenAITTS(OpenAIConfig{APIKey: "k", BaseURL: "http://x", Model: "tts-1", Voice: "alloy", Format: "mp3"})
	if _, err := p.Synthesize(context.Background(), "hi", SynthesizeOptions{Voice: "robot"}); err == nil ||
		!strings.Contains(err.Error(), "invalid tts voice") {
		t.Fatalf("voice validation failed: %v", err)
	}
	if _, err := p.Synthesize(context.Background(), "hi", SynthesizeOptions{Format: "midi"}); err == nil ||
		!strings.Contains(err.Error(), "invalid tts format") {
		t.Fatalf("format validation failed: %v", err)
	}
	if _, err := p.Synthesize(context.Background(), "", SynthesizeOptions{}); err == nil {
		t.Fatal("empty text should fail")
	}
}

func TestOpenAITTS_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	p := NewOpenAITTS(OpenAIConfig{APIKey: "k", BaseURL: srv.URL, Model: "tts-1", Voice: "alloy", Format: "mp3"})
	_, err := p.Synthesize(context.Background(), "hi", SynthesizeOptions{})
	if err == nil || !strings.Contains(err.Error(), "status=429") {
		t.Fatalf("expected 429 error, got %v", err)
	}
}

func TestFactory(t *testing.T) {
	if p, err := NewTTS(config.TTSConfig{Provider: "mock"}); err != nil || p.Name() != "mock" {
		t.Fatalf("mock factory: %v %v", p, err)
	}
	if p, err := NewTTS(config.TTSConfig{Provider: ""}); err != nil || p.Name() != "mock" {
		t.Fatalf("empty provider should default mock: %v %v", p, err)
	}
	if _, err := NewTTS(config.TTSConfig{Provider: "openai"}); err == nil ||
		!strings.Contains(err.Error(), "TTS_API_KEY is required") {
		t.Fatalf("openai without key must fail: %v", err)
	}
	if _, err := NewTTS(config.TTSConfig{Provider: "azure"}); err == nil ||
		!strings.Contains(err.Error(), "unsupported TTS provider") {
		t.Fatalf("unknown provider must fail: %v", err)
	}
}

func TestFormatContentType(t *testing.T) {
	cases := map[string]string{
		"mp3": "audio/mpeg", "wav": "audio/wav", "opus": "audio/ogg",
		"aac": "audio/aac", "flac": "audio/flac", "pcm": "audio/L16", "xyz": "audio/mpeg",
	}
	for format, want := range cases {
		if got := FormatContentType(format); got != want {
			t.Errorf("%s = %s, want %s", format, got, want)
		}
	}
}

// 编译期保证 Provider 实现
var (
	_ TTS = (*MockTTS)(nil)
	_ TTS = (*OpenAITTS)(nil)
)
