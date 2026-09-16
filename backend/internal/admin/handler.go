package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/settings"
	"ai-interview-platform/internal/user"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/llm"
	"ai-interview-platform/pkg/response"
)

// UserLister 用户查询接口（避免直接依赖具体类型）
type UserLister interface {
	ListAll(ctx context.Context, page, pageSize int) ([]*user.User, int, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

// Handler 管理后台 HTTP 处理器
type Handler struct {
	settings *settings.Service
	users    UserLister
	llmSwap  *llm.SwappableProvider
	cfg      *config.Config
	log      *slog.Logger
}

// NewHandler 创建 admin Handler
func NewHandler(settingsSvc *settings.Service, users UserLister, llmSwap *llm.SwappableProvider, cfg *config.Config, log *slog.Logger) *Handler {
	return &Handler{
		settings: settingsSvc,
		users:    users,
		llmSwap:  llmSwap,
		cfg:      cfg,
		log:      log,
	}
}

// GetSettings 列出全部设置项（API Key 掩码）
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	items, err := h.settings.All(r.Context())
	if err != nil {
		response.Error(w, apperrors.Wrap("INTERNAL_ERROR", "查询配置失败", 500, err))
		return
	}
	// 掩码 API Key
	for i := range items {
		if items[i].Key == settings.KeyLLMAPIKey && len(items[i].Value) > 8 {
			items[i].Value = items[i].Value[:3] + "***" + items[i].Value[len(items[i].Value)-4:]
		}
	}
	response.Success(w, items)
}

// UpdateSettings 批量更新设置，含 LLM 热切换
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req settings.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apperrors.New("INVALID_INPUT", "请求体格式错误", 400))
		return
	}

	// 过滤空值（空值表示不修改）
	filtered := make(settings.UpdateSettingsRequest)
	for k, v := range req {
		if v != "" {
			filtered[k] = v
		}
	}

	if len(filtered) == 0 {
		response.Error(w, apperrors.New("INVALID_INPUT", "无有效更新项", 400))
		return
	}

	// 持久化到 DB + 更新缓存
	if err := h.settings.Update(r.Context(), filtered); err != nil {
		response.Error(w, apperrors.Wrap("INTERNAL_ERROR", "保存配置失败", 500, err))
		return
	}

	// 若包含 LLM 字段则触发热切换
	hasLLM := false
	for k := range filtered {
		if strings.HasPrefix(k, "llm.") {
			hasLLM = true
			break
		}
	}

	if hasLLM {
		if err := h.rebuildLLM(); err != nil {
			h.log.Error("llm hot-swap failed, keeping old provider", "error", err)
			response.Error(w, apperrors.Wrap("LLM_SWAP_FAILED", "配置已保存但模型切换失败，需重启生效", 500, err))
			return
		}
		h.log.Info("llm provider hot-swapped", "keys", len(filtered))
	}

	// 返回更新后的设置（掩码）
	items, err := h.settings.All(r.Context())
	if err != nil {
		response.Success(w, filtered)
		return
	}
	for i := range items {
		if items[i].Key == settings.KeyLLMAPIKey && len(items[i].Value) > 8 {
			items[i].Value = items[i].Value[:3] + "***" + items[i].Value[len(items[i].Value)-4:]
		}
	}
	response.Success(w, items)
}

// ListUsers 分页列出用户
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	users, total, err := h.users.ListAll(r.Context(), page, pageSize)
	if err != nil {
		response.Error(w, apperrors.Wrap("INTERNAL_ERROR", "查询用户失败", 500, err))
		return
	}

	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	response.Success(w, map[string]any{
		"list":  users,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// UpdateUser 更新用户状态（启用/禁用）
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.Error(w, apperrors.New("INVALID_INPUT", "缺少用户 ID", 400))
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, apperrors.New("INVALID_INPUT", "请求体格式错误", 400))
		return
	}

	if body.Status != "active" && body.Status != "disabled" {
		response.Error(w, apperrors.New("INVALID_INPUT", "状态仅支持 active 或 disabled", 400))
		return
	}

	if err := h.users.UpdateStatus(r.Context(), id, body.Status); err != nil {
		response.Error(w, apperrors.Wrap("INTERNAL_ERROR", "更新用户状态失败", 500, err))
		return
	}

	response.Success(w, map[string]string{
		"id":     id,
		"status": body.Status,
	})
}

// rebuildLLM 构造新 Provider 并原子替换；失败不替换旧 Provider
func (h *Handler) rebuildLLM() error {
	newCfg := h.settings.LLMConfig()
	prov, err := llm.NewProvider(newCfg)
	if err != nil {
		return err
	}
	metered := llm.NewMeteredProvider(prov, newCfg.PriceInputPer1K, newCfg.PriceOutputPer1K)
	h.llmSwap.Swap(metered)
	return nil
}
