package knowledge

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/response"
)

// Handler 知识库 HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建知识库 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Create 摄取知识文档（分块 + 向量化 + 入库）
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	doc, err := h.svc.CreateDocument(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, doc)
}

// Get 文档详情
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	doc, err := h.svc.GetDocument(r.Context(), id)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, doc)
}

// List 文档列表
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	list, total, err := h.svc.ListDocuments(r.Context(), page, pageSize)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, map[string]interface{}{
		"list":  list,
		"total": total,
	})
}

// Delete 删除文档（分块级联删除）
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteDocument(r.Context(), id); err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, map[string]string{"status": "deleted"})
}

// Search 混合检索
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	result, err := h.svc.Search(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Success(w, result)
}
