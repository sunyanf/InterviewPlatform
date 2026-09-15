package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"ai-interview-platform/pkg/requestid"
)

// RequestID 为每个请求生成唯一 ID 并注入上下文和响应头
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}

		ctx := requestid.With(r.Context(), id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID 从上下文中获取 Request ID
func GetRequestID(ctx context.Context) string {
	return requestid.From(ctx)
}
