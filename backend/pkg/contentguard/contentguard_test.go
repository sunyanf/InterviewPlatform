package contentguard

import (
	"strings"
	"testing"
)

func TestCheck_AllowsNormalText(t *testing.T) {
	texts := []string{
		"张三，3 年 Go 后端经验，负责支付网关开发，熟悉 PostgreSQL 与 Redis。",
		"请介绍一下你在高并发场景下的限流方案。",
		"我负责过订单系统的重构，把平均响应时间从 800ms 降到 120ms。",
	}
	for _, text := range texts {
		if v := Check(text); !v.Allowed {
			t.Errorf("正常文本应通过，got %+v（文本：%s）", v, text)
		}
	}
}

func TestCheck_AllowsEmptyAndWhitespace(t *testing.T) {
	for _, text := range []string{"", "   ", "\n\t "} {
		if v := Check(text); !v.Allowed {
			t.Errorf("空/空白文本应通过，got %+v", v)
		}
	}
}

func TestCheck_BlocksByWordList(t *testing.T) {
	v := Check("我可以帮你代考，保证通过")
	if v.Allowed {
		t.Fatalf("命中词表应拒绝，got %+v", v)
	}
	if v.Category != CategoryIllegal || v.RuleID != "illegal.exam_fraud_ghost" {
		t.Errorf("分类/规则 ID 不符：%+v", v)
	}
}

func TestCheck_ReasonDoesNotLeakMatchedText(t *testing.T) {
	// 契约：Reason 只描述规则与类别，不得回显命中原文（审计日志不得泄露用户内容）
	v := Check("出售海洛因")
	if v.Allowed {
		t.Fatalf("应拒绝，got %+v", v)
	}
	if strings.Contains(v.Reason, "海洛因") {
		t.Errorf("Reason 不得包含命中原文：%q", v.Reason)
	}
}

func TestCheck_DefeatsObfuscatedWord(t *testing.T) {
	// 关键安全属性：插入零宽字符的混淆写法必须仍被词表命中
	v := Check("代\u200B考包过")
	if v.Allowed {
		t.Fatalf("混淆写法应被归一化后命中，got %+v", v)
	}
	if v.RuleID != "illegal.exam_fraud_ghost" {
		t.Errorf("规则 ID 不符：%+v", v)
	}
}

func TestCheck_AllowsSingleZeroWidthButSanitizes(t *testing.T) {
	// 边界：单个零宽字符视为复制粘贴残留，不拒绝，但清洗时应移除
	if v := Check("张三\u200B 3 年经验"); !v.Allowed {
		t.Fatalf("单个零宽字符不应拒绝，got %+v", v)
	}
	cleaned, removed := Sanitize("张三\u200B 3 年经验")
	if strings.ContainsRune(cleaned, '\u200B') {
		t.Errorf("Sanitize 应移除零宽字符：%q", cleaned)
	}
	if len(removed) != 1 || removed[0] != "structure.zero_width" {
		t.Errorf("应返回被移除的规则 ID，got %v", removed)
	}
}

func TestCheck_ZeroWidthThreshold(t *testing.T) {
	// 边界：4 个零宽字符不拒绝，5 个达阈值拒绝
	if v := Check("正常" + strings.Repeat("\u200B", zeroWidthBlockThreshold-1)); !v.Allowed {
		t.Errorf("低于阈值不应拒绝，got %+v", v)
	}
	v := Check("正常" + strings.Repeat("\u200B", zeroWidthBlockThreshold))
	if v.Allowed || v.RuleID != "structure.zero_width" {
		t.Errorf("达到阈值应拒绝，got %+v", v)
	}
}

func TestCheck_BlocksBidiControl(t *testing.T) {
	v := Check("正常文本\u202E")
	if v.Allowed || v.RuleID != "structure.bidi_control" {
		t.Errorf("双向控制字符应拒绝，got %+v", v)
	}
}

func TestCheck_BlocksRepeatedRune(t *testing.T) {
	v := Check(strings.Repeat("啊", repeatedRuneThreshold))
	if v.Allowed || v.RuleID != "structure.repeated_rune" {
		t.Errorf("异常重复字符应拒绝，got %+v", v)
	}
}

func TestCheck_Oversize(t *testing.T) {
	// 用交替字符构造长文本，避免触发「异常重复字符」结构规则而干扰长度判定
	longText := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteRune(rune('a' + i%26))
		}
		return b.String()
	}
	v := CheckLimited(longText(101), 100)
	if v.Allowed || v.Category != CategoryOversize {
		t.Errorf("超长文本应拒绝，got %+v", v)
	}
	// 边界：恰好等于上限应通过
	if v := CheckLimited(longText(100), 100); !v.Allowed {
		t.Errorf("长度等于上限应通过，got %+v", v)
	}
}

func TestCheckList(t *testing.T) {
	if v := CheckList([]string{"建议加强结构化表达", "可以研究一下赌博网站的引流手法"}); v.Allowed {
		t.Errorf("数组中任一命中即应拒绝，got %+v", v)
	}
	if v := CheckList([]string{"建议加强结构化表达", "建议补充项目数据"}); !v.Allowed {
		t.Errorf("全部合规应通过，got %+v", v)
	}
	if v := CheckList(nil); !v.Allowed {
		t.Errorf("空数组应通过，got %+v", v)
	}
}

func TestSanitize_PreservesNewlineAndTab(t *testing.T) {
	cleaned, removed := Sanitize("第一行\n第二行\t结尾")
	if cleaned != "第一行\n第二行\t结尾" {
		t.Errorf("不应移除换行与制表符：%q", cleaned)
	}
	if len(removed) != 0 {
		t.Errorf("不应报告移除项：%v", removed)
	}
}

func TestSanitize_NormalizesBidiAndControl(t *testing.T) {
	cleaned, removed := Sanitize("a\u202Eb\x07c")
	if cleaned != "abc" {
		t.Errorf("应移除双向控制与 C0 控制字符：%q", cleaned)
	}
	if len(removed) != 2 {
		t.Errorf("应返回 2 条被移除规则 ID，got %v", removed)
	}
}

func TestCategoryLabel_CoversAllCategories(t *testing.T) {
	categories := []string{
		CategoryIllegal, CategoryPorn, CategoryViolence,
		CategoryAbuse, CategoryObstruction, CategoryOversize, "unknown",
	}
	for _, c := range categories {
		if label := CategoryLabel(c); label == "" {
			t.Errorf("类别 %q 缺少标签", c)
		}
	}
}

func TestBaselineRules_WellFormed(t *testing.T) {
	// 契约：规则必须分类合法、ID 唯一、Pattern 非空且已小写
	seen := make(map[string]bool, len(baselineRules))
	for _, r := range baselineRules {
		if r.ID == "" || r.Pattern == "" {
			t.Errorf("规则字段不完整：%+v", r)
		}
		if seen[r.ID] {
			t.Errorf("规则 ID 重复：%s", r.ID)
		}
		seen[r.ID] = true
		if r.Pattern != strings.ToLower(r.Pattern) {
			t.Errorf("Pattern 必须小写：%+v", r)
		}
		switch r.Category {
		case CategoryIllegal, CategoryPorn, CategoryViolence, CategoryAbuse:
		default:
			t.Errorf("未预期的分类：%+v", r)
		}
	}
}
