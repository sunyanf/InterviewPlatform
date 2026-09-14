package handler

import (
	"net/http"

	"ai-interview-platform/pkg/response"
)

// Health 健康检查
func Health(w http.ResponseWriter, r *http.Request) {
	response.Success(w, map[string]string{"status": "ok"})
}
