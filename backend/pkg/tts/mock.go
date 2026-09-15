package tts

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// MockTTS Mock 语音合成：生成确定性的 16kHz 单声道 16-bit WAV（440Hz 正弦波）。
// 输出与输入文本内容无关（固定时长），用于本地开发与测试；
// 确定性字节保证缓存键与测试断言稳定。
type MockTTS struct{}

// NewMockTTS 创建 Mock TTS
func NewMockTTS() *MockTTS { return &MockTTS{} }

// Name 返回 Provider 名称
func (p *MockTTS) Name() string { return "mock" }

// FixedFormat mock 固定输出 WAV
func (p *MockTTS) FixedFormat() string { return "wav" }

const (
	mockSampleRate = 16000
	mockDurationS  = 0.4 // 固定 400ms，控制 mock 响应体大小
	mockFreqHz     = 440.0
)

// Synthesize 合成 mock WAV
func (p *MockTTS) Synthesize(_ context.Context, text string, _ SynthesizeOptions) (*Speech, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("empty tts text")
	}

	samples := int(mockSampleRate * mockDurationS)
	// PCM 数据
	pcm := make([]byte, samples*2)
	for i := 0; i < samples; i++ {
		// 0.3 幅度的 440Hz 正弦波，避免削波
		v := int16(0.3 * float64(math.MaxInt16) * math.Sin(2*math.Pi*mockFreqHz*float64(i)/mockSampleRate))
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(v))
	}

	wav := buildWAV(pcm, mockSampleRate, 1, 16)
	return &Speech{
		Audio:       wav,
		Format:      "wav",
		ContentType: "audio/wav",
	}, nil
}

// buildWAV 构造标准 PCM WAV 文件字节
func buildWAV(pcm []byte, sampleRate, channels, bitsPerSample int) []byte {
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := len(pcm)

	buf := make([]byte, 44+dataSize)
	offset := 0
	put := func(b []byte) { copy(buf[offset:], b); offset += len(b) }
	putString := func(s string, n int) {
		b := make([]byte, n)
		copy(b, s)
		put(b)
	}
	putUint32 := func(v uint32) {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, v)
		put(b)
	}
	putUint16 := func(v uint16) {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, v)
		put(b)
	}

	putString("RIFF", 4)
	putUint32(uint32(36 + dataSize))
	putString("WAVE", 4)
	putString("fmt ", 4)
	putUint32(16) // PCM fmt chunk size
	putUint16(1)  // PCM format
	putUint16(uint16(channels))
	putUint32(uint32(sampleRate))
	putUint32(uint32(byteRate))
	putUint16(uint16(blockAlign))
	putUint16(uint16(bitsPerSample))
	putString("data", 4)
	putUint32(uint32(dataSize))
	copy(buf[44:], pcm)
	return buf
}
