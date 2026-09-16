package settings

import "time"

// 配置项 key 常量
const (
	KeyLLMProvider         = "llm.provider"
	KeyLLMAPIKey           = "llm.api_key"
	KeyLLMBaseURL          = "llm.base_url"
	KeyLLMModel            = "llm.model"
	KeyLLMPriceInputPer1K  = "llm.price_input_per_1k"
	KeyLLMPriceOutputPer1K = "llm.price_output_per_1k"

	KeyRateLimitEnabled    = "rate_limit.enabled"
	KeyRateLimitAuthPerMin = "rate_limit.auth_per_min"
	KeyRateLimitAPIPerMin  = "rate_limit.api_per_min"

	KeyTaskEnabled         = "task.enabled"
	KeyTaskWorkers         = "task.workers"
	KeyTaskPollInterval    = "task.poll_interval"
	KeyTaskLeaseTimeout    = "task.lease_timeout"
	KeyTaskShutdownTimeout = "task.shutdown_timeout"
)

// Setting 配置项
type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpdateSettingsRequest 批量更新请求 { "llm.model": "gpt-4o", ... }
type UpdateSettingsRequest map[string]string
