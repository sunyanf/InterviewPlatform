package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"ai-interview-platform/pkg/metrics"
)

// Metrics 记录 HTTP 请求计数与耗时直方图。
// 路由名取 chi 匹配后的 RoutePattern（如 /api/v1/jobs/{id}），
// 未匹配路由记为 "unmatched"，避免把任意路径打成高基数标签。
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(rec.status)
		metrics.HTTPRequests.Inc(r.Method, route, status)
		metrics.HTTPDuration.Observe(time.Since(start).Seconds(), r.Method, route)
	})
}
