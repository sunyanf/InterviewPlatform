package resume

import (
	"testing"
)

func TestExtractSkillKeywords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"Go, MySQL, Redis", 3},
		{"Go、MySQL、Redis", 3},
		{"", 0},
		{"Go MySQL Redis", 3},
		{"熟悉 Go 和 Python", 4}, // 熟悉(3字节) Go(2) 和(3字节) Python(6)
	}

	for _, tt := range tests {
		got := extractSkillKeywords(tt.input)
		if len(got) != tt.want {
			t.Errorf("extractSkillKeywords(%q) returned %d keywords (%v), want %d", tt.input, len(got), got, tt.want)
		}
	}
}

func TestCleanJSON(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{"name":"test"}`, `{"name":"test"}`},
		{"```json\n{\"name\":\"test\"}\n```", `{"name":"test"}`},
		{"```\n{\"name\":\"test\"}\n```", `{"name":"test"}`},
		{"  {\"name\":\"test\"}  ", `{"name":"test"}`},
	}

	for _, tt := range tests {
		got := cleanJSON(tt.input)
		if got != tt.want {
			t.Errorf("cleanJSON(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
