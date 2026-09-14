package embedding

import (
	"context"
	"fmt"

	"ai-interview-platform/internal/config"
)

// Embedder Embedding Provider 抽象接口
// 业务模块不得直接绑定单一模型厂商（见 AGENTS.md #12）
type Embedder interface {
	// Embed 批量生成文本向量，返回顺序与输入一致
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Name 返回 Provider 名称
	Name() string
	// Dimensions 返回向量维度
	Dimensions() int
}

// NewProvider 根据 config 创建 Embedding Provider
func NewProvider(cfg config.EmbeddingConfig) (Embedder, error) {
	switch cfg.Provider {
	case "openai":
		return NewOpenAI(cfg), nil
	case "mock":
		return NewMock(cfg.Dimensions), nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", cfg.Provider)
	}
}

// FormatVector 将向量转为 pgvector 文本格式 '[0.1,0.2,...]'
func FormatVector(vec []float32) string {
	out := make([]byte, 0, len(vec)*10)
	out = append(out, '[')
	for i, v := range vec {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, []byte(fmt.Sprintf("%g", v))...)
	}
	out = append(out, ']')
	return string(out)
}
