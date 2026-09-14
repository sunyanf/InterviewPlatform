package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// MockProvider Mock LLM Provider，用于开发和测试
// 返回预定义的结构化输出，不调用真实 API
type MockProvider struct{}

// NewMockProvider 创建 Mock Provider
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// Name 返回 Provider 名称
func (p *MockProvider) Name() string {
	return "mock"
}

// Chat 返回 Mock 响应
func (p *MockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// 根据最后一条消息内容返回不同的 mock 结果
	lastMsg := ""
	if len(req.Messages) > 0 {
		lastMsg = req.Messages[len(req.Messages)-1].Content
	}

	// 简历解析 mock 输出
	if contains(lastMsg, "简历") || contains(lastMsg, "resume") {
		mockResume := map[string]interface{}{
			"name":  "张三",
			"email": "zhangsan@example.com",
			"phone": "13800138000",
			"education": []map[string]string{
				{"school": "某某大学", "major": "计算机科学与技术", "degree": "本科"},
			},
			"work_experience": []map[string]string{
				{"company": "某某科技", "position": "后端工程师", "duration": "2021-至今"},
			},
			"skills":          []string{"Go", "MySQL", "Redis", "Kafka", "微服务", "Docker"},
			"summary":         "5年后端开发经验，熟悉高并发系统设计",
			"target_position": "后端工程师",
		}
		data, _ := json.Marshal(mockResume)
		return &ChatResponse{
			Content: string(data),
			Model:   "mock",
		}, nil
	}

	// 默认返回简单响应
	return &ChatResponse{
		Content:      fmt.Sprintf(`{"response": "mock response for: %s"}`, truncate(lastMsg, 50)),
		Model:        "mock",
		InputTokens:  10,
		OutputTokens: 20,
	}, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
