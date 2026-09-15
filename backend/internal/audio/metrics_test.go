package audio

import (
	"strings"
	"testing"
)

func TestDetectFormat(t *testing.T) {
	ok := map[string]string{
		"answer.wav":       "wav",
		"answer.MP3":       "mp3",
		"/path/to/rec.m4a": "m4a",
		"rec.webm":         "webm",
		"rec.ogg":          "ogg",
	}
	for name, want := range ok {
		got, err := DetectFormat(name)
		if err != nil || got != want {
			t.Errorf("DetectFormat(%q) = %q, %v; want %q", name, got, err, want)
		}
	}

	bad := []string{"noext", "answer.exe", "answer.txt", "answer.", "answer.avi"}
	for _, name := range bad {
		if _, err := DetectFormat(name); err == nil {
			t.Errorf("DetectFormat(%q) should fail", name)
		}
	}
}

func TestContentType(t *testing.T) {
	if got := ContentType("mp3"); got != "audio/mpeg" {
		t.Errorf("ContentType(mp3) = %q", got)
	}
	if got := ContentType("xyz"); got != "application/octet-stream" {
		t.Errorf("ContentType(xyz) = %q", got)
	}
}

func TestComputeSpeechMetrics_Normal(t *testing.T) {
	// 60 秒 240 个有效字符 → 240 字/分钟 normal
	transcript := strings.Repeat("功能", 120) // 240 个汉字
	m := ComputeSpeechMetrics(transcript, 60000)
	if m.CharsPerMinute != 240 {
		t.Errorf("cpm = %v, want 240", m.CharsPerMinute)
	}
	if m.Pace != "normal" {
		t.Errorf("pace = %q, want normal", m.Pace)
	}
}

func TestComputeSpeechMetrics_Slow(t *testing.T) {
	transcript := strings.Repeat("字", 100) // 100 字 / 1 分钟
	m := ComputeSpeechMetrics(transcript, 60000)
	if m.Pace != "slow" {
		t.Errorf("pace = %q, want slow", m.Pace)
	}
}

func TestComputeSpeechMetrics_Fast(t *testing.T) {
	transcript := strings.Repeat("字", 300)
	m := ComputeSpeechMetrics(transcript, 60000)
	if m.Pace != "fast" {
		t.Errorf("pace = %q, want fast", m.Pace)
	}
}

func TestComputeSpeechMetrics_Unknown(t *testing.T) {
	// 无时长 → unknown
	m := ComputeSpeechMetrics("这是一段足够长的转写文本内容", 0)
	if m.Pace != "unknown" || m.CharsPerMinute != 0 {
		t.Errorf("metrics = %+v, want unknown/0", m)
	}
	// 字符过少 → unknown
	m = ComputeSpeechMetrics("好的", 60000)
	if m.Pace != "unknown" {
		t.Errorf("pace = %q, want unknown (too few chars)", m.Pace)
	}
}

func TestComputeSpeechMetrics_Fillers(t *testing.T) {
	// 口头禅计数（大小写不敏感）
	transcript := "嗯，我觉得那个 goroutine 很好，那个 channel 也不错，呃，UM 也常用。"
	m := ComputeSpeechMetrics(transcript, 60000)
	if m.FillerDetail["嗯"] != 1 {
		t.Errorf("嗯 = %d, want 1", m.FillerDetail["嗯"])
	}
	if m.FillerDetail["那个"] != 2 {
		t.Errorf("那个 = %d, want 2", m.FillerDetail["那个"])
	}
	if m.FillerDetail["呃"] != 1 {
		t.Errorf("呃 = %d, want 1", m.FillerDetail["呃"])
	}
	if m.FillerDetail["um"] != 1 {
		t.Errorf("um = %d, want 1 (case-insensitive)", m.FillerDetail["um"])
	}
	if m.FillerCount != 5 {
		t.Errorf("filler_count = %d, want 5", m.FillerCount)
	}
}

func TestComputeSpeechMetrics_PunctuationExcluded(t *testing.T) {
	// 标点与空白不计入有效字符
	m := ComputeSpeechMetrics("你好，世界！ Go 语言。", 60000)
	// 有效字符：你好世界Go语言 = 4 + 2 + 2 = 8 < 10 → unknown
	if m.Pace != "unknown" {
		t.Errorf("pace = %q, want unknown (8 effective chars)", m.Pace)
	}
}
