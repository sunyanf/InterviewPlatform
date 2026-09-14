package job

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/response"
)

// Handler 岗位 HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建岗位 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ListCategories 岗位分类列表
func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListCategories(r.Context())
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, list)
}

// CreateJob 创建岗位
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	j, err := h.svc.CreateJob(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, j)
}

// GetJob 岗位详情
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	j, err := h.svc.GetJob(r.Context(), id)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, j)
}

// ListJobs 岗位列表
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	req := ListJobsRequest{
		CategoryID: r.URL.Query().Get("category_id"),
		Page:       atoi(r.URL.Query().Get("page")),
		PageSize:   atoi(r.URL.Query().Get("page_size")),
	}

	list, total, err := h.svc.ListJobs(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	response.Success(w, map[string]interface{}{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// atoi 字符串转整数，失败返回 0
func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
