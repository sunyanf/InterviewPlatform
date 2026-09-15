package evaluation

import (
	"strings"
	"testing"

	"ai-interview-platform/internal/agent"
)

func TestBuildRetrievalQuery(t *testing.T) {
	qa := []agent.EvaluatedQA{
		{Question: "goroutine 泄漏如何排查？"},
		{Question: "context 的作用？"},
	}

	got := buildRetrievalQuery("Go 后端工程师", qa)
	if !strings.Contains(got, "Go 后端工程师") {
		t.Fatalf("query should contain job title, got %q", got)
	}
	for _, want := range []string{"goroutine 泄漏如何排查？", "context 的作用？"} {
		if !strings.Contains(got, want) {
			t.Fatalf("query should contain question %q, got %q", want, got)
		}
	}

	// 超长查询按 rune 截断（中文按字符而非字节），避免 embed 输入无界
	long := buildRetrievalQuery("", []agent.EvaluatedQA{{Question: strings.Repeat("问", 5000)}})
	if n := len([]rune(long)); n > 1000 {
		t.Fatalf("query should be truncated to 1000 runes, got %d", n)
	}
}
