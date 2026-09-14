package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ai-interview-platform/internal/config"
)

// OpenAIEmbedder OpenAI 兼容 Embedding Provider（标准库 net/http 实现）
// 兼容 /v1/embeddings 接口的厂商均可使用
type OpenAIEmbedder struct {
	cfg        config.EmbeddingConfig
	httpClient *http.Client
}

// NewOpenAI 创建 OpenAI 兼容 Embedder
func NewOpenAI(cfg config.EmbeddingConfig) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *OpenAIEmbedder) Name() string    { return "openai" }
func (p *OpenAIEmbedder) Dimensions() int { return p.cfg.Dimensions }

// embeddingsRequest /embeddings 请求体
type embeddingsRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

// embeddingsResponse /embeddings 响应体
type embeddingsResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed 批量生成向量（分批，每批最多 64 条）
func (p *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	const batchSize = 64
	vectors := make([][]float32, len(texts))

	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		if err := p.embedBatch(ctx, texts[start:end], vectors[start:end]); err != nil {
			return nil, err
		}
	}
	return vectors, nil
}

func (p *OpenAIEmbedder) embedBatch(ctx context.Context, texts []string, out [][]float32) error {
	reqBody, err := json.Marshal(embeddingsRequest{
		Model:      p.cfg.Model,
		Input:      texts,
		Dimensions: p.cfg.Dimensions,
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.cfg.BaseURL+"/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("embeddings api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("embeddings api status %d: %s", resp.StatusCode, string(body))
	}

	var result embeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(result.Data) != len(texts) {
		return fmt.Errorf("embeddings count mismatch: got %d, want %d", len(result.Data), len(texts))
	}

	for _, item := range result.Data {
		if item.Index < 0 || item.Index >= len(out) {
			return fmt.Errorf("embeddings index out of range: %d", item.Index)
		}
		out[item.Index] = item.Embedding
	}
	return nil
}
