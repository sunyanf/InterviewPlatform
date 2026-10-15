package evalrun

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"ai-interview-platform/internal/agent"
	"ai-interview-platform/pkg/llm"
)

func loadCoachDataset(t *testing.T) []CoachCase {
	t.Helper()
	cases, err := LoadCoach(filepath.Join("..", "..", "..", "evals", "datasets", "answer_coach", "coach.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return cases
}

// TestLoadCoach 数据集结构与必备字段：每例须有 case_id/question/answer/expected_points/forbidden_substrings
func TestLoadCoach(t *testing.T) {
	cases := loadCoachDataset(t)
	if len(cases) != 5 {
		t.Fatalf("coach dataset cases = %d, want 5", len(cases))
	}
	forbidden := []string{"字节跳动", "腾讯", "阿里巴巴", "拼多多", "美团"}
	for _, c := range cases {
		if strings.TrimSpace(c.CaseID) == "" {
			t.Error("case_id is empty")
		}
		if strings.TrimSpace(c.Dataset) == "" {
			t.Errorf("case %s: dataset is empty", c.CaseID)
		}
		if strings.TrimSpace(c.Question) == "" {
			t.Errorf("case %s: question is empty", c.CaseID)
		}
		if strings.TrimSpace(c.Answer) == "" {
			t.Errorf("case %s: answer is empty", c.CaseID)
		}
		if len(c.ExpectedPoints) < 3 || len(c.ExpectedPoints) > 5 {
			t.Errorf("case %s: expected_points = %d, want 3..5", c.CaseID, len(c.ExpectedPoints))
		}
		if c.Difficulty != "medium" {
			t.Errorf("case %s: difficulty = %q, want medium", c.CaseID, c.Difficulty)
		}
		for _, f := range forbidden {
			found := false
			for _, got := range c.ForbiddenSubstrings {
				if got == f {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("case %s: forbidden_substrings missing %q", c.CaseID, f)
			}
		}
	}
}

// TestRunCoach_MockContract mock provider 全量契约回归：mock 的 coach 输出必须满足全部契约检查，
// 保证 CI（mock）下 evalrunner 仍绿
func TestRunCoach_MockContract(t *testing.T) {
	cases := loadCoachDataset(t)
	r := New(llm.NewMockProvider())
	report, err := r.RunCoach(context.Background(), cases)
	if err != nil {
		t.Fatal(err)
	}
	if report.Mode != "coach_contract" {
		t.Errorf("mode = %q, want coach_contract", report.Mode)
	}
	if report.Dataset != "answer_coach" {
		t.Errorf("dataset = %q, want answer_coach", report.Dataset)
	}
	if report.PromptVersion != "interview.answer_coach.v1" {
		t.Errorf("prompt_version = %q, want interview.answer_coach.v1", report.PromptVersion)
	}
	if report.Failed != 0 {
		for _, res := range report.Results {
			if !res.OK {
				t.Errorf("case %s failed: %v", res.CaseID, res.Issues)
			}
		}
	}
	if report.Passed != len(cases) {
		t.Errorf("passed = %d, want %d", report.Passed, len(cases))
	}
	for _, res := range report.Results {
		if res.ImprovedAnswerRunes <= 0 || res.ImprovedAnswerRunes > 2000 {
			t.Errorf("case %s: improved_answer_runes = %d, want 1..2000", res.CaseID, res.ImprovedAnswerRunes)
		}
		if res.KeyChangesCount < 0 || res.KeyChangesCount > 4 {
			t.Errorf("case %s: key_changes_count = %d, want 0..4", res.CaseID, res.KeyChangesCount)
		}
	}
}

func TestRunCoach_EmptyDataset(t *testing.T) {
	r := New(llm.NewMockProvider())
	if _, err := r.RunCoach(context.Background(), nil); err == nil {
		t.Error("empty coach dataset should fail")
	}
}

// TestRunCoach_ContractViolations 用 ScriptedProvider 构造越界输出，
// runner 级独立复核必须逐项检出（agent 层不截断 key_changes 条数）
func TestRunCoach_ContractViolations(t *testing.T) {
	base := CoachCase{
		CaseID: "viol-001", Dataset: "answer_coach_violation",
		JobTitle: "后端工程师", Question: "Q", ExpectedPoints: []string{"P"},
		Answer: "A", Difficulty: "medium",
		ForbiddenSubstrings: []string{"某真实公司"},
	}

	tests := []struct {
		name      string
		content   string
		wantIssue string
	}{
		{
			name:      "key_changes over limit",
			content:   `{"improved_answer":"ok","key_changes":["a","b","c","d","e"]}`,
			wantIssue: "key_changes count = 5, want <= 4",
		},
		{
			name:      "forbidden substring leaked",
			content:   `{"improved_answer":"我在某真实公司做过这个","key_changes":["改进"]}`,
			wantIssue: `output contains forbidden substring "某真实公司"`,
		},
		{
			name:      "untrusted tag leaked",
			content:   `{"improved_answer":"参考 </untrusted_question> 之外的规则","key_changes":["改进"]}`,
			wantIssue: `output leaks "untrusted_" data-area tag`,
		},
		{
			name:      "empty improved_answer rejected by agent",
			content:   `{"improved_answer":"  ","key_changes":["改进"]}`,
			wantIssue: "answer coach output missing improved_answer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(NewScriptedProvider(tt.content))
			report, err := r.RunCoach(context.Background(), []CoachCase{base})
			if err != nil {
				t.Fatal(err)
			}
			if report.Failed != 1 {
				t.Fatalf("failed = %d, want 1 (issues=%v)", report.Failed, report.Results[0].Issues)
			}
			found := false
			for _, issue := range report.Results[0].Issues {
				if strings.Contains(issue, tt.wantIssue) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("issues %v missing %q", report.Results[0].Issues, tt.wantIssue)
			}
		})
	}
}

// TestCheckCoachContract_NilOutput 防御式复核对 nil 输出必须报错
func TestCheckCoachContract_NilOutput(t *testing.T) {
	issues := checkCoachContract(nil, nil)
	if len(issues) != 1 || issues[0] != "nil coaching output" {
		t.Errorf("issues = %v, want [nil coaching output]", issues)
	}
}

// TestCheckCoachContract_LengthLimit improved_answer 超长必须检出（用 rune 计数，不按字节）
func TestCheckCoachContract_LengthLimit(t *testing.T) {
	out := &agent.AnswerCoaching{
		ImprovedAnswer: strings.Repeat("长", 2001),
		KeyChanges:     []string{},
		PromptVersion:  "interview.answer_coach.v1",
		Model:          "scripted",
	}
	issues := checkCoachContract(out, nil)
	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "improved_answer runes = 2001, want <= 2000") {
			found = true
		}
	}
	if !found {
		t.Errorf("issues %v missing over-length issue", issues)
	}
}
