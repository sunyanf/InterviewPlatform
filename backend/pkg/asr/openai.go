package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// ASRConfig OpenAI 兼容 ASR 配置
type ASRConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// OpenAIASR OpenAI 兼容语音识别 Provider（whisper-1 / gpt-4o-transcribe 等 /v1/audio/transcriptions 接口）
type OpenAIASR struct {
	cfg    ASRConfig
	client *http.Client
}

// NewOpenAIASR 创建 OpenAI 兼容 ASR
func NewOpenAIASR(cfg ASRConfig) *OpenAIASR {
	return &OpenAIASR{
		cfg: cfg,
		client: &http.Client{
			Timeout: 120 * time.Second, // 语音转写耗时较长
		},
	}
}

// Name 返回 Provider 名称
func (p *OpenAIASR) Name() string {
	return "openai"
}

// openAITranscriptionResponse /v1/audio/transcriptions 响应（json 格式）
type openAITranscriptionResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
}

// Transcribe 调用 OpenAI 兼容转写接口
func (p *OpenAIASR) Transcribe(ctx context.Context, audio []byte, filename string, opts TranscribeOptions) (*Transcript, error) {
	if len(audio) == 0 {
		return nil, fmt.Errorf("empty audio data")
	}

	// multipart 构造
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	fw, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := fw.Write(audio); err != nil {
		return nil, fmt.Errorf("write audio: %w", err)
	}

	_ = writer.WriteField("model", p.cfg.Model)
	// response_format=json 为默认值，显式声明避免 Provider 差异
	_ = writer.WriteField("response_format", "json")
	if opts.Language != "" {
		_ = writer.WriteField("language", opts.Language)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	url := p.cfg.BaseURL + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call asr api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("asr api error status=%d body=%s", resp.StatusCode, string(body))
	}

	var tr openAITranscriptionResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &Transcript{
		Text:       tr.Text,
		Language:   tr.Language,
		DurationMs: int64(tr.Duration * 1000),
	}, nil
}
