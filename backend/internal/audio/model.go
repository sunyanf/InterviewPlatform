package audio

import (
	"time"

	"ai-interview-platform/internal/agent"
)

// 音频状态
const (
	StatusUploaded    = "uploaded"
	StatusTranscribed = "transcribed"
	StatusAnalyzed    = "analyzed"
)

// Audio 答案语音（一题一份录音，重传覆盖）
type Audio struct {
	ID          string                `json:"id"`
	SessionID   string                `json:"session_id"`
	QuestionID  string                `json:"question_id"`
	UserID      string                `json:"user_id"`
	ObjectKey   string                `json:"object_key"`
	Format      string                `json:"format"`
	SizeBytes   int64                 `json:"size_bytes"`
	DurationMs  int64                 `json:"duration_ms"`
	Transcript  string                `json:"transcript"`
	Language    string                `json:"language"`
	Status      string                `json:"status"`
	Analysis    *agent.SpeechAnalysis `json:"analysis,omitempty"`
	ASRProvider string                `json:"asr_provider"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// SpeechMetrics 语音量化指标（确定性计算，不落库，按请求计算返回）
type SpeechMetrics struct {
	CharsPerMinute float64        `json:"chars_per_minute"`
	Pace           string         `json:"pace"` // slow / normal / fast / unknown
	FillerCount    int            `json:"filler_count"`
	FillerDetail   map[string]int `json:"filler_detail"`
}

// AudioView 语音详情视图（音频 + 指标 + 临时下载 URL）
type AudioView struct {
	Audio          *Audio        `json:"audio"`
	Metrics        SpeechMetrics `json:"metrics"`
	DownloadURL    string        `json:"download_url"`
	DownloadExpire int           `json:"download_expire_seconds"`
}
