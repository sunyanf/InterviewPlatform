package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"ai-interview-platform/pkg/llm"
)

// Agent AI 编排器：负责 Prompt 管理、LLM 调用、结构化输出校验。
// Agent 只产出内容（问题、分析、建议），不决定任何业务状态（见 ADR-0004）。
type Agent struct {
	llm llm.Provider
	log *slog.Logger
}

// New 创建 Agent
func New(llmProv llm.Provider, log *slog.Logger) *Agent {
	return &Agent{llm: llmProv, log: log}
}

// chat 调用 LLM 并清理输出
func (a *Agent) chat(ctx context.Context, system, user string, temperature float64) (string, error) {
	resp, err := a.llm.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: temperature,
	})
	if err != nil {
		return "", fmt.Errorf("llm chat: %w", err)
	}
	return cleanJSON(resp.Content), nil
}

// cleanJSON 清理 LLM 输出中的 markdown 代码块标记
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// unmarshal 解析 JSON 到目标结构（Schema 校验入口）
func unmarshal(content string, target any) error {
	if err := json.Unmarshal([]byte(content), target); err != nil {
		return fmt.Errorf("invalid llm json output: %w", err)
	}
	return nil
}

// PlannedQuestion 规划出的问题（AI 数据 Contract，见 docs/AI_DATA_CONTRACTS.md）
type PlannedQuestion struct {
	Question       string   `json:"question"`
	Type           string   `json:"type"`
	Difficulty     string   `json:"difficulty"`
	ExpectedPoints []string `json:"expected_points"`
}

// PlanQuestionsInput 出题规划输入
type PlanQuestionsInput struct {
	JobTitle        string
	JobSkills       []string
	JobRequirements []string
	ResumeSkills    []string
	InterviewType   string
	Count           int
}

// AnswerAnalysis 回答分析结果（AI 数据 Contract）
type AnswerAnalysis struct {
	Claims        []string `json:"claims"`
	CorrectPoints []string `json:"correct_points"`
	WrongPoints   []string `json:"wrong_points"`
	MissingPoints []string `json:"missing_points"`
	KnowledgeGaps []string `json:"knowledge_gaps"`
	PromptVersion string   `json:"prompt_version"`
	Model         string   `json:"model"`
}

// AnalyzeAnswerInput 回答分析输入
type AnalyzeAnswerInput struct {
	Question       string
	ExpectedPoints []string
	AnswerText     string
	Difficulty     string
}

// FollowUpDecision 追问决策（AI 数据 Contract）
type FollowUpDecision struct {
	ShouldFollowUp bool   `json:"should_follow_up"`
	Reason         string `json:"reason"`
	Question       string `json:"question"`
	TargetGap      string `json:"target_gap"`
}

// FollowUpInput 追问决策输入
type FollowUpInput struct {
	Question       string
	ExpectedPoints []string
	AnswerText     string
	Analysis       *AnswerAnalysis
}

// OpeningInput 开场白输入
type OpeningInput struct {
	JobTitle      string
	InterviewType string
	FirstQuestion string
}

// 合法枚举
var validQuestionTypes = map[string]bool{"technical": true, "behavioral": true, "project": true, "follow_up": true}
var validDifficulties = map[string]bool{"easy": true, "medium": true, "hard": true}
