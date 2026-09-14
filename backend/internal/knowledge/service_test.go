package knowledge

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"ai-interview-platform/pkg/embedding"
)

func TestChunkText_ParagraphAggregation(t *testing.T) {
	// 三个短段落应聚合为一个 chunk
	content := "Go 是一门静态类型语言。\n\nGo 支持垃圾回收。\n\nGo 的并发模型基于 goroutine。"
	chunks := ChunkText(content)
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1: %v", len(chunks), chunks)
	}
	if !strings.Contains(chunks[0], "goroutine") {
		t.Errorf("chunk should contain all paragraphs")
	}
}

func TestChunkText_LongContentSplits(t *testing.T) {
	// 构造多个长段落，每段 600 字节左右，应切分为多个 chunk
	var sb strings.Builder
	for i := 0; i < 6; i++ {
		sb.WriteString(strings.Repeat("Go 语言并发编程知识点讲解。", 25)) // ~600 bytes
		sb.WriteString("\n\n")
	}
	chunks := ChunkText(sb.String())
	if len(chunks) < 2 {
		t.Fatalf("got %d chunks, want >= 2", len(chunks))
	}
	for _, c := range chunks {
		if len(c) > MaxChunkSize+DefaultChunkSize {
			t.Errorf("chunk too large: %d bytes", len(c))
		}
	}
}

func TestChunkText_OversizedParagraphSentences(t *testing.T) {
	// 单段无换行超长，应按句子边界切分
	long := strings.Repeat("这是一句话。", 100) // 500 runes / 1500 bytes
	chunks := ChunkText(long)
	if len(chunks) < 2 {
		t.Fatalf("got %d chunks, want >= 2", len(chunks))
	}
	// 每个分块都应以句号结尾（句子边界切分）
	for i, c := range chunks {
		if i < len(chunks)-1 && !strings.HasSuffix(c, "。") {
			t.Errorf("chunk %d should end with sentence boundary: %q", i, c[len(c)-10:])
		}
	}
}

func TestChunkText_Empty(t *testing.T) {
	for _, content := range []string{"", "   ", "\n\n  \n"} {
		if chunks := ChunkText(content); len(chunks) != 0 {
			t.Errorf("ChunkText(%q) = %d chunks, want 0", content, len(chunks))
		}
	}
}

func makeHit(id string) scoredChunk {
	return scoredChunk{Chunk: Chunk{ID: id}, score: 0}
}

func TestFuseRRF(t *testing.T) {
	// vector 通道: a, b, c；fts 通道: a, b, d
	channels := []string{"vector", "fts"}
	hitLists := [][]scoredChunk{
		{makeHit("a"), makeHit("b"), makeHit("c")},
		{makeHit("a"), makeHit("b"), makeHit("d")},
	}
	fused := FuseRRF(channels, hitLists, 3)

	if len(fused) != 3 {
		t.Fatalf("got %d results, want 3", len(fused))
	}
	// a 双通道 rank1: 2/61；b 双通道 rank2: 2/62 → a 最高
	if fused[0].ChunkID != "a" {
		t.Errorf("top result = %s, want a", fused[0].ChunkID)
	}
	if fused[1].ChunkID != "b" {
		t.Errorf("second result = %s, want b", fused[1].ChunkID)
	}
	// c 与 d 同分 1/63，按 ChunkID 确定性排序 → c
	if fused[2].ChunkID != "c" {
		t.Errorf("third result = %s, want c (tie-break by id)", fused[2].ChunkID)
	}
	// 命中通道记录
	if len(fused[0].Channels) != 2 {
		t.Errorf("a should hit 2 channels, got %v", fused[0].Channels)
	}
	// RRF 分数验证：a = 1/(60+1) + 1/(60+1)
	wantScore := 1.0/61 + 1.0/61
	if diff := fused[0].Score - wantScore; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("score = %f, want %f", fused[0].Score, wantScore)
	}
}

func TestFuseRRF_TopK(t *testing.T) {
	channels := []string{"vector"}
	hitLists := [][]scoredChunk{
		{makeHit("a"), makeHit("b"), makeHit("c"), makeHit("d"), makeHit("e")},
	}
	fused := FuseRRF(channels, hitLists, 2)
	if len(fused) != 2 {
		t.Fatalf("got %d results, want 2", len(fused))
	}
	if fused[0].ChunkID != "a" || fused[1].ChunkID != "b" {
		t.Errorf("top-2 = [%s, %s], want [a, b]", fused[0].ChunkID, fused[1].ChunkID)
	}
}

func TestFuseRRF_Empty(t *testing.T) {
	fused := FuseRRF([]string{"vector"}, [][]scoredChunk{nil}, 5)
	if len(fused) != 0 {
		t.Errorf("got %d results, want 0", len(fused))
	}
}

func TestSplitQueryTokens(t *testing.T) {
	tokens := splitQueryTokens("Go channel 调度原理，goroutine？")
	want := []string{"Go", "channel", "调度原理", "goroutine"}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %v, want %v", tokens, want)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Errorf("token[%d] = %q, want %q", i, tokens[i], want[i])
		}
	}
}

func TestKeywordsForSearch_CJKBigrams(t *testing.T) {
	// "调度模型" 应拆出整词 + 二元组，使部分短语（如"调度模"）也能命中
	kws := keywordsForSearch("Go channel 调度模型")
	want := []string{"go", "channel", "调度模型", "调度", "度模", "模型"}
	if len(kws) != len(want) {
		t.Fatalf("kws = %v, want %v", kws, want)
	}
	for i := range want {
		if kws[i] != want[i] {
			t.Errorf("kw[%d] = %q, want %q", i, kws[i], want[i])
		}
	}
}

func TestKeywordsForSearch_DedupAndShortFilter(t *testing.T) {
	// 单字符 ASCII 与重复关键词应被过滤/去重
	kws := keywordsForSearch("Go go a 并发 并发")
	want := []string{"go", "并发"}
	if len(kws) != len(want) {
		t.Fatalf("kws = %v, want %v", kws, want)
	}
}

func TestCheckDimensions(t *testing.T) {
	embedder := embedding.NewMock(4)
	s := &Service{
		embedder: embedder,
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if err := s.checkDimensions([][]float32{make([]float32, 4)}); err != nil {
		t.Errorf("matching dimensions should pass: %v", err)
	}
	if err := s.checkDimensions([][]float32{make([]float32, 8)}); err == nil {
		t.Error("mismatched dimensions should fail")
	}
}
