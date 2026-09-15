package realtime

import (
	"net/http"
	"strings"
)

// newOriginChecker 构造 WebSocket 升级的 Origin 校验函数。
//
// 规则：
//   - allowedOrigins 为空：放行所有来源（仅用于本地开发；生产必须配置 ALLOWED_ORIGINS）
//   - 请求不带 Origin 头：放行（非浏览器客户端，如 curl / 服务端 / 原生 WS）
//   - Origin 精确匹配白名单（scheme://host[:port]，大小写不敏感，忽略结尾斜线）：放行
//   - 其余跨域页面：拒绝
func newOriginChecker(allowedOrigins []string) func(r *http.Request) bool {
	if len(allowedOrigins) == 0 {
		return func(*http.Request) bool { return true }
	}
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if n := normalizeOrigin(o); n != "" {
			allowed[n] = struct{}{}
		}
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		_, ok := allowed[normalizeOrigin(origin)]
		return ok
	}
}

// normalizeOrigin 去空白、转小写、去结尾斜线，便于精确比较
func normalizeOrigin(s string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(s), "/"))
}
