package settings

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"ai-interview-platform/internal/config"
)

// Service 配置中心：内存缓存 + DB 持久化
type Service struct {
	repo  *Repository
	cfg   *config.Config
	mu    sync.RWMutex
	cache map[string]string
	log   *slog.Logger
}

// NewService 创建配置 Service
func NewService(repo *Repository, cfg *config.Config, log *slog.Logger) *Service {
	return &Service{
		repo:  repo,
		cfg:   cfg,
		cache: make(map[string]string),
		log:   log,
	}
}

// Load 启动时加载：DB 中空值用 env 默认填充，再读入缓存
func (s *Service) Load(ctx context.Context) error {
	// 确保 DB 有全部预置行
	defaults := s.envDefaults()
	if err := s.repo.EnsureDefaults(ctx, defaults); err != nil {
		return err
	}

	// 读入缓存，空值用 env 填充并回写 DB
	items, err := s.repo.GetAll(ctx)
	if err != nil {
		return err
	}

	updates := make(map[string]string)
	s.mu.Lock()
	for _, item := range items {
		if item.Value == "" {
			if def, ok := defaults[item.Key]; ok && def != "" {
				item.Value = def
				updates[item.Key] = def
			}
		}
		s.cache[item.Key] = item.Value
	}
	s.mu.Unlock()

	// 回写空值填充
	if len(updates) > 0 {
		if err := s.repo.UpsertMany(ctx, updates); err != nil {
			s.log.Warn("backfill settings failed", "error", err)
		}
	}

	s.log.Info("settings loaded", "count", len(s.cache))
	return nil
}

// Get 取字符串值
func (s *Service) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[key]
}

// GetInt 取整数值（parse 失败回退 0）
func (s *Service) GetInt(key string) int {
	v := s.Get(key)
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

// GetBool 取布尔值
func (s *Service) GetBool(key string) bool {
	v := s.Get(key)
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}

// GetDuration 取时长值（parse 失败回退 0）
func (s *Service) GetDuration(key string) time.Duration {
	v := s.Get(key)
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0
	}
	return d
}

// Update 批量更新：tx 写 DB + 同步更新 cache
func (s *Service) Update(ctx context.Context, req UpdateSettingsRequest) error {
	if err := s.repo.UpsertMany(ctx, map[string]string(req)); err != nil {
		return err
	}
	s.mu.Lock()
	for k, v := range req {
		s.cache[k] = v
	}
	s.mu.Unlock()
	return nil
}

// All 返回全部设置（admin 列表用）
func (s *Service) All(ctx context.Context) ([]Setting, error) {
	return s.repo.GetAll(ctx)
}

// LLMConfig 用 settings 覆盖 cfg.LLM 返回新 LLMConfig
func (s *Service) LLMConfig() config.LLMConfig {
	result := s.cfg.LLM // 从 env 默认值开始

	if v := s.Get(KeyLLMProvider); v != "" {
		result.Provider = v
	}
	if v := s.Get(KeyLLMAPIKey); v != "" {
		result.APIKey = v
	}
	if v := s.Get(KeyLLMBaseURL); v != "" {
		result.BaseURL = v
	}
	if v := s.Get(KeyLLMModel); v != "" {
		result.Model = v
	}
	// 价格允许 0 值
	if v := s.Get(KeyLLMPriceInputPer1K); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			result.PriceInputPer1K = f
		}
	}
	if v := s.Get(KeyLLMPriceOutputPer1K); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			result.PriceOutputPer1K = f
		}
	}

	return result
}

// envDefaults 从 cfg 提取 env 默认值
func (s *Service) envDefaults() map[string]string {
	return map[string]string{
		KeyLLMProvider:         s.cfg.LLM.Provider,
		KeyLLMAPIKey:           s.cfg.LLM.APIKey,
		KeyLLMBaseURL:          s.cfg.LLM.BaseURL,
		KeyLLMModel:            s.cfg.LLM.Model,
		KeyLLMPriceInputPer1K:  strconv.FormatFloat(s.cfg.LLM.PriceInputPer1K, 'f', -1, 64),
		KeyLLMPriceOutputPer1K: strconv.FormatFloat(s.cfg.LLM.PriceOutputPer1K, 'f', -1, 64),
		KeyRateLimitEnabled:    strconv.FormatBool(s.cfg.Security.RateLimitEnabled),
		KeyRateLimitAuthPerMin: strconv.Itoa(s.cfg.Security.AuthRatePerMinute),
		KeyRateLimitAPIPerMin:  strconv.Itoa(s.cfg.Security.APIRatePerMinute),
		KeyTaskEnabled:         strconv.FormatBool(s.cfg.Task.Enabled),
		KeyTaskWorkers:         strconv.Itoa(s.cfg.Task.Workers),
		KeyTaskPollInterval:    s.cfg.Task.PollInterval.String(),
		KeyTaskLeaseTimeout:    s.cfg.Task.LeaseTimeout.String(),
		KeyTaskShutdownTimeout: s.cfg.Task.ShutdownTimeout.String(),
	}
}
