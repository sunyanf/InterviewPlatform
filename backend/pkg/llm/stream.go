package llm

import "context"

// StreamChunk 流式输出片段。
// 终端事件约定：channel 关闭表示流正常结束；
// 异常时先发送 Err 非空的片段，随后关闭 channel。
type StreamChunk struct {
	Content string
	Err     error
}

// Streamer 可选的流式能力接口。
// Provider 实现该接口即表示支持 token 级流式输出；
// 业务层通过类型断言判断能力，不支持时走非流式 Chat。
type Streamer interface {
	// StreamChat 发起流式聊天，返回增量片段 channel。
	// 返回错误表示流未能建立；流建立后的错误通过 StreamChunk.Err 传递。
	StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}
