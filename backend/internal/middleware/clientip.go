package middleware

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP 提取客户端 IP。
// 优先取 X-Forwarded-For 首跳（经反向代理时），其次 X-Real-IP，最后 RemoteAddr。
// 注意：仅在受信反向代理后部署时 XFF 才可信；直连场景回退 RemoteAddr。
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			xff = xff[:idx]
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-Ip")); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
