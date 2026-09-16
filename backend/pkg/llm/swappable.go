package llm

import (
	"context"
	"sync/atomic"
)

// SwappableProvider 原子指针持有当前 Provider，admin 切换时不中断在途请求。
// 实现了 Provider 和 Streamer 接口，可无缝替换原有 llm.Provider 字段。
type SwappableProvider struct {
	p atomic.Pointer[Provider]
}

// NewSwappableProvider 创建可热替换的 Provider 包装器
func NewSwappableProvider(initial Provider) *SwappableProvider {
	sp := &SwappableProvider{}
	sp.p.Store(&initial)
	return sp
}

// Swap 原子替换底层 Provider（admin 调用）
func (s *SwappableProvider) Swap(new Provider) {
	s.p.Store(&new)
}

// Current 返回当前 Provider
func (s *SwappableProvider) Current() Provider {
	return *s.p.Load()
}

// Name 返回 Provider 名称
func (s *SwappableProvider) Name() string {
	return s.Current().Name()
}

// Chat 透传当前 Provider 的聊天请求
func (s *SwappableProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return s.Current().Chat(ctx, req)
}

// StreamChat 透传流式调用：当前 Provider 实现 Streamer 时委托
func (s *SwappableProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	inner := s.Current()
	streamer, ok := inner.(Streamer)
	if !ok {
		return nil, ErrStreamingNotSupported
	}
	return streamer.StreamChat(ctx, req)
}
