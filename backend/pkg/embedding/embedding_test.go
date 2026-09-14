package embedding

import (
	"context"
	"testing"

	"ai-interview-platform/internal/config"
)

func TestMockEmbedder_DeterministicAndNormalized(t *testing.T) {
	m := NewMock(1536)

	v1, err := m.Embed(context.Background(), []string{"Go 并发编程 channel", "Go 并发编程 channel"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(v1) != 2 {
		t.Fatalf("got %d vectors, want 2", len(v1))
	}

	// 确定性：相同文本 → 相同向量
	for i := range v1[0] {
		if v1[0][i] != v1[1][i] {
			t.Fatalf("same text produced different vectors at index %d", i)
		}
	}

	// L2 归一化：模长应为 1
	var norm float64
	for _, v := range v1[0] {
		norm += float64(v) * float64(v)
	}
	if norm < 0.999 || norm > 1.001 {
		t.Errorf("vector norm = %f, want ~1.0", norm)
	}

	// 不同文本 → 不同向量
	v2, _ := m.Embed(context.Background(), []string{"React 组件开发"})
	if len(v2[0]) != len(v1[0]) {
		t.Fatalf("dimension mismatch")
	}
	same := true
	for i := range v2[0] {
		if v2[0][i] != v1[0][i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different texts should produce different vectors")
	}
}

func TestMockEmbedder_EmptyInput(t *testing.T) {
	m := NewMock(0)
	v, err := m.Embed(context.Background(), []string{"", "   "})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(v) != 2 || len(v[0]) != 1536 {
		t.Errorf("unexpected output shape: %d vectors, dim %d", len(v), len(v[0]))
	}
}

func TestFormatVector(t *testing.T) {
	got := FormatVector([]float32{0.5, -1.25, 0})
	want := "[0.5,-1.25,0]"
	if got != want {
		t.Errorf("formatVector = %q, want %q", got, want)
	}
}

func TestNewProvider_Unknown(t *testing.T) {
	if _, err := NewProvider(config.EmbeddingConfig{Provider: "unknown"}); err == nil {
		t.Error("unknown provider should fail")
	}
}
