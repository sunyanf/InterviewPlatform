package knowledge

import "strings"

const (
	// DefaultChunkSize 目标分块大小（字符）
	DefaultChunkSize = 500
	// MaxChunkSize 单段超过此大小时强制按句子切分
	MaxChunkSize = 800
)

// ChunkText 将文档内容切分为语义相对完整的分块。
// 策略：优先按段落聚合；单段超长时按句子边界切分。
// Chunk 必须语义完整、不跨主题（见 docs/RAG.md #5）。
func ChunkText(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if strings.TrimSpace(content) == "" {
		return nil
	}

	paragraphs := strings.Split(content, "\n")
	var chunks []string
	var buf []string
	bufLen := 0

	flush := func() {
		text := strings.TrimSpace(strings.Join(buf, "\n"))
		if text != "" {
			chunks = append(chunks, text)
		}
		buf = buf[:0]
		bufLen = 0
	}

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 超长段落：按句子边界切分
		if len(p) > MaxChunkSize {
			if bufLen > 0 {
				flush()
			}
			for _, piece := range splitLongParagraph(p) {
				chunks = append(chunks, piece)
			}
			continue
		}
		// 加入当前段会超过目标大小，先落盘
		if bufLen > 0 && bufLen+len(p) > DefaultChunkSize {
			flush()
		}
		buf = append(buf, p)
		bufLen += len(p)
	}
	flush()

	return chunks
}

// splitLongParagraph 按句子边界（中英文句末标点）切分超长段落
func splitLongParagraph(p string) []string {
	sentences := splitSentences(p)

	var out []string
	var buf strings.Builder
	for _, s := range sentences {
		if buf.Len() > 0 && buf.Len()+len(s) > DefaultChunkSize {
			out = append(out, strings.TrimSpace(buf.String()))
			buf.Reset()
		}
		buf.WriteString(s)
	}
	if s := strings.TrimSpace(buf.String()); s != "" {
		out = append(out, s)
	}
	return out
}

// splitSentences 按句末标点切分句子（保留标点）
func splitSentences(p string) []string {
	var sentences []string
	var buf strings.Builder
	for _, r := range p {
		buf.WriteRune(r)
		switch r {
		case '。', '！', '？', '；', '.', '!', '?', ';':
			sentences = append(sentences, buf.String())
			buf.Reset()
		}
	}
	if s := buf.String(); strings.TrimSpace(s) != "" {
		sentences = append(sentences, s)
	}
	return sentences
}
