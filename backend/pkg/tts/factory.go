package tts

import (
	"fmt"

	"ai-interview-platform/internal/config"
)

// NewTTS 根据配置创建 TTS Provider
func NewTTS(cfg config.TTSConfig) (TTS, error) {
	switch cfg.Provider {
	case "openai":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("TTS_API_KEY is required for openai provider")
		}
		return NewOpenAITTS(OpenAIConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
			Voice:   cfg.Voice,
			Format:  cfg.Format,
		}), nil
	case "mock", "":
		return NewMockTTS(), nil
	default:
		return nil, fmt.Errorf("unsupported TTS provider: %s", cfg.Provider)
	}
}
