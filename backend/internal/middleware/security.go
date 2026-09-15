package middleware

import (
	"net/http"
	"strings"

	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/response"
)

// SecureHeaders 注入基线安全响应头（API 服务，无第三方 HTML 嵌入需求）
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-XSS-Protection", "0") // 现代浏览器建议关闭遗留 XSS Auditor
		next.ServeHTTP(w, r)
	})
}

// MaxBody 限制请求体大小：先按 Content-Length 快速拒绝，再用 MaxBytesReader 兜底
// （chunked 请求无 Content-Length，由 MaxBytesReader 在读超限时返回错误）。
func MaxBody(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				response.Error(w, apperrors.ErrPayloadTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

// CORS 跨域中间件。
// allowedOrigins 为空时放行所有 Origin（仅限本地开发，生产由 config.Validate 强制配置）；
// 非空时精确匹配白名单。同源请求（无 Origin 头）直接放行。
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" {
				permitted := len(allowed) == 0
				if !permitted {
					_, permitted = allowed[origin]
				}
				if permitted {
					h := w.Header()
					h.Set("Access-Control-Allow-Origin", origin)
					h.Set("Vary", "Origin")
					h.Set("Access-Control-Allow-Credentials", "true")
					h.Set("Access-Control-Expose-Headers", "X-Request-ID")
				}
			}

			// 预检请求
			if r.Method == http.MethodOptions && origin != "" {
				h := w.Header()
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
