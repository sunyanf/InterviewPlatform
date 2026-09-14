package report

import (
	"math"
	"strings"

	"ai-interview-platform/internal/evaluation"
)

// historyWindow 历史对比取最近 N 场
const historyWindow = 5

// knowledgeGapLimit 知识缺口聚合上限（避免报告无限膨胀）
const knowledgeGapLimit = 10

// round2 四舍五入保留 2 位
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildHistoryComparison 历史对比纯函数：本次评估 vs 历史评估均值（确定性）
// histories 为按时间倒序的历史评估；无历史时返回 nil（首次面试）
func BuildHistoryComparison(current evaluation.Evaluation, histories []evaluation.Evaluation) *HistoryComparison {
	if len(histories) == 0 {
		return nil
	}

	count := 0
	sumTotal := 0.0
	sumDims := map[string]float64{}

	for _, h := range histories {
		count++
		sumTotal += h.TotalScore
		for k, v := range h.Dimensions {
			sumDims[k] += v
		}
	}

	avgTotal := round2(sumTotal / float64(count))
	avgDims := make(map[string]float64, len(sumDims))
	deltaDims := make(map[string]float64, len(sumDims))
	for k, s := range sumDims {
		avg := round2(s / float64(count))
		avgDims[k] = avg
		if cur, ok := current.Dimensions[k]; ok {
			deltaDims[k] = round2(cur - avg)
		}
	}

	return &HistoryComparison{
		ComparedCount:   count,
		AvgTotalScore:   avgTotal,
		AvgDimensions:   avgDims,
		DeltaTotalScore: round2(current.TotalScore - avgTotal),
		DeltaDimensions: deltaDims,
	}
}

// AggregateKnowledgeGaps 知识缺口确定性聚合：从已存储的回答分析收集并去重（nil 安全）
// 按首次出现顺序保留，截断至 knowledgeGapLimit
func AggregateKnowledgeGaps(analyses []*analysisView) []string {
	seen := map[string]bool{}
	var gaps []string
	for _, a := range analyses {
		if a == nil {
			continue
		}
		for _, g := range a.KnowledgeGaps {
			g = strings.TrimSpace(g)
			if g == "" || seen[g] {
				continue
			}
			seen[g] = true
			gaps = append(gaps, g)
			if len(gaps) >= knowledgeGapLimit {
				return gaps
			}
		}
	}
	return gaps
}

// analysisView 回答分析的轻量视图（供聚合使用，避免依赖 interview 内部类型）
type analysisView struct {
	KnowledgeGaps []string
}
