package tts

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"ai-interview-platform/internal/middleware"
	"ai-interview-platform/pkg/response"
)

// Handler 面试官语音 HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建 TTS Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Opening GET /api/v1/interviews/{id}/speech/opening
func (h *Handler) Opening(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	sessionID := chi.URLParam(r, "id")

	res, err := h.svc.SynthesizeOpening(r.Context(), userID, sessionID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, res)
}

// Question GET /api/v1/interviews/{id}/questions/{questionID}/speech
func (h *Handler) Question(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	sessionID := chi.URLParam(r, "id")
	questionID := chi.URLParam(r, "questionID")

	res, err := h.svc.SynthesizeQuestion(r.Context(), userID, sessionID, questionID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, res)
}
