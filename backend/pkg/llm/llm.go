package llm

import "context"

// Message 聊天消息
type Message struct {
	Role    string `json:"role"` // system / user / assistant
	Content string `json:"content"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Content      string
	Model        string
	InputTokens  int
	OutputTokens int
}

// Provider LLM Provider 接口
// 业务层只依赖此接口，不直接绑定具体厂商
type Provider interface {
	// Chat 完成一次聊天请求
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// Name 返回 Provider 名称
	Name() string
}
