package agent

import (
	"context"
	"log/slog"
	"testing"
)

func TestAnalyzeSpeech_HappyPath(t *testing.T) {
	stub := &stubProvider{content: `{
		"strengths": ["概念表述清晰"],
		"issues": ["口头禅较多"],
		"suggestions": ["先结论后展开", "用停顿替代口头禅"]
	}`}
	a := &Agent{llm: stub, log: slog.Default()}

	out, err := a.AnalyzeSpeech(context.Background(), "Q1", "转写文本内容", SpeechMetricsInput{
		DurationSec:    60,
		CharsPerMinute: 240,
		Pace:           "normal",
		FillerCount:    5,
		FillerDetail:   map[string]int{"嗯": 3, "那个": 2},
	})
	if err != nil {
		t.Fatalf("AnalyzeSpeech error: %v", err)
	}
	if len(out.Strengths) != 1 || len(out.Issues) != 1 || len(out.Suggestions) != 2 {
		t.Errorf("output = %+v", out)
	}
	if out.PromptVersion != PromptSpeechAnalyzer {
		t.Errorf("prompt_version = %q", out.PromptVersion)
	}
}

func TestAnalyzeSpeech_EmptyTranscript(t *testing.T) {
	a := &Agent{llm: &stubProvider{}, log: slog.Default()}
	if _, err := a.AnalyzeSpeech(context.Background(), "Q1", "", SpeechMetricsInput{}); err == nil {
		t.Error("empty transcript should fail")
	}
}

func TestAnalyzeSpeech_EmptySuggestions(t *testing.T) {
	// 无建议 → 分析无效（可重跑）
	stub := &stubProvider{content: `{"strengths":["s"],"issues":["i"],"suggestions":["","  "]}`}
	a := &Agent{llm: stub, log: slog.Default()}
	if _, err := a.AnalyzeSpeech(context.Background(), "Q1", "转写文本", SpeechMetricsInput{}); err == nil {
		t.Error("empty suggestions should fail validation")
	}
}

func TestAnalyzeSpeech_WrongFieldType(t *testing.T) {
	// suggestions 为字符串 → Schema 校验失败
	stub := &stubProvider{content: `{"strengths":["s"],"issues":["i"],"suggestions":"not-an-array"}`}
	a := &Agent{llm: stub, log: slog.Default()}
	if _, err := a.AnalyzeSpeech(context.Background(), "Q1", "转写文本", SpeechMetricsInput{}); err == nil {
		t.Error("wrong field type should fail schema validation")
	}
}

func TestFormatFillerDetail(t *testing.T) {
	got := formatFillerDetail(map[string]int{"那个": 2, "嗯": 3})
	// 字典序：嗯(unicode) vs 那个 — 按字节序 "嗯" < "那个"（E5 97 AF < E9 82 A3）
	want := ": 嗯×3、那个×2"
	if got != want {
		t.Errorf("format = %q, want %q", got, want)
	}
	if formatFillerDetail(nil) != "" {
		t.Error("nil detail should be empty")
	}
}
