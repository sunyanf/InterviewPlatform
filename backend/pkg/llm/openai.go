package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIConfig OpenAI 兼容 Provider 配置
type OpenAIConfig struct {
	APIKey  string
	BaseURL string // e.g. https://api.openai.com/v1
	Model   string
}

// OpenAIProvider OpenAI 兼容 Provider 实现
// 适用于 OpenAI、通义千问、DeepSeek 等兼容 OpenAI API 的服务
type OpenAIProvider struct {
	cfg    OpenAIConfig
	client *http.Client
}

// 重试策略：仅对网络错误与 429/5xx 重试（瞬时故障），指数退避
const (
	maxRetries     = 2                      // 额外重试次数
	retryBaseDelay = 200 * time.Millisecond // 首次退避时长
)

// NewOpenAIProvider 创建 OpenAI Provider
func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider {
	return &OpenAIProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Name 返回 Provider 名称
func (p *OpenAIProvider) Name() string {
	return "openai-compatible"
}

// openAIRequest OpenAI API 请求体
type openAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// openAIResponse OpenAI API 响应
type openAIResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Chat 调用 OpenAI 兼容 API（对网络错误与 429/5xx 有限重试）
func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避，尊重 ctx 取消
			delay := retryBaseDelay << (attempt - 1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, status, err := p.doChat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		// status==0 为网络层错误（可重试）；429/5xx 为瞬时故障（可重试）
		if status != 0 && status != http.StatusTooManyRequests && status < 500 {
			return nil, err
		}
	}
	return nil, lastErr
}

// doChat 执行单次 HTTP 调用；status=0 表示网络层错误
func (p *OpenAIProvider) doChat(ctx context.Context, req ChatRequest) (*ChatResponse, int, error) {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}

	body := openAIRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal request: %w", err)
	}

	url := p.cfg.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("call llm api: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("llm api error status=%d body=%s", resp.StatusCode, string(respBytes))
	}

	var oaiResp openAIResponse
	if err := json.Unmarshal(respBytes, &oaiResp); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, resp.StatusCode, fmt.Errorf("llm returned no choices")
	}

	return &ChatResponse{
		Content:      oaiResp.Choices[0].Message.Content,
		Model:        model,
		InputTokens:  oaiResp.Usage.PromptTokens,
		OutputTokens: oaiResp.Usage.CompletionTokens,
	}, resp.StatusCode, nil
}
