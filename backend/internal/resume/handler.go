package resume

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ai-interview-platform/internal/middleware"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/response"
)

// Handler 简历 HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建简历 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Upload 上传简历
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// 限制上传大小 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)

	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		response.Error(w, apperrors.New("INVALID_FILE", "文件解析失败或超过 10MB", 400))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, apperrors.New("NO_FILE", "请上传简历文件", 400))
		return
	}
	defer file.Close()

	rs, err := h.svc.Upload(r.Context(), userID, header.Filename, header.Size, file)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, rs)
}

// Parse 提交简历解析任务（异步：202 + task_id，前端轮询简历状态或任务状态）
func (h *Handler) Parse(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	view, err := h.svc.RequestParse(r.Context(), userID, id)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Accepted(w, view)
}

// Get 简历详情
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rs, err := h.svc.Get(r.Context(), id)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, rs)
}

// List 简历列表
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	list, err := h.svc.List(r.Context(), userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, list)
}

// Match 岗位-简历匹配
func (h *Handler) Match(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		response.Error(w, apperrors.New("MISSING_JOB_ID", "请提供 job_id 参数", 400))
		return
	}

	result, err := h.svc.Match(r.Context(), id, jobID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, result)
}

// atoi 字符串转整数
func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
