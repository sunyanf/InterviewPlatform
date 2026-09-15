package asr

import "context"

// Transcript ASR 转写结果
type Transcript struct {
	// Text 转写文本
	Text string
	// Language 识别语言（如 zh / en，Provider 可能返回空）
	Language string
	// DurationMs 音频时长（Provider 可得时返回，否则为 0）
	DurationMs int64
}

// TranscribeOptions 转写选项
type TranscribeOptions struct {
	// Language 提示语言（如 "zh"），空表示自动检测
	Language string
}

// ASR 语音识别 Provider 抽象（业务模块不绑定单一厂商，对齐 pkg/llm 模式）
type ASR interface {
	// Transcribe 将音频数据转写为文本
	// audio 为完整音频文件内容；filename 用于 Provider 识别格式（如 answer.wav）
	Transcribe(ctx context.Context, audio []byte, filename string, opts TranscribeOptions) (*Transcript, error)
	// Name 返回 Provider 名称
	Name() string
}
