package interview

import (
	"time"

	"ai-interview-platform/internal/agent"
)

// 会话状态常量（状态只能由业务代码控制，见 ADR-0004）
const (
	StatusInit       = "INIT"
	StatusReady      = "READY"
	StatusRunning    = "RUNNING"
	StatusPaused     = "PAUSED"
	StatusFinishing  = "FINISHING"
	StatusEvaluating = "EVALUATING"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
)

// QuestionType 问题类型
const (
	QTypeTechnical  = "technical"
	QTypeBehavioral = "behavioral"
	QTypeProject    = "project"
	QTypeFollowUp   = "follow_up"
)

// 答题链路中各 LLM 步骤的超时上限：超时即降级（回答已保存，不阻塞候选人继续作答）
const (
	analyzeAnswerTimeout  = 30 * time.Second
	decideFollowUpTimeout = 20 * time.Second
)

// SessionConfig 面试配置
type SessionConfig struct {
	QuestionCount   int `json:"question_count"`
	DurationMinutes int `json:"duration_minutes"`
}

// Session 面试会话
type Session struct {
	ID            string        `json:"id"`
	UserID        string        `json:"user_id"`
	JobID         string        `json:"job_id"`
	ResumeID      string        `json:"resume_id,omitempty"`
	InterviewType string        `json:"interview_type"`
	Mode          string        `json:"mode"`
	Status        string        `json:"status"`
	Config        SessionConfig `json:"config"`
	StartedAt     *time.Time    `json:"started_at,omitempty"`
	EndedAt       *time.Time    `json:"ended_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`

	// 关联数据（查询详情时填充）
	JobTitle      string                 `json:"job_title,omitempty"`
	Questions     []Question             `json:"questions,omitempty"`
	AnsweredCount int                    `json:"answered_count"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Question 面试问题
type Question struct {
	ID             string                 `json:"id"`
	SessionID      string                 `json:"session_id"`
	Seq            int                    `json:"seq"`
	QuestionType   string                 `json:"question_type"`
	Question       string                 `json:"question"`
	Difficulty     string                 `json:"difficulty"`
	Source         string                 `json:"source"`
	ExpectedPoints []string               `json:"expected_points"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`

	// 回答状态（查询详情时填充）
	Answered bool    `json:"answered"`
	AnswerID string  `json:"answer_id,omitempty"`
	Answer   *Answer `json:"answer,omitempty"`
}

// Answer 面试回答
type Answer struct {
	ID          string                `json:"id"`
	SessionID   string                `json:"session_id"`
	QuestionID  string                `json:"question_id"`
	InputType   string                `json:"input_type"`
	TextContent string                `json:"text_content"`
	DurationMs  int                   `json:"duration_ms"`
	Analysis    *agent.AnswerAnalysis `json:"analysis,omitempty"`
	CreatedAt   time.Time             `json:"created_at"`
}

// CreateSessionRequest 创建面试请求
type CreateSessionRequest struct {
	JobID         string `json:"job_id"`
	ResumeID      string `json:"resume_id"`
	InterviewType string `json:"interview_type"`
	Mode          string `json:"mode"`
	QuestionCount int    `json:"question_count"`
	DurationMins  int    `json:"duration_minutes"`
}

// SubmitAnswerRequest 提交回答请求
type SubmitAnswerRequest struct {
	QuestionID  string `json:"question_id"`
	TextContent string `json:"text_content"`
	DurationMs  int    `json:"duration_ms"`
}

// SubmitAnswerResult 提交回答结果（含 AI 分析与追问）
type SubmitAnswerResult struct {
	Answer   *Answer               `json:"answer"`
	Analysis *agent.AnswerAnalysis `json:"analysis,omitempty"`
	FollowUp *Question             `json:"follow_up,omitempty"`
}

// 合法的面试类型和模式
var validInterviewTypes = map[string]bool{
	"technical":  true,
	"behavioral": true,
	"mixed":      true,
}

var validModes = map[string]bool{
	"text":  true,
	"voice": true,
}
