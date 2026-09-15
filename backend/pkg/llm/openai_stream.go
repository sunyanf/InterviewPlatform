package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// openAIStreamChunk OpenAI SSE 流式响应分片
type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// StreamChat 调用 OpenAI 兼容 API 的 SSE 流式接口。
// 连接建立前的错误（网络错误 / 429 / 5xx）按统一策略有限重试；
// 一旦开始读取流（HTTP 200），中途错误只能通过 StreamChunk.Err 传递，不重试（避免重复推送）。
func (p *OpenAIProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay << (attempt - 1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		var status int
		var err error
		resp, status, err = p.openStream(ctx, req)
		if err == nil {
			break
		}
		lastErr = err
		// 流尚未建立：网络错误（status=0）/429/5xx 可重试，4xx 立即失败
		if status != 0 && status != http.StatusTooManyRequests && status < 500 {
			return nil, err
		}
	}
	if resp == nil {
		return nil, lastErr
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		if err := scanSSE(ctx, resp.Body, ch); err != nil {
			sendChunk(ctx, ch, StreamChunk{Err: err})
		}
	}()
	return ch, nil
}

// openStream 建立 SSE 连接；status=0 表示网络层错误
func (p *OpenAIProvider) openStream(ctx context.Context, req ChatRequest) (*http.Response, int, error) {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}

	bodyBytes, err := json.Marshal(openAIRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.streamClient.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("call llm api: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, resp.StatusCode, fmt.Errorf("llm stream api error status=%d body=%s", resp.StatusCode, string(respBytes))
	}
	return resp, resp.StatusCode, nil
}

// scanSSE 解析 OpenAI SSE 数据流并推送增量内容
func scanSSE(ctx context.Context, r io.Reader, ch chan<- StreamChunk) error {
	scanner := bufio.NewScanner(r)
	// 单行事件可能较大（部分网关合并分片），扩大缓冲到 1MB
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return fmt.Errorf("unmarshal stream chunk: %w", err)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		if !sendChunk(ctx, ch, StreamChunk{Content: delta}) {
			return ctx.Err()
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}
	return nil
}

// sendChunk 在尊重 ctx 取消的前提下发送一个片段；返回 false 表示上下文已取消
func sendChunk(ctx context.Context, ch chan<- StreamChunk, c StreamChunk) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- c:
		return true
	}
}
