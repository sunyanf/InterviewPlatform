package report

import (
	"time"

	"ai-interview-platform/internal/agent"
)

// Report 面试报告（一场面试一份，重跑覆盖）
type Report struct {
	ID                string            `json:"id"`
	SessionID         string            `json:"session_id"`
	UserID            string            `json:"user_id"`
	TotalScore        float64           `json:"total_score"`
	CapabilityProfile CapabilityProfile `json:"capability_profile"`
	Strengths         []string          `json:"strengths"`
	Weaknesses        []string          `json:"weaknesses"`
	KnowledgeGaps     []string          `json:"knowledge_gaps"`
	LearningPlan      LearningPlan      `json:"learning_plan"`
	PromptVersion     string            `json:"prompt_version"`
	Model             string            `json:"model"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// CapabilityProfile 能力画像（确定性：评估维度分 + 历史对比）
type CapabilityProfile struct {
	// Dimensions 本次评估维度分（快照）
	Dimensions map[string]float64 `json:"dimensions"`
	// TotalScore 本次总分（复制自评估，确定性）
	TotalScore float64 `json:"total_score"`
	// History 历史对比（首次面试为 nil）
	History *HistoryComparison `json:"history,omitempty"`
}

// HistoryComparison 历史对比（与用户同类型面试最近 N 场评估的平均值对比，确定性计算）
type HistoryComparison struct {
	// ComparedCount 参与对比的历史场次数
	ComparedCount int `json:"compared_count"`
	// AvgTotalScore 历史平均总分
	AvgTotalScore float64 `json:"avg_total_score"`
	// AvgDimensions 历史平均维度分
	AvgDimensions map[string]float64 `json:"avg_dimensions"`
	// DeltaTotalScore 本次总分 - 历史平均（正数=进步）
	DeltaTotalScore float64 `json:"delta_total_score"`
	// DeltaDimensions 各维度 delta（键与 Rubric 维度一致）
	DeltaDimensions map[string]float64 `json:"delta_dimensions"`
}

// LearningPlan 学习计划（LLM 生成，已归一化校验；优点/不足在报告顶层字段）
type LearningPlan struct {
	FocusAreas   []agent.FocusArea  `json:"focus_areas"`
	NextTraining agent.NextTraining `json:"next_training"`
}
