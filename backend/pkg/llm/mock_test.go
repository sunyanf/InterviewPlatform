package llm

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMockProvider_ResumeParse(t *testing.T) {
	prov := NewMockProvider()
	resp, err := prov.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "请解析这份简历"},
		},
	})
	if err != nil {
		t.Fatalf("Chat error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	if _, ok := result["skills"]; !ok {
		t.Error("expected skills field in mock response")
	}
}

func TestMockProvider_Name(t *testing.T) {
	prov := NewMockProvider()
	if prov.Name() != "mock" {
		t.Errorf("expected name 'mock', got %s", prov.Name())
	}
}
