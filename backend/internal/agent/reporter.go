package agent

import (
	"context"
	"fmt"
	"strings"
)

// FocusArea 学习重点
type FocusArea struct {
	Topic       string   `json:"topic"`
	Reason      string   `json:"reason"`
	Suggestions []string `json:"suggestions"`
}

// NextTraining 下一次训练建议
type NextTraining struct {
	Focus                 string   `json:"focus"`
	SuggestedQuestionType string   `json:"suggested_question_type"`
	SuggestedDifficulty   string   `json:"suggested_difficulty"` // easy / medium / hard
	SuggestedTopics       []string `json:"suggested_topics"`
}

// LearningPlanOutput 学习计划输出（AI 数据 Contract）
// 注意：不含任何分数——分数由评估模块确定性计算，此处只产建议内容
type LearningPlanOutput struct {
	Strengths     []string     `json:"strengths"`
	Weaknesses    []string     `json:"weaknesses"`
	FocusAreas    []FocusArea  `json:"focus_areas"`
	NextTraining  NextTraining `json:"next_training"`
	PromptVersion string       `json:"prompt_version"`
	Model         string       `json:"model"`
}

// LearningPlannerInput 学习计划生成输入（全部来自已验证的评估结果）
type LearningPlannerInput struct {
	JobTitle      string
	InterviewType string
	Dimensions    map[string]float64
	// Evidence 评估证据（含不足项，用于归纳优点/不足）
	Evidence []EvidenceItem
	// KnowledgeGaps 确定性聚合的知识缺口
	KnowledgeGaps []string
	// EvalRecommendations 评估产出的改进建议
	EvalRecommendations []string
	// HistorySummary 历史对比摘要（确定性计算结果，如 "近 2 场平均总分 65.5，本次 70.75（+5.25）"）
	HistorySummary string
}

// LearningPlan 基于评估结果生成学习计划（LLM 只归纳与建议，不算分）
func (a *Agent) LearningPlan(ctx context.Context, in LearningPlannerInput) (*LearningPlanOutput, error) {
	if len(in.Dimensions) == 0 {
		return nil, fmt.Errorf("learning plan requires evaluation dimensions")
	}

	// 证据摘要
	var evidence strings.Builder
	for _, e := range in.Evidence {
		evidence.WriteString(fmt.Sprintf("[题 %d] %s（证据：%s）\n", e.QuestionSeq, e.Issue, e.Evidence))
	}

	// 评估建议
	recs := joinOrNone(in.EvalRecommendations)

	// 知识缺口
	gaps := joinOrNone(in.KnowledgeGaps)

	// 历史对比摘要（可选）
	historySection := ""
	if strings.TrimSpace(in.HistorySummary) != "" {
		historySection = fmt.Sprintf("\n历史对比：%s", in.HistorySummary)
	}

	prompt := fmt.Sprintf(`请基于以下评估结果生成学习计划。请先归纳本场面试的优点与不足，再给出学习重点与下一次训练建议。

岗位：%s
面试类型：%s
维度评分（0-100）：%s
评估证据：
%s
评估建议：%s
知识缺口：%s
%s
要求：
1. 只返回 JSON，不要包含任何解释文字
2. JSON 结构：{"strengths":[""],"weaknesses":[""],"focus_areas":[{"topic":"","reason":"","suggestions":[""]}],"next_training":{"focus":"","suggested_question_type":"","suggested_difficulty":"medium","suggested_topics":[""]}}
3. strengths 和 weaknesses 各 1-4 条，必须基于评估证据，不得臆测
4. focus_areas 给 1-3 个学习重点，reason 说明与哪个维度/知识缺口相关
5. suggested_difficulty 只能是 easy、medium、hard 之一，根据维度评分水平选择
6. 建议必须具体可执行，与知识缺口和不足对应`,
		in.JobTitle, in.InterviewType, formatDimensions(in.Dimensions),
		evidence.String(), recs, gaps, historySection)

	content, err := a.chat(ctx, buildSystemPrompt("你是一个专业的技术学习规划师，只基于已有评估结果给出建议，不重新评分", PromptLearningPlanner), prompt, 0.4)
	if err != nil {
		return nil, err
	}

	var out LearningPlanOutput
	if err := unmarshal(content, &out); err != nil {
		return nil, err
	}
	normalizeLearningPlan(&out)
	if err := validateLearningPlan(&out); err != nil {
		return nil, err
	}

	out.PromptVersion = PromptLearningPlanner
	out.Model = a.llm.Name()
	return &out, nil
}

// normalizeLearningPlan 学习计划输出业务归一化（AGENTS.md #11）
func normalizeLearningPlan(out *LearningPlanOutput) {
	// 字符串数组清洗
	out.Strengths = cleanStringList(out.Strengths)
	out.Weaknesses = cleanStringList(out.Weaknesses)

	// focus_areas 清洗
	validAreas := out.FocusAreas[:0]
	for _, fa := range out.FocusAreas {
		if strings.TrimSpace(fa.Topic) == "" {
			continue
		}
		fa.Topic = strings.TrimSpace(fa.Topic)
		fa.Reason = strings.TrimSpace(fa.Reason)
		fa.Suggestions = cleanStringList(fa.Suggestions)
		validAreas = append(validAreas, fa)
	}
	out.FocusAreas = validAreas

	// next_training：难度归一化（非法值默认 medium）
	if !validDifficulties[out.NextTraining.SuggestedDifficulty] {
		out.NextTraining.SuggestedDifficulty = "medium"
	}
	out.NextTraining.Focus = strings.TrimSpace(out.NextTraining.Focus)
	out.NextTraining.SuggestedQuestionType = strings.ToLower(strings.TrimSpace(out.NextTraining.SuggestedQuestionType))
	out.NextTraining.SuggestedTopics = cleanStringList(out.NextTraining.SuggestedTopics)
}

// validateLearningPlan 学习计划业务校验：优点/不足/学习重点至少各 1 条（缺失视为无效，可重跑）
func validateLearningPlan(out *LearningPlanOutput) error {
	if len(out.Strengths) == 0 {
		return fmt.Errorf("learning plan output missing strengths")
	}
	if len(out.Weaknesses) == 0 {
		return fmt.Errorf("learning plan output missing weaknesses")
	}
	if len(out.FocusAreas) == 0 {
		return fmt.Errorf("learning plan output missing focus_areas")
	}
	return nil
}

// cleanStringList 清洗字符串列表：trim、去空
func cleanStringList(items []string) []string {
	out := items[:0]
	for _, s := range items {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// formatDimensions 维度分格式化（确定性排序便于测试与 prompt 稳定）
func formatDimensions(dims map[string]float64) string {
	order := []string{DimCorrectness, DimDepth, DimLogic, DimCommunication}
	var parts []string
	for _, k := range order {
		if v, ok := dims[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%.0f", k, v))
		}
	}
	// 兜底：非标准维度（理论上不会出现，白名单已在评估层过滤）
	for k, v := range dims {
		if k != DimCorrectness && k != DimDepth && k != DimLogic && k != DimCommunication {
			parts = append(parts, fmt.Sprintf("%s=%.0f", k, v))
		}
	}
	return strings.Join(parts, ", ")
}
