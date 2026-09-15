package asr

import (
	"fmt"

	"ai-interview-platform/internal/config"
)

// NewASR 根据配置创建 ASR Provider
func NewASR(cfg config.ASRConfig) (ASR, error) {
	switch cfg.Provider {
	case "openai":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("ASR_API_KEY is required for openai provider")
		}
		return NewOpenAIASR(ASRConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	case "mock", "":
		return NewMockASR(), nil
	default:
		return nil, fmt.Errorf("unsupported ASR provider: %s", cfg.Provider)
	}
}
