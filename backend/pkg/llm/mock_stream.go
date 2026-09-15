package llm

import "context"

// mockStreamChunkRunes mock 流式输出每个分片包含的 rune 数（固定值保证测试确定性）
const mockStreamChunkRunes = 4

// StreamChat Mock 流式输出：复用 Chat 的完整响应，按固定 rune 数切块推送
func (p *MockProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		runes := []rune(resp.Content)
		for i := 0; i < len(runes); i += mockStreamChunkRunes {
			end := i + mockStreamChunkRunes
			if end > len(runes) {
				end = len(runes)
			}
			select {
			case <-ctx.Done():
				ch <- StreamChunk{Err: ctx.Err()}
				return
			case ch <- StreamChunk{Content: string(runes[i:end])}:
			}
		}
	}()
	return ch, nil
}
