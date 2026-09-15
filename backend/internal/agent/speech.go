package agent

import (
	"context"
	"fmt"
	"sort"
)

// SpeechAnalysis 语音表达分析输出（AI 数据 Contract）
// 只产定性建议，不计算分数——语速/口头禅等量化指标由业务代码确定性计算
type SpeechAnalysis struct {
	Strengths     []string `json:"strengths"`
	Issues        []string `json:"issues"`
	Suggestions   []string `json:"suggestions"`
	PromptVersion string   `json:"prompt_version"`
	Model         string   `json:"model"`
}

// SpeechMetricsInput 语音指标（确定性计算结果，供 LLM 参考）
type SpeechMetricsInput struct {
	// DurationSec 音频时长（秒）
	DurationSec float64
	// CharsPerMinute 语速（有效字符/分钟）
	CharsPerMinute float64
	// Pace 语速评估（slow / normal / fast / unknown）
	Pace string
	// FillerCount 口头禅总次数
	FillerCount int
	// FillerDetail 各口头禅出现次数（如 {"嗯":3,"那个":2}）
	FillerDetail map[string]int
}

// AnalyzeSpeech 语音表达分析（LLM 只产建议内容）
func (a *Agent) AnalyzeSpeech(ctx context.Context, question, transcript string, m SpeechMetricsInput) (*SpeechAnalysis, error) {
	if transcript == "" {
		return nil, fmt.Errorf("speech analysis requires transcript")
	}

	prompt := fmt.Sprintf(`请基于以下语音回答的转写文本与量化指标，分析候选人的口头表达能力。

问题：%s
转写文本：%s
音频时长：%.1f 秒
语速：%.0f 字/分钟（评估：%s）
口头禅：共 %d 次%s

要求：
1. 只返回 JSON，不要包含任何解释文字
2. JSON 结构：{"strengths":[""],"issues":[""],"suggestions":[""]}
3. strengths 和 issues 各 0-3 条，必须基于转写文本与指标事实
4. suggestions 给 1-4 条具体可执行的口头表达改进建议（如结构化表达、控制语速、减少口头禅）
5. 不得臆测转写文本以外的内容`,
		question, transcript, m.DurationSec, m.CharsPerMinute, m.Pace, m.FillerCount, formatFillerDetail(m.FillerDetail))

	content, err := a.chat(ctx, buildSystemPrompt("你是一个专业的口头表达教练，只基于转写文本与量化指标给出建议，不计算分数", PromptSpeechAnalyzer), prompt, 0.4)
	if err != nil {
		return nil, err
	}

	var out SpeechAnalysis
	if err := unmarshal(content, &out); err != nil {
		return nil, err
	}
	// 归一化：清洗空项
	out.Strengths = cleanStringList(out.Strengths)
	out.Issues = cleanStringList(out.Issues)
	out.Suggestions = cleanStringList(out.Suggestions)
	if len(out.Suggestions) == 0 {
		return nil, fmt.Errorf("speech analysis output missing suggestions")
	}

	out.PromptVersion = PromptSpeechAnalyzer
	out.Model = a.llm.Name()
	return &out, nil
}

// formatFillerDetail 口头禅明细格式化（按字典序稳定输出，供 prompt 稳定与测试）
func formatFillerDetail(detail map[string]int) string {
	if len(detail) == 0 {
		return ""
	}
	keys := make([]string, 0, len(detail))
	for k := range detail {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	s := ": "
	for i, k := range keys {
		if i > 0 {
			s += "、"
		}
		s += fmt.Sprintf("%s×%d", k, detail[k])
	}
	return s
}
