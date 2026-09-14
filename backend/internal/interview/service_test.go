package interview

import "testing"

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		// 合法转换
		{"INIT 到 RUNNING", StatusInit, StatusRunning, true},
		{"INIT 到 READY", StatusInit, StatusReady, true},
		{"INIT 到 FAILED", StatusInit, StatusFailed, true},
		{"READY 到 RUNNING", StatusReady, StatusRunning, true},
		{"RUNNING 到 COMPLETED", StatusRunning, StatusCompleted, true},
		{"RUNNING 到 PAUSED", StatusRunning, StatusPaused, true},
		{"RUNNING 到 FINISHING", StatusRunning, StatusFinishing, true},
		{"PAUSED 到 RUNNING", StatusPaused, StatusRunning, true},
		{"FINISHING 到 EVALUATING", StatusFinishing, StatusEvaluating, true},
		{"FINISHING 到 COMPLETED", StatusFinishing, StatusCompleted, true},
		{"EVALUATING 到 COMPLETED", StatusEvaluating, StatusCompleted, true},

		// 非法转换
		{"INIT 到 COMPLETED", StatusInit, StatusCompleted, false},
		{"COMPLETED 到 RUNNING", StatusCompleted, StatusRunning, false},
		{"COMPLETED 到 COMPLETED", StatusCompleted, StatusCompleted, false},
		{"RUNNING 到 INIT", StatusRunning, StatusInit, false},
		{"FAILED 到 RUNNING", StatusFailed, StatusRunning, false},
		{"PAUSED 到 COMPLETED", StatusPaused, StatusCompleted, false},
		{"未知状态 到 RUNNING", "UNKNOWN", StatusRunning, false},
		{"RUNNING 到 未知状态", StatusRunning, "UNKNOWN", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("canTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestCleanJSON(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{"questions":[]}`, `{"questions":[]}`},
		{"```json\n{\"questions\":[]}\n```", `{"questions":[]}`},
		{"```\n{\"questions\":[]}\n```", `{"questions":[]}`},
		{"  {\"questions\":[]}  ", `{"questions":[]}`},
	}

	for _, tt := range tests {
		got := cleanJSON(tt.input)
		if got != tt.want {
			t.Errorf("cleanJSON(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
