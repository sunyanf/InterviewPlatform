package evaluation

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"ai-interview-platform/internal/middleware"
	"ai-interview-platform/pkg/response"
)

// Handler 评估 HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建评估 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Evaluate 提交面试评估任务（异步：202 + task_id；可重复触发，评估可重跑覆盖）
func (h *Handler) Evaluate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	sessionID := chi.URLParam(r, "sessionID")

	view, err := h.svc.RequestEvaluate(r.Context(), userID, sessionID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Accepted(w, view)
}

// Get 查询会话评估
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	sessionID := chi.URLParam(r, "sessionID")

	e, err := h.svc.Get(r.Context(), userID, sessionID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, e)
}

// List 查询当前用户的评估列表
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
