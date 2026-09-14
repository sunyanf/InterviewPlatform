package embedding

import (
	"context"
	"hash/fnv"
	"math"
)

// MockEmbedder 确定性哈希向量，用于本地开发和测试（不调用外部 API）
// 相同文本生成相同向量；向量搜索语义近似度为零，仅用于跑通链路，
// 开发环境检索质量依赖关键词（FTS）通道。
type MockEmbedder struct {
	dimensions int
}

// NewMock 创建 Mock Embedder
func NewMock(dimensions int) *MockEmbedder {
	if dimensions <= 0 {
		dimensions = 1536
	}
	return &MockEmbedder{dimensions: dimensions}
}

func (m *MockEmbedder) Name() string    { return "mock" }
func (m *MockEmbedder) Dimensions() int { return m.dimensions }

// Embed 基于 FNV 哈希的确定性向量：按 token 哈希落位，+1/-1 交替，最后 L2 归一化
func (m *MockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vectors[i] = m.embedOne(text)
	}
	return vectors, nil
}

func (m *MockEmbedder) embedOne(text string) []float32 {
	vec := make([]float32, m.dimensions)

	// 按空白和标点粗分 token（对中英文都可用）
	var token []rune
	flush := func() {
		if len(token) == 0 {
			return
		}
		h := fnv.New64a()
		_, _ = h.Write([]byte(string(token)))
		idx := int(h.Sum64() % uint64(m.dimensions))
		sign := float32(1)
		if h.Sum64()&(1<<63) != 0 {
			sign = -1
		}
		vec[idx] += sign
		token = token[:0]
	}
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
			r == '、' || r == '，' || r == '。' || r == '；' || r == '：' || r == ',' || r == '.' || r == ';' || r == ':' {
			flush()
			continue
		}
		token = append(token, r)
	}
	flush()

	// L2 归一化
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		norm = 1.0 / math.Sqrt(norm)
		for i := range vec {
			vec[i] *= float32(norm)
		}
	}
	return vec
}
