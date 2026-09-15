package task

import (
	"encoding/json"
	"time"
)

// 任务状态机：pending → running → succeeded；失败按退避回到 pending，超过上限 → failed
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// 任务类型（业务模块入队与 worker 注册表共用）
const (
	TypeResumeParse       = "resume_parse"
	TypeSessionEvaluation = "session_evaluation"
	TypeReportGeneration  = "report_generation"
)

// Task 异步任务记录。LLM 不拥有任务状态：状态流转只由 worker 业务代码写入。
type Task struct {
	ID             string
	Type           string
	Payload        json.RawMessage
	Status         string
	Priority       int
	Attempts       int
	MaxAttempts    int
	RunAfter       time.Time
	LockedBy       string
	LockedAt       *time.Time
	LastError      string
	IdempotencyKey string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EnqueueRequest 入队请求
type EnqueueRequest struct {
	Type           string
	Payload        any    // 序列化为 JSONB；必须包含 user_id 以支持任务鉴权
	MaxAttempts    int    // <=0 时使用默认 3
	IdempotencyKey string // 空串表示不做幂等去重
	Priority       int
}

// ResumeParsePayload resume_parse 任务负载
type ResumeParsePayload struct {
	UserID   string `json:"user_id"`
	ResumeID string `json:"resume_id"`
}

// SessionTaskPayload session_evaluation / report_generation 任务负载
type SessionTaskPayload struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}
