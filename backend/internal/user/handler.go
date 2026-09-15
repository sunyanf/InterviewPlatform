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

// Register 注册接口（注册即登录，返回令牌对）
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	resp, err := h.svc.Register(r.Context(), req, tokenMeta(r))
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, resp)
}

// Login 登录接口
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	resp, err := h.svc.Login(r.Context(), req, tokenMeta(r))
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Success(w, resp)
}

// Refresh 刷新令牌：旋转 refresh token，返回新令牌对
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	resp, err := h.svc.Refresh(r.Context(), req.RefreshToken, tokenMeta(r))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, resp)
}

// Logout 登出：吊销 refresh token（幂等）
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}
	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, nil)
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

// tokenMeta 从请求中提取审计元信息
func tokenMeta(r *http.Request) TokenMeta {
	return TokenMeta{
		UserAgent: r.UserAgent(),
		ClientIP:  middleware.ClientIP(r),
	}
}
