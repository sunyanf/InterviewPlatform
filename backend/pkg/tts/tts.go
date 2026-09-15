package tts

import "context"

// Speech 合成语音结果
type Speech struct {
	// Audio 音频文件内容
	Audio []byte
	// Format 音频格式（wav / mp3 / opus / aac / flac）
	Format string
	// ContentType HTTP Content-Type（如 audio/mpeg）
	ContentType string
}

// SynthesizeOptions 合成选项
type SynthesizeOptions struct {
	// Voice 音色（OpenAI: alloy / echo / fable / onyx / nova / shimmer），空则用 Provider 默认
	Voice string
	// Format 输出格式，空则用 Provider 默认
	Format string
	// Speed 语速（0.25-4.0，1.0 为正常），0 表示不指定
	Speed float64
}

// TTS 语音合成 Provider 抽象（业务模块不绑定单一厂商，对齐 pkg/llm、pkg/asr 模式）
type TTS interface {
	// Synthesize 将文本合成为语音音频
	Synthesize(ctx context.Context, text string, opts SynthesizeOptions) (*Speech, error)
	// Name 返回 Provider 名称
	Name() string
}

// FixedFormatProvider 输出格式固定、与请求格式无关的 Provider
// （如 mock 始终产出 WAV）。业务层据此选择缓存键格式，避免每次缓存 miss。
type FixedFormatProvider interface {
	FixedFormat() string
}

// FormatContentType 格式到 Content-Type 的映射，未知格式默认 audio/mpeg
func FormatContentType(format string) string {
	switch format {
	case "wav":
		return "audio/wav"
	case "mp3":
		return "audio/mpeg"
	case "opus":
		return "audio/ogg"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "pcm":
		return "audio/L16"
	default:
		return "audio/mpeg"
	}
}
