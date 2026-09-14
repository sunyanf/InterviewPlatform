package evaluation

import (
	"time"

	"ai-interview-platform/internal/agent"
)

// Evaluation 评估结果（一场面试一份，重跑覆盖）
type Evaluation struct {
	ID              string               `json:"id"`
	SessionID       string               `json:"session_id"`
	UserID          string               `json:"user_id"`
	Dimensions      map[string]float64   `json:"dimensions"`
	Evidence        []agent.EvidenceItem `json:"evidence"`
	Recommendations []string             `json:"recommendations"`
	Rubric          Rubric               `json:"rubric"`
	TotalScore      float64              `json:"total_score"`
	PromptVersion   string               `json:"prompt_version"`
	Model           string               `json:"model"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}
