package user

import (
	"encoding/json"
	"net/http"

	"ai-interview-platform/internal/middleware"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/response"
)

// Handler 用户 HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建用户 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register 注册接口
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	u, err := h.svc.Register(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, u)
}

// Login 登录接口
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Success(w, resp)
}

// Me 获取当前用户资料
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	u, err := h.svc.GetProfile(r.Context(), userID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Success(w, u)
}
