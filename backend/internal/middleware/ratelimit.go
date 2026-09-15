package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/response"
)

// Limiter 单机内存令牌桶限流器（按 key 隔离）。
// MVP 为单实例部署；多实例水平扩容时需替换为 Redis 版（同一接口语义）。
//
// 生命周期：Start 启动过期桶清理 goroutine；Shutdown 停止（AGENTS.md #22）。
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rate     float64 // 每秒补充令牌数
	burst    float64 // 桶容量（允许的瞬时突发）
	stopCh   chan struct{}
	stopOnce sync.Once
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

// NewLimiter 创建限流器；perMinute 为稳态每分钟允许请求数，burst 为突发容量。
func NewLimiter(perMinute, burst int) *Limiter {
	return &Limiter{
		buckets: make(map[string]*tokenBucket),
		rate:    float64(perMinute) / 60.0,
		burst:   float64(burst),
		stopCh:  make(chan struct{}),
	}
}

// Start 启动周期性清理 goroutine（随服务 ctx 退出也会停止）
func (l *Limiter) Start(ctx context.Context) {
	go l.janitor(ctx)
}

// Shutdown 停止清理 goroutine
func (l *Limiter) Shutdown() {
	l.stopOnce.Do(func() { close(l.stopCh) })
}

func (l *Limiter) janitor(ctx context.Context) {
	// 清理间隔：空闲桶超过 3 分钟未使用即删除；最低每分钟扫一次
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	idleThreshold := 3 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.stopCh:
			return
		case now := <-ticker.C:
			l.mu.Lock()
			for k, b := range l.buckets {
				if now.Sub(b.last) > idleThreshold {
					delete(l.buckets, k)
				}
			}
			l.mu.Unlock()
		}
	}
}

// Allow 判断 key 当前是否可取用一个令牌（惰性补充）
func (l *Limiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimit 限流中间件；keyFn 决定按什么维度限流（IP / 用户 ID）
func RateLimit(l *Limiter, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(keyFn(r)) {
				w.Header().Set("Retry-After", "1")
				response.Error(w, apperrors.ErrRateLimited)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// KeyByIP 按客户端 IP 限流
func KeyByIP(r *http.Request) string {
	return "ip:" + ClientIP(r)
}

// KeyByUser 优先按登录用户限流，未登录回退 IP
func KeyByUser(r *http.Request) string {
	if uid := GetUserID(r.Context()); uid != "" {
		return "user:" + uid
	}
	return KeyByIP(r)
}
