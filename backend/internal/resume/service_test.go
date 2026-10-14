package resume

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/llm"
	"ai-interview-platform/pkg/storage"
)

// ---------- fakes ----------

// fakeRepo 内存版简历 repo（实现 resumeRepo）
type fakeRepo struct {
	resume            *Resume
	parseUpdates      int
	lastParseStatus   string
	lastParsedContent string
}

func (f *fakeRepo) Create(ctx context.Context, rs *Resume) error { return nil }
func (f *fakeRepo) GetByID(ctx context.Context, id string) (*Resume, error) {
	return f.resume, nil
}
func (f *fakeRepo) ListByUserID(ctx context.Context, userID string) ([]Resume, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateParseResult(ctx context.Context, id, parsedContent string, structuredData any, skills []string, status string) error {
	f.parseUpdates++
	f.lastParseStatus = status
	f.lastParsedContent = parsedContent
	return nil
}
func (f *fakeRepo) UpdateStatus(ctx context.Context, id, status string) error { return nil }

// fakeStorage 内存版对象存储（实现 storage.Storage）
type fakeStorage struct {
	content []byte
}

func (f *fakeStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	return nil
}
func (f *fakeStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	if f.content == nil {
		return nil, errors.New("not found")
	}
	return io.NopCloser(strings.NewReader(string(f.content))), nil
}
func (f *fakeStorage) GetSignedURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return "", nil
}
func (f *fakeStorage) Exists(ctx context.Context, key string) (bool, error) { return true, nil }
func (f *fakeStorage) Delete(ctx context.Context, key string) error         { return nil }

var _ storage.Storage = (*fakeStorage)(nil)

// fakeLLM 固定返回错误的 LLM（用于验证正常文本不会在内容安全阶段被拦）
type fakeLLM struct{ err error }

func (f *fakeLLM) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, f.err
}
func (f *fakeLLM) Name() string { return "fake" }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestParse_ContentRejected 简历正文含违禁词 → CONTENT_REJECTED 且置解析失败
func TestParse_ContentRejected(t *testing.T) {
	repo := &fakeRepo{resume: &Resume{ID: "r1", FileName: "r.txt", FileURL: "k1"}}
	st := &fakeStorage{content: []byte("张三，3 年经验；可提供代考服务")}
	svc := &Service{repo: repo, storage: st, log: discardLogger()}

	_, err := svc.Parse(context.Background(), "r1")
	if err == nil {
		t.Fatal("expected CONTENT_REJECTED, got nil")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "CONTENT_REJECTED" || ae.Status != 400 {
		t.Fatalf("got %v, want CONTENT_REJECTED 400", err)
	}
	if repo.lastParseStatus != StatusFailed {
		t.Fatalf("parse status = %q, want %q", repo.lastParseStatus, StatusFailed)
	}
	if repo.lastParsedContent != "" {
		t.Fatalf("rejected resume must not persist parsed content, got %q", repo.lastParsedContent)
	}
}

// TestParse_NormalTextPassesGuard 正常简历文本不应被内容安全误拒（用 fake llm 触发后续错误即可区分）
func TestParse_NormalTextPassesGuard(t *testing.T) {
	repo := &fakeRepo{resume: &Resume{ID: "r1", FileName: "r.txt", FileURL: "k1"}}
	st := &fakeStorage{content: []byte("张三，3 年 Go 后端经验，熟悉 PostgreSQL 与 Redis")}
	svc := &Service{repo: repo, storage: st, llm: &fakeLLM{err: errors.New("boom")}, log: discardLogger()}

	_, err := svc.Parse(context.Background(), "r1")
	if err == nil {
		t.Fatal("expected downstream LLM error, got nil")
	}
	var ae *apperrors.AppError
	if errors.As(err, &ae) && ae.Code == "CONTENT_REJECTED" {
		t.Fatalf("normal text must not be rejected: %v", err)
	}
}

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
