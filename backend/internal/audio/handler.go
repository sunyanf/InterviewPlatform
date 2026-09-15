package audio

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ai-interview-platform/internal/middleware"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/response"
)

// Handler 语音 HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建语音 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Upload 上传答案语音（multipart: file + duration_ms）
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	questionID := chi.URLParam(r, "questionID")

	// 限制上传大小 10MB
	r.Body = http.MaxBytesReader(w, r.Body, maxAudioSize)
	if err := r.ParseMultipartForm(maxAudioSize); err != nil {
		response.Error(w, apperrors.New("INVALID_FILE", "文件解析失败或超过 10MB", 400))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, apperrors.New("INVALID_FILE", "缺少 file 字段", 400))
		return
	}
	defer file.Close()

	// 客户端上报时长（可选，不可信仅作参考）
	durationMs, _ := strconv.ParseInt(r.FormValue("duration_ms"), 10, 64)

	a, err := h.svc.Upload(r.Context(), userID, questionID, header.Filename, header.Size, file, durationMs)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, a)
}

// Transcribe 转写语音为文本
func (h *Handler) Transcribe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	questionID := chi.URLParam(r, "questionID")

	a, err := h.svc.Transcribe(r.Context(), userID, questionID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, a)
}

// Analyze 语音表达分析
func (h *Handler) Analyze(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	questionID := chi.URLParam(r, "questionID")

	view, err := h.svc.Analyze(r.Context(), userID, questionID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, view)
}

// Get 查询语音详情
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	questionID := chi.URLParam(r, "questionID")

	view, err := h.svc.Get(r.Context(), userID, questionID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, view)
}
