package asr

import (
	"context"
	"fmt"
	"strings"
)

// MockASR Mock ASR Provider，用于开发和测试
// 返回预定义转写文本，不调用真实 API
type MockASR struct{}

// NewMockASR 创建 Mock ASR
func NewMockASR() *MockASR {
	return &MockASR{}
}

// Name 返回 Provider 名称
func (m *MockASR) Name() string {
	return "mock"
}

// Transcribe 返回 Mock 转写结果
func (m *MockASR) Transcribe(ctx context.Context, audio []byte, filename string, opts TranscribeOptions) (*Transcript, error) {
	if len(audio) == 0 {
		return nil, fmt.Errorf("empty audio data")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 根据格式返回不同的 mock 转写文本
	var text string
	switch {
	case strings.Contains(filename, "empty"):
		text = ""
	default:
		text = "嗯，我觉得 goroutine 是用户态的轻量级协程，那个初始栈很小可以动态增长。呃，channel 遵循 CSP 模型用于通信，然后还有 select 可以多路复用。"
	}

	return &Transcript{
		Text:       text,
		Language:   "zh",
		DurationMs: 0, // Mock 无真实时长，由业务侧使用客户端上报时长
	}, nil
}
