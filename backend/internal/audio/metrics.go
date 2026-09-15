package audio

import (
	"fmt"
	"strings"
	"unicode"
)

// 允许的音频格式（扩展名白名单）
var allowedFormats = map[string]string{
	"wav":  "audio/wav",
	"mp3":  "audio/mpeg",
	"m4a":  "audio/mp4",
	"webm": "audio/webm",
	"ogg":  "audio/ogg",
}

// ContentType 返回格式对应的 MIME 类型
func ContentType(format string) string {
	if ct, ok := allowedFormats[format]; ok {
		return ct
	}
	return "application/octet-stream"
}

// DetectFormat 从文件名扩展名检测格式（白名单校验）
func DetectFormat(filename string) (string, error) {
	dot := strings.LastIndex(filename, ".")
	if dot < 0 || dot == len(filename)-1 {
		return "", fmt.Errorf("missing file extension")
	}
	ext := strings.ToLower(filename[dot+1:])
	if _, ok := allowedFormats[ext]; !ok {
		return "", fmt.Errorf("unsupported audio format: %s", ext)
	}
	return ext, nil
}

// fillerWords 口头禅词表（MVP：常见中英文填充词，字符串计数）
var fillerWords = []string{"嗯", "呃", "那个", "就是说", "然后", "em", "um", "uh", "like"}

// 语速评估阈值（有效字符/分钟）
const (
	paceSlowThreshold  = 120.0
	paceFastThreshold  = 280.0
	paceNormalMinCount = 10 // 有效字符数过低时不评估语速
)

// ComputeSpeechMetrics 语音量化指标纯函数（确定性，AGENTS.md #9）
// transcript 为 ASR 转写文本；durationMs 为音频时长（客户端上报或 Provider 返回）
func ComputeSpeechMetrics(transcript string, durationMs int64) SpeechMetrics {
	chars := effectiveChars(transcript)
	minutes := float64(durationMs) / 60000.0

	metrics := SpeechMetrics{
		FillerDetail: countFillers(transcript),
	}
	for _, n := range metrics.FillerDetail {
		metrics.FillerCount += n
	}

	if minutes <= 0 || chars < paceNormalMinCount {
		metrics.Pace = "unknown"
		return metrics
	}

	metrics.CharsPerMinute = round2(float64(chars) / minutes)
	switch {
	case metrics.CharsPerMinute < paceSlowThreshold:
		metrics.Pace = "slow"
	case metrics.CharsPerMinute > paceFastThreshold:
		metrics.Pace = "fast"
	default:
		metrics.Pace = "normal"
	}
	return metrics
}

// effectiveChars 有效字符数：仅统计字母/数字/CJK 等文字字符（排除空白与标点）
func effectiveChars(s string) int {
	count := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			count++
		}
	}
	return count
}

// countFillers 口头禅计数（大小写不敏感，字符串包含计数）
func countFillers(text string) map[string]int {
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	detail := make(map[string]int)
	for _, w := range fillerWords {
		n := strings.Count(lower, strings.ToLower(w))
		if n > 0 {
			detail[w] = n
		}
	}
	if len(detail) == 0 {
		return nil
	}
	return detail
}

// round2 四舍五入保留 2 位
func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
