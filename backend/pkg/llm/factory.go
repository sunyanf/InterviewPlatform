package llm

import (
	"fmt"

	"ai-interview-platform/internal/config"
)

// NewProvider 根据配置创建 LLM Provider
func NewProvider(cfg config.LLMConfig) (Provider, error) {
	switch cfg.Provider {
	case "openai":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("LLM_API_KEY is required for openai provider")
		}
		return NewOpenAIProvider(OpenAIConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	case "mock", "":
		return NewMockProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}
