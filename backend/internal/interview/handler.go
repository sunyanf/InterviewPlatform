package interview

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ai-interview-platform/internal/middleware"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/response"
)

// Handler 面试 HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建面试 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Create 创建面试会话
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	sess, err := h.svc.Create(r.Context(), userID, req)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, sess)
}

// Start 开始面试
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")

	sess, err := h.svc.Start(r.Context(), userID, id)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Success(w, sess)
}

// Get 面试详情
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")

	sess, err := h.svc.Get(r.Context(), userID, id)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Success(w, sess)
}

// List 面试列表
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	list, err := h.svc.List(r.Context(), userID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Success(w, list)
}

// SubmitAnswer 提交回答
func (h *Handler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")

	var req SubmitAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	answer, err := h.svc.SubmitAnswer(r.Context(), userID, id, req)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, answer)
}

// Finish 结束面试
func (h *Handler) Finish(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")

	sess, err := h.svc.Finish(r.Context(), userID, id)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Success(w, sess)
}
