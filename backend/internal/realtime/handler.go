package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"ai-interview-platform/internal/agent"
	"ai-interview-platform/internal/interview"
	ttssvc "ai-interview-platform/internal/tts"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/jwt"
	"ai-interview-platform/pkg/llm"
)

const (
	maxChatMessageRunes = 2000 // 单条实时对话最大长度
	readLimitBytes      = 1 << 20
)

// InterviewService realtime 依赖的面试服务最小接口（生产由 *interview.Service 满足，测试可用 fake）
type InterviewService interface {
	Get(ctx context.Context, userID, sessionID string) (*interview.Session, error)
	SubmitAnswer(ctx context.Context, userID, sessionID string, req interview.SubmitAnswerRequest) (*interview.SubmitAnswerResult, error)
}

// ChatAgent realtime 依赖的流式对话能力（生产由 *agent.Agent 满足）
type ChatAgent interface {
	ChatInterviewer(ctx context.Context, in agent.RealtimeChatInput) (<-chan agent.RealtimeChatEvent, error)
}

// SpeechSynthesizer 实时对话回复的语音合成能力（生产由 *tts.Service 满足，可为 nil）
type SpeechSynthesizer interface {
	SynthesizeForSession(ctx context.Context, userID, sessionID, text string) (*ttssvc.SpeechResult, error)
}

// Handler WebSocket 实时面试处理器
type Handler struct {
	jwtMgr *jwt.Manager
	svc    InterviewService
	agent  ChatAgent
	speech SpeechSynthesizer
	hubs   *HubManager
	log    *slog.Logger

	upgrader websocket.Upgrader
}

// NewHandler 创建实时面试 Handler。speech 传 nil 时，chat 消息的 tts 选项会返回 TTS_UNAVAILABLE。
func NewHandler(jwtMgr *jwt.Manager, svc InterviewService, ag ChatAgent, speech SpeechSynthesizer, log *slog.Logger) *Handler {
	return &Handler{
		jwtMgr: jwtMgr,
		svc:    svc,
		agent:  ag,
		speech: speech,
		hubs:   NewHubManager(),
		log:    log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			// MVP 阶段前后端分离开发，允许跨域 WS；生产应由同源策略 / 网关保证
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// HandleWS GET /api/v1/interviews/{id}/ws?token=...
// 浏览器 WebSocket 无法设置 Authorization 头，JWT 通过 query 参数传递。
func (h *Handler) HandleWS(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")

	// 鉴权（query token；注意不得把 token 记入访问日志，见 Logger 中间件脱敏）
	claims, err := h.jwtMgr.Parse(r.URL.Query().Get("token"))
	if err != nil {
		http.Error(w, apperrors.ErrInvalidToken.Message, http.StatusUnauthorized)
		return
	}
	userID := claims.UserID

	// 会话归属校验 + 全量快照数据（403/404 在升级前以普通 HTTP 响应返回）
	// 注意：不能使用 r.Context()，HTTP handler 返回后该 context 即被取消
	sess, err := h.svc.Get(context.Background(), userID, sessionID)
	if err != nil {
		writeHTTPError(w, err)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("websocket upgrade failed", "session_id", sessionID, "error", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newClient(sessionID, userID, conn)
	hub := h.hubs.acquire(sessionID)
	if old := hub.replace(client); old != nil {
		// 新连接顶替旧连接（断线重连）
		old.close("replaced by new connection")
		h.log.Info("realtime connection replaced", "session_id", sessionID)
	}

	defer func() {
		hub.clear(client)
		client.close("disconnected")
		<-client.closed // 等待 writePump 关闭底层连接
		h.hubs.release(sessionID, hub)
		h.log.Info("realtime connection closed", "session_id", sessionID)
	}()

	// 连接即推送全量快照：重连方据此恢复 UI，无需服务端事件持久化
	if !client.sendFrame(marshalEvent(typeSnapshot, sess)) {
		return
	}
	h.log.Info("realtime connection established", "session_id", sessionID, "user_id", userID)

	conn.SetReadLimit(readLimitBytes)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure, websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure, closeReplaced) {
				h.log.Warn("realtime read failed", "session_id", sessionID, "error", err)
			}
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))

		var env Envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			if !client.sendFrame(marshalEvent(typeError, ErrorData{Code: "BAD_REQUEST", Message: "消息格式必须为 JSON"})) {
				return
			}
			continue
		}

		switch env.Type {
		case typePing:
			if !client.sendFrame(marshalEvent(typePong, struct{}{})) {
				return
			}
		case typeAnswer:
			if !h.handleAnswer(ctx, client, sess, env.Data) {
				return
			}
		case typeChat:
			if !h.handleChat(ctx, client, sess, env.Data) {
				return
			}
		default:
			if !client.sendFrame(marshalEvent(typeError, ErrorData{Code: "UNKNOWN_MESSAGE_TYPE", Message: "未知消息类型: " + env.Type})) {
				return
			}
		}
	}
}

// handleAnswer 处理结构化回答，返回 false 表示连接已失效需退出
func (h *Handler) handleAnswer(ctx context.Context, c *Client, sess *interview.Session, raw json.RawMessage) bool {
	var msg AnswerMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return c.sendFrame(marshalEvent(typeError, ErrorData{Code: "BAD_REQUEST", Message: "answer 消息格式错误"}))
	}

	res, err := h.svc.SubmitAnswer(ctx, c.userID, sess.ID, interview.SubmitAnswerRequest{
		QuestionID:  msg.QuestionID,
		TextContent: msg.TextContent,
		DurationMs:  msg.DurationMs,
	})
	if err != nil {
		h.log.Warn("realtime submit answer failed",
			"session_id", sess.ID, "question_id", msg.QuestionID, "error", err)
		return c.sendFrame(marshalEvent(typeError, toErrorData(err)))
	}

	// 分阶段推送：回答落库 → 分析完成 → 追问产生
	if !c.sendFrame(marshalEvent(typeAnswerSaved, res.Answer)) {
		return false
	}
	if res.Analysis != nil {
		if !c.sendFrame(marshalEvent(typeAnalysis, res.Analysis)) {
			return false
		}
	}
	if res.FollowUp != nil {
		if !c.sendFrame(marshalEvent(typeFollowUp, res.FollowUp)) {
			return false
		}
	}
	h.log.Info("realtime answer processed",
		"session_id", sess.ID, "question_id", msg.QuestionID,
		"answer_chars", utf8.RuneCountInString(msg.TextContent),
		"has_analysis", res.Analysis != nil, "has_follow_up", res.FollowUp != nil)
	return true
}

// handleChat 处理实时面试官对话，token 流式推送，返回 false 表示连接已失效
func (h *Handler) handleChat(ctx context.Context, c *Client, sess *interview.Session, raw json.RawMessage) bool {
	var msg ChatMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return c.sendFrame(marshalEvent(typeError, ErrorData{Code: "BAD_REQUEST", Message: "chat 消息格式错误"}))
	}
	if sess.Status != interview.StatusRunning {
		return c.sendFrame(marshalEvent(typeError, ErrorData{
			Code:    "SESSION_NOT_RUNNING",
			Message: "只有进行中的面试可以实时对话",
		}))
	}
	msgLen := utf8.RuneCountInString(msg.Message)
	if strings.TrimSpace(msg.Message) == "" {
		return c.sendFrame(marshalEvent(typeError, ErrorData{Code: "EMPTY_MESSAGE", Message: "对话内容不能为空"}))
	}
	if msgLen > maxChatMessageRunes {
		return c.sendFrame(marshalEvent(typeError, ErrorData{Code: "MESSAGE_TOO_LONG", Message: "对话内容超过长度限制"}))
	}

	history := make([]llm.Message, 0, len(msg.History))
	for _, t := range msg.History {
		history = append(history, llm.Message{Role: t.Role, Content: t.Content})
	}

	events, err := h.agent.ChatInterviewer(ctx, agent.RealtimeChatInput{
		JobTitle:      sess.JobTitle,
		InterviewType: sess.InterviewType,
		History:       history,
		Message:       msg.Message,
	})
	if err != nil {
		h.log.Warn("realtime chat start failed", "session_id", sess.ID, "error", err)
		return c.sendFrame(marshalEvent(typeError, toErrorData(err)))
	}

	// 流式期间通过连接关闭事件取消 LLM 上下文（客户端断开 / 写失败 / 顶替）
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-c.closed:
			cancel()
		case <-streamCtx.Done():
		}
	}()

	var replyBuilder strings.Builder
	for ev := range events {
		if ev.Err != nil {
			h.log.Warn("realtime chat stream failed", "session_id", sess.ID, "error", ev.Err)
			return c.sendFrame(marshalEvent(typeError, ErrorData{Code: "LLM_STREAM_ERROR", Message: "实时对话生成失败"}))
		}
		replyBuilder.WriteString(ev.Delta)
		if !c.sendFrame(marshalEvent(typeChatDelta, ChatDeltaData{Delta: ev.Delta})) {
			cancel()
			return false
		}
	}

	// 请求语音：合成整段回复（合成失败不影响文字结果，发 error 事件后仍正常结束）
	// chat_speech 在 chat_done 之前下发，客户端以 chat_done 作为本轮终结信号
	if msg.TTS {
		if !h.sendChatSpeech(ctx, c, sess, strings.TrimSpace(replyBuilder.String())) {
			return false
		}
	}

	if !c.sendFrame(marshalEvent(typeChatDone, struct{}{})) {
		return false
	}
	h.log.Info("realtime chat streamed", "session_id", sess.ID,
		"input_chars", msgLen, "tts_requested", msg.TTS)
	return true
}

// sendChatSpeech 合成并推送对话回复语音，返回 false 表示连接已失效
func (h *Handler) sendChatSpeech(ctx context.Context, c *Client, sess *interview.Session, replyText string) bool {
	if h.speech == nil {
		return c.sendFrame(marshalEvent(typeError, ErrorData{Code: "TTS_UNAVAILABLE", Message: "服务未启用语音合成"}))
	}
	if replyText == "" {
		return c.sendFrame(marshalEvent(typeError, ErrorData{Code: "TTS_EMPTY_REPLY", Message: "回复为空，无法合成语音"}))
	}

	res, err := h.speech.SynthesizeForSession(ctx, c.userID, sess.ID, replyText)
	if err != nil {
		h.log.Warn("realtime chat tts failed", "session_id", sess.ID, "error", err)
		return c.sendFrame(marshalEvent(typeError, toErrorData(err)))
	}
	return c.sendFrame(marshalEvent(typeChatSpeech, ChatSpeechData{
		DownloadURL:           res.DownloadURL,
		Format:                res.Format,
		ContentType:           res.ContentType,
		SizeBytes:             res.SizeBytes,
		Cached:                res.Cached,
		DownloadExpireSeconds: res.DownloadExpireSeconds,
	}))
}

// toErrorData 将业务错误映射为 WS 错误事件数据
func toErrorData(err error) ErrorData {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ErrorData{Code: ae.Code, Message: ae.Message}
	}
	return ErrorData{Code: "INTERNAL_ERROR", Message: "服务器内部错误"}
}

// writeHTTPError 升级前的错误按 HTTP 状态码返回
func writeHTTPError(w http.ResponseWriter, err error) {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		http.Error(w, ae.Message, ae.Status)
		return
	}
	http.Error(w, "服务器内部错误", http.StatusInternalServerError)
}
