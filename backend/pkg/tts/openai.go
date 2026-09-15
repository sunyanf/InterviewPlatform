package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIConfig OpenAI 兼容 TTS Provider 配置
type OpenAIConfig struct {
	APIKey  string
	BaseURL string // e.g. https://api.openai.com/v1
	Model   string // tts-1 / tts-1-hd
	Voice   string // alloy / echo / fable / onyx / nova / shimmer
	Format  string // mp3 / wav / opus / aac / flac
}

// OpenAITTS OpenAI 兼容 /audio/speech 实现
type OpenAITTS struct {
	cfg    OpenAIConfig
	client *http.Client
}

// NewOpenAITTS 创建 OpenAI TTS Provider
func NewOpenAITTS(cfg OpenAIConfig) *OpenAITTS {
	return &OpenAITTS{
		cfg: cfg,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Name 返回 Provider 名称
func (p *OpenAITTS) Name() string { return "openai-compatible" }

// openAISpeechRequest /audio/speech 请求体
type openAISpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

// 合法输出格式与音色白名单（发送前业务校验，避免无效请求消耗配额）
var validFormats = map[string]bool{"mp3": true, "wav": true, "opus": true, "aac": true, "flac": true, "pcm": true}
var validVoices = map[string]bool{
	"alloy": true, "echo": true, "fable": true, "onyx": true, "nova": true, "shimmer": true,
}

// Synthesize 调用 OpenAI 兼容 TTS API
func (p *OpenAITTS) Synthesize(ctx context.Context, text string, opts SynthesizeOptions) (*Speech, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("empty tts text")
	}

	voice := firstNonEmpty(opts.Voice, p.cfg.Voice)
	if !validVoices[voice] {
		return nil, fmt.Errorf("invalid tts voice: %s", voice)
	}
	format := firstNonEmpty(opts.Format, p.cfg.Format)
	if format == "" {
		format = "mp3"
	}
	if !validFormats[format] {
		return nil, fmt.Errorf("invalid tts format: %s", format)
	}

	reqBody, err := json.Marshal(openAISpeechRequest{
		Model:          p.cfg.Model,
		Input:          text,
		Voice:          voice,
		ResponseFormat: format,
		Speed:          opts.Speed,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/audio/speech", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	httpReq.Header.Set("Accept", FormatContentType(format))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call tts api: %w", err)
	}
	defer resp.Body.Close()

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tts api error status=%d body=%s", resp.StatusCode, string(audio))
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("tts api returned empty audio")
	}

	return &Speech{
		Audio:       audio,
		Format:      format,
		ContentType: firstNonEmpty(resp.Header.Get("Content-Type"), FormatContentType(format)),
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
