package task

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"ai-interview-platform/internal/middleware"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/response"
)

// AcceptedView 202 响应：客户端凭 task_id 轮询任务状态
type AcceptedView struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// View 任务状态视图（不回显 payload，业务负载仅用于服务端鉴权）
type View struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"max_attempts"`
	LastError   string    `json:"last_error"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// taskUser 所有任务 payload 的公共字段，用于归属鉴权
type taskUser struct {
	UserID string `json:"user_id"`
}

// Handler 任务查询 HTTP 处理器
type Handler struct {
	repo *Repository
}

// NewHandler 创建任务 Handler
func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// Get 查询任务状态：仅任务所属用户（payload.user_id）可查看
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "taskID")
	if id == "" {
		response.Error(w, apperrors.ErrBadRequest)
		return
	}

	t, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		response.Error(w, apperrors.Wrap("INTERNAL_ERROR", "查询任务失败", 500, err))
		return
	}
	if t == nil {
		response.Error(w, apperrors.New("TASK_NOT_FOUND", "任务不存在", 404))
		return
	}

	var owner taskUser
	_ = json.Unmarshal(t.Payload, &owner)
	if owner.UserID != callerID {
		response.Error(w, apperrors.ErrForbidden)
		return
	}

	response.Success(w, View{
		ID:          t.ID,
		Type:        t.Type,
		Status:      t.Status,
		Attempts:    t.Attempts,
		MaxAttempts: t.MaxAttempts,
		LastError:   t.LastError,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	})
}
