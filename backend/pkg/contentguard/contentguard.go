// Package contentguard 提供本地规则式内容安全审核（Plan A）。
//
// 定位：零外部依赖、零按量成本的基线防线，为「双向审核」提供统一判定——
// 输入侧（简历、岗位描述、候选人回答、实时对话）与输出侧（报告、讲解、
// 实时对话回复）共用同一套规则；命中即拒绝（输入侧）或替换（输出侧），
// 并留审计记录。
//
// 边界与已知不足（调用方必须知晓，禁止对外宣称为完整内容安全能力）：
//  1. 内置词表只是「基线示例」，只覆盖明确无争议的违禁/违规类别，不是权威词表。
//     生产环境的完整词表必须来自可追踪来源（AGENTS.md #14），或按 Plan B 接入
//     第三方内容审核 API。基线词表变更必须记录来源与生效时间。
//  2. 词表为子串匹配，存在误报可能；输入侧拒绝时必须给出可申诉/可修改的提示。
//  3. 结构规则用于拦截「绕过型文本」（零宽字符、双向控制字符、异常重复字符）：
//     先归一化再匹配，使「代\u200B考」这类混淆写法无法绕过词表。
package contentguard

import (
	"fmt"
	"strings"
	"unicode"
)

// 违规分类
const (
	CategoryIllegal     = "illegal"     // 违禁品、违法服务（代考/办证/博彩等）
	CategoryPorn        = "porn"        // 色情低俗
	CategoryViolence    = "violence"    // 暴力恐怖
	CategoryAbuse       = "abuse"       // 极端辱骂与威胁
	CategoryObstruction = "obstruction" // 绕过型文本（零宽/双向控制字符、异常重复）
	CategoryOversize    = "oversize"    // 超长文本
)

// DefaultMaxRunes 单条自由文本的默认长度上限（按字符计，非字节）
const DefaultMaxRunes = 20000

// 结构规则阈值
const (
	// zeroWidthBlockThreshold 零宽字符达到该数量即判定刻意混淆；少量视为复制粘贴残留
	zeroWidthBlockThreshold = 5
	// repeatedRuneThreshold 同一字符连续重复达到该数量即判定灌水/绕过
	repeatedRuneThreshold = 30
)

// Verdict 审核结论。Reason 只描述规则与类别，不含命中原文（避免审计日志泄露用户内容）
type Verdict struct {
	Allowed  bool   `json:"allowed"`
	Category string `json:"category,omitempty"`
	RuleID   string `json:"rule_id,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// Allowed 通过审核的结论
func Allowed() Verdict { return Verdict{Allowed: true} }

// Check 审核单条自由文本（使用默认长度上限）
func Check(text string) Verdict { return CheckLimited(text, DefaultMaxRunes) }

// CheckLimited 审核单条自由文本，可自定义长度上限（简历等长文本可放宽）
func CheckLimited(text string, maxRunes int) Verdict {
	// 先做结构检查（基于原文，识别不可见字符与异常重复）
	if v := checkStructure(text); !v.Allowed {
		return v
	}

	// 归一化后按长度与词表判定，避免混淆写法绕过
	cleaned, _ := Sanitize(text)
	if maxRunes > 0 && len([]rune(cleaned)) > maxRunes {
		return Verdict{
			Allowed:  false,
			Category: CategoryOversize,
			RuleID:   "structure.oversize",
			Reason:   fmt.Sprintf("文本长度超过上限（%d 字符）", maxRunes),
		}
	}
	return checkWords(cleaned)
}

// CheckList 审核多条文本（如报告的建议数组），任一命中即返回该条结论
func CheckList(items []string) Verdict {
	for _, item := range items {
		if v := Check(item); !v.Allowed {
			return v
		}
	}
	return Allowed()
}

// CategoryLabel 违规类别的中文标签，用于用户提示与审计日志
func CategoryLabel(category string) string {
	switch category {
	case CategoryIllegal:
		return "违法违规"
	case CategoryPorn:
		return "色情低俗"
	case CategoryViolence:
		return "暴力恐怖"
	case CategoryAbuse:
		return "辱骂攻击"
	case CategoryObstruction:
		return "异常文本"
	case CategoryOversize:
		return "文本过长"
	default:
		return "内容不合规"
	}
}

// Sanitize 移除绕过型与不可见字符，返回清洗后的文本与被移除规则 ID 列表。
// 用于输入归一化；是否因这些字符拒绝由 Check 的结构规则决定。
func Sanitize(text string) (string, []string) {
	if text == "" {
		return text, nil
	}
	var b strings.Builder
	b.Grow(len(text))
	var sawBidi, sawZeroWidth, sawControl bool
	for _, r := range text {
		switch {
		case isBidiControl(r):
			sawBidi = true
			continue
		case isZeroWidth(r):
			sawZeroWidth = true
			continue
		case isControl(r):
			sawControl = true
			continue
		}
		b.WriteRune(r)
	}
	removed := make([]string, 0, 3)
	if sawBidi {
		removed = append(removed, "structure.bidi_control")
	}
	if sawZeroWidth {
		removed = append(removed, "structure.zero_width")
	}
	if sawControl {
		removed = append(removed, "structure.control_char")
	}
	return b.String(), removed
}

// checkStructure 结构规则：拦截绕过型文本
func checkStructure(text string) Verdict {
	if text == "" {
		return Allowed()
	}
	var zeroWidth int
	var prev rune
	run := 0
	for _, r := range text {
		switch {
		case isBidiControl(r):
			// 双向控制字符能改变显示顺序且无正常用途，出现即拦
			return Verdict{
				Allowed:  false,
				Category: CategoryObstruction,
				RuleID:   "structure.bidi_control",
				Reason:   "检测到双向文本控制字符（异常文本）",
			}
		case isZeroWidth(r):
			zeroWidth++
		}
		if r == prev {
			run++
			if run >= repeatedRuneThreshold && !unicode.IsSpace(r) {
				return Verdict{
					Allowed:  false,
					Category: CategoryObstruction,
					RuleID:   "structure.repeated_rune",
					Reason:   "检测到异常重复字符（异常文本）",
				}
			}
		} else {
			prev = r
			run = 1
		}
	}
	if zeroWidth >= zeroWidthBlockThreshold {
		return Verdict{
			Allowed:  false,
			Category: CategoryObstruction,
			RuleID:   "structure.zero_width",
			Reason:   "检测到大量零宽字符（异常文本）",
		}
	}
	return Allowed()
}

// checkWords 词表规则：按归一化后的文本做子串匹配
func checkWords(text string) Verdict {
	if strings.TrimSpace(text) == "" {
		return Allowed()
	}
	lower := strings.ToLower(text)
	for _, rule := range baselineRules {
		if strings.Contains(lower, rule.Pattern) {
			return Verdict{
				Allowed:  false,
				Category: rule.Category,
				RuleID:   rule.ID,
				Reason:   "命中内容安全规则（" + CategoryLabel(rule.Category) + "）",
			}
		}
	}
	return Allowed()
}

// isZeroWidth 零宽字符：不可见但参与匹配，常用于绕过词表
func isZeroWidth(r rune) bool {
	switch r {
	case '\u200B', '\u200C', '\u200D', '\u200E', '\u200F', '\uFEFF', '\u2060':
		return true
	}
	return false
}

// isBidiControl 双向文本控制字符：可改变显示顺序
func isBidiControl(r rune) bool {
	return (r >= '\u202A' && r <= '\u202E') || (r >= '\u2066' && r <= '\u2069')
}

// isControl C0/C1 控制字符（保留换行与制表符）
func isControl(r rune) bool {
	if r == '\n' || r == '\t' || r == '\r' {
		return false
	}
	return r < 0x20 || (r >= 0x7F && r <= 0x9F)
}
