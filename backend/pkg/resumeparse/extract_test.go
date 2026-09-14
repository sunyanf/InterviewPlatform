package resumeparse

import "testing"

func TestExtractText_TXT(t *testing.T) {
	content := []byte("这是一份简历文本\n姓名：张三\n技能：Go, MySQL")
	text, err := ExtractText("resume.txt", content)
	if err != nil {
		t.Fatalf("ExtractText txt error = %v", err)
	}
	if text != string(content) {
		t.Errorf("expected content to be returned as-is")
	}
}

func TestExtractText_Unsupported(t *testing.T) {
	_, err := ExtractText("resume.doc", []byte("test"))
	if err == nil {
		t.Error("expected error for unsupported file type")
	}
}
