package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// readinessTimeout 单项依赖探活超时
const readinessTimeout = 3 * time.Second

// healthResponse 健康检查响应
type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// namedCheck 命名的依赖探活函数（nil error 表示健康）
type namedCheck struct {
	name string
	fn   func(ctx context.Context) error
}

// livezHandler 存活探针：进程能响应即存活（不检查依赖，避免依赖抖动触发无谓重启）
func livezHandler(w http.ResponseWriter, r *http.Request) {
	writeHealth(w, http.StatusOK, healthResponse{Status: "ok"})
}

// readinessHandler 就绪探针：只有真实依赖（PostgreSQL、MinIO）全部可用才接收流量。
// Redis 当前未被业务代码使用，故不纳入判定；待限流/缓存接入后补充。
func (s *Server) readinessHandler(w http.ResponseWriter, r *http.Request) {
	status, resp := evaluateReadiness(r.Context(), []namedCheck{
		{"postgres", s.checkPostgres},
		{"minio", s.checkMinIO},
	})
	writeHealth(w, status, resp)
}

// evaluateReadiness 并发无关地顺序执行探活（数量少、各带 3s 超时），
// 聚合为统一状态：任一失败即 503。纯函数化以便无外部依赖单测。
func evaluateReadiness(ctx context.Context, checks []namedCheck) (int, healthResponse) {
	result := healthResponse{Status: "ok", Checks: make(map[string]string, len(checks))}
	status := http.StatusOK
	for _, c := range checks {
		if err := c.fn(ctx); err != nil {
			result.Status = "unavailable"
			status = http.StatusServiceUnavailable
			// 探活错误只含 host/port（无凭据）；readyz 为运维端点
			result.Checks[c.name] = "error: " + err.Error()
		} else {
			result.Checks[c.name] = "ok"
		}
	}
	return status, result
}

// checkPostgres 探活 PostgreSQL 连接池
func (s *Server) checkPostgres(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()
	return s.db.Ping(pingCtx)
}

// checkMinIO 探测 MinIO liveness 端点（不经过 SDK、不需要凭据）
func (s *Server) checkMinIO(ctx context.Context) error {
	scheme := "http"
	if s.cfg.Storage.UseSSL {
		scheme = "https"
	}
	url := scheme + "://" + s.cfg.Storage.Endpoint + "/minio/health/live"

	reqCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &minioHealthError{statusCode: resp.StatusCode}
	}
	return nil
}

// minioHealthError MinIO 非 200 健康响应
type minioHealthError struct{ statusCode int }

func (e *minioHealthError) Error() string {
	return "unexpected status " + http.StatusText(e.statusCode)
}

func writeHealth(w http.ResponseWriter, status int, body healthResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
