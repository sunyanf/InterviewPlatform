package realtime

import "encoding/json"

// 消息类型（C→S 与 S→C 共用 Envelope 信封）
const (
	// C→S
	typeAnswer = "answer" // 提交结构化回答
	typeChat   = "chat"   // 实时面试官对话（流式回复）
	typePing   = "ping"   // 客户端心跳

	// S→C
	typeSnapshot        = "snapshot"         // 连接/重连全量状态
	typeAnswerSaved     = "answer_saved"     // 回答已持久化
	typeAnalysis        = "analysis"         // 回答分析完成
	typeAnalysisSkipped = "analysis_skipped" // 分析服务超时/失败，回答已记录但本轮无分析
	typeFollowUp        = "follow_up"        // 产生追问问题
	typeChatDelta       = "chat_delta"       // 实时对话增量
	typeChatDone        = "chat_done"        // 实时对话结束
	typeChatSpeech      = "chat_speech"      // 实时对话回复的语音合成结果（请求 tts=true 时在 chat_done 前下发）
	typePong            = "pong"
	typeError           = "error"
)

// Envelope WebSocket 消息信封
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// AnswerMessage C→S 提交回答
type AnswerMessage struct {
	QuestionID  string `json:"question_id"`
	TextContent string `json:"text_content"`
	DurationMs  int    `json:"duration_ms"`
}

// ChatTurn 连接内的一轮历史对话
type ChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatMessage C→S 实时对话
type ChatMessage struct {
	Message string     `json:"message"`
	History []ChatTurn `json:"history,omitempty"`
	// TTS 为 true 时，回复结束后额外下发一条 chat_speech 事件（合成语音下载 URL）
	TTS bool `json:"tts,omitempty"`
}

// ErrorData S→C 错误事件数据
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AnalysisSkippedData S→C 分析降级（回答已保存，但分析超时/失败）
type AnalysisSkippedData struct {
	AnswerID   string `json:"answer_id"`
	QuestionID string `json:"question_id"`
}

// ChatDeltaData S→C 对话增量
type ChatDeltaData struct {
	Delta string `json:"delta"`
}

// ChatSpeechData S→C 对话回复语音合成结果
type ChatSpeechData struct {
	DownloadURL           string `json:"download_url"`
	Format                string `json:"format"`
	ContentType           string `json:"content_type"`
	SizeBytes             int64  `json:"size_bytes"`
	Cached                bool   `json:"cached"`
	DownloadExpireSeconds int    `json:"download_expire_seconds"`
}

// marshalEvent 将类型与数据组装为 JSON 帧（数据序列化失败时返回错误帧）
func marshalEvent(typ string, data any) []byte {
	raw, err := json.Marshal(data)
	if err != nil {
		raw, _ = json.Marshal(ErrorData{Code: "INTERNAL_ERROR", Message: "事件序列化失败"})
		typ = typeError
	}
	env := Envelope{Type: typ, Data: raw}
	b, err := json.Marshal(env)
	if err != nil {
		b = []byte(`{"type":"error","data":{"code":"INTERNAL_ERROR","message":"事件序列化失败"}}`)
	}
	return b
}
