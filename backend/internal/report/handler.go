package report

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"ai-interview-platform/internal/middleware"
	"ai-interview-platform/pkg/response"
)

// Handler 报告 HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建报告 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Generate 触发生成报告（重跑覆盖；前置：评估已完成）
func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	sessionID := chi.URLParam(r, "sessionID")

	rp, err := h.svc.Generate(r.Context(), userID, sessionID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, rp)
}

// Get 查询会话报告
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	sessionID := chi.URLParam(r, "sessionID")

	rp, err := h.svc.Get(r.Context(), userID, sessionID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, rp)
}

// List 查询当前用户的报告列表
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	list, err := h.svc.List(r.Context(), userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, map[string]interface{}{
		"list":  list,
		"total": len(list),
	})
}
