package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config 应用配置
type Config struct {
	App       AppConfig
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Storage   StorageConfig
	JWT       JWTConfig
	LLM       LLMConfig
	ASR       ASRConfig
	TTS       TTSConfig
	Embedding EmbeddingConfig
	Log       LogConfig
}

type AppConfig struct {
	Name string
	Env  string // dev / staging / prod
}

type ServerConfig struct {
	Port           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	AllowedOrigins []string // WebSocket 跨域白名单（ALLOWED_ORIGINS，逗号分隔）；空=放行（仅限开发）
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxConns int32
}

// DSN 返回 PostgreSQL 连接字符串
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode,
	)
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type StorageConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type JWTConfig struct {
	Secret     string
	ExpireTime time.Duration
	Issuer     string
}

type LLMConfig struct {
	Provider string // openai / mock
	APIKey   string
	BaseURL  string
	Model    string
}

type ASRConfig struct {
	Provider string // openai / mock
	APIKey   string
	BaseURL  string
	Model    string
	Language string // 默认转写语言（zh / en），空为自动检测
}

type TTSConfig struct {
	Provider string // openai / mock
	APIKey   string
	BaseURL  string
	Model    string // tts-1 / tts-1-hd
	Voice    string // alloy / echo / fable / onyx / nova / shimmer
	Format   string // mp3 / wav / opus / aac / flac
}

type EmbeddingConfig struct {
	Provider   string // openai / mock
	APIKey     string
	BaseURL    string
	Model      string
	Dimensions int
}

type LogConfig struct {
	Level  string // debug / info / warn / error
	Format string // text / json
}

// Load 从环境变量加载配置
func Load() (*Config, error) {
	_ = godotenv.Load() // .env 文件可选

	cfg := &Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "ai-interview-platform"),
			Env:  getEnv("APP_ENV", "dev"),
		},
		Server: ServerConfig{
			Port:           getEnv("SERVER_PORT", "8080"),
			ReadTimeout:    getEnvDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:   getEnvDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			AllowedOrigins: getEnvList("ALLOWED_ORIGINS"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "ai_interview"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
			MaxConns: int32(getEnvInt("DB_MAX_CONNS", 25)),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Storage: StorageConfig{
			Endpoint:  getEnv("STORAGE_ENDPOINT", "localhost:9000"),
			AccessKey: getEnv("STORAGE_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnv("STORAGE_SECRET_KEY", "minioadmin"),
			Bucket:    getEnv("STORAGE_BUCKET", "ai-interview"),
			UseSSL:    getEnvBool("STORAGE_USE_SSL", false),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "change-me-in-production"),
			ExpireTime: getEnvDuration("JWT_EXPIRE_TIME", 24*time.Hour),
			Issuer:     getEnv("JWT_ISSUER", "ai-interview-platform"),
		},
		LLM: LLMConfig{
			Provider: getEnv("LLM_PROVIDER", "mock"),
			APIKey:   getEnv("LLM_API_KEY", ""),
			BaseURL:  getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
			Model:    getEnv("LLM_MODEL", "gpt-4o-mini"),
		},
		ASR: ASRConfig{
			Provider: getEnv("ASR_PROVIDER", "mock"),
			APIKey:   getEnv("ASR_API_KEY", ""),
			BaseURL:  getEnv("ASR_BASE_URL", "https://api.openai.com/v1"),
			Model:    getEnv("ASR_MODEL", "whisper-1"),
			Language: getEnv("ASR_LANGUAGE", "zh"),
		},
		TTS: TTSConfig{
			Provider: getEnv("TTS_PROVIDER", "mock"),
			APIKey:   getEnv("TTS_API_KEY", ""),
			BaseURL:  getEnv("TTS_BASE_URL", "https://api.openai.com/v1"),
			Model:    getEnv("TTS_MODEL", "tts-1"),
			Voice:    getEnv("TTS_VOICE", "alloy"),
			Format:   getEnv("TTS_FORMAT", "mp3"),
		},
		Embedding: EmbeddingConfig{
			Provider:   getEnv("EMBEDDING_PROVIDER", "mock"),
			APIKey:     getEnv("EMBEDDING_API_KEY", ""),
			BaseURL:    getEnv("EMBEDDING_BASE_URL", "https://api.openai.com/v1"),
			Model:      getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
			Dimensions: getEnvInt("EMBEDDING_DIMENSIONS", 1536),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate 校验关键配置
func (c *Config) Validate() error {
	if c.App.Env == "prod" {
		if c.JWT.Secret == "change-me-in-production" {
			return fmt.Errorf("JWT_SECRET must be set in production")
		}
		if len(c.Server.AllowedOrigins) == 0 {
			return fmt.Errorf("ALLOWED_ORIGINS must be set in production (empty whitelist allows any WebSocket origin)")
		}
	}
	if c.Server.Port == "" {
		return fmt.Errorf("SERVER_PORT is required")
	}
	return nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

// getEnvList 解析逗号分隔的环境变量为切片，自动去空白、丢弃空条目；未设置返回 nil
func getEnvList(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	return result
}
