package user

import (
	"context"
	"encoding/hex"
	"io"
	"log/slog"
	"testing"
	"time"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/pkg/jwt"
)

func newTestService() *Service {
	jwtMgr := jwt.NewManager(config.JWTConfig{
		Secret:     "test-secret",
		ExpireTime: time.Hour,
	})
	// repo 为 nil，仅用于测试不触发 DB 调用的校验逻辑
	return NewService(nil, jwtMgr, 24*time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

var noMeta = TokenMeta{}

func TestRegister_InvalidEmail(t *testing.T) {
	svc := newTestService()
	_, err := svc.Register(context.Background(), RegisterRequest{
		Email:    "not-an-email",
		Password: "password123",
	}, noMeta)
	if err == nil {
		t.Error("expected error for invalid email")
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	svc := newTestService()
	_, err := svc.Register(context.Background(), RegisterRequest{
		Email:    "valid@example.com",
		Password: "123",
	}, noMeta)
	if err == nil {
		t.Error("expected error for short password")
	}
}

func TestLogin_EmptyCredentials(t *testing.T) {
	svc := newTestService()
	_, err := svc.Login(context.Background(), LoginRequest{
		Email:    "",
		Password: "",
	}, noMeta)
	if err == nil {
		t.Error("expected error for empty credentials")
	}
}

// TestRefresh_EmptyToken 空 refresh token 必须在访问 DB 前被拒绝
func TestRefresh_EmptyToken(t *testing.T) {
	svc := newTestService()
	if _, err := svc.Refresh(context.Background(), "", noMeta); err == nil {
		t.Error("expected error for empty refresh token")
	}
}

// TestLogout_EmptyToken 空 token 登出视为幂等成功
func TestLogout_EmptyToken(t *testing.T) {
	svc := newTestService()
	if err := svc.Logout(context.Background(), ""); err != nil {
		t.Errorf("logout with empty token should be no-op, got %v", err)
	}
}

func TestGenerateRawToken(t *testing.T) {
	a, err := generateRawToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	b, err := generateRawToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if len(a) != 64 {
		t.Errorf("token length = %d, want 64 hex chars", len(a))
	}
	if a == b {
		t.Error("two random tokens must differ")
	}
}

func TestHashToken(t *testing.T) {
	h1 := hashToken("raw-token")
	h2 := hashToken("raw-token")
	if h1 != h2 {
		t.Error("same input must hash to same value")
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64 hex chars", len(h1))
	}
	if h1 == hashToken("raw-tokeN") {
		t.Error("different input must hash differently")
	}
	// 哈希必须是合法 hex（数据库不存明文）
	if _, err := hex.DecodeString(h1); err != nil {
		t.Errorf("hash must be hex: %v", err)
	}
	if h1 == "raw-token" {
		t.Error("stored value must not equal raw token")
	}
}

func TestTruncateMeta(t *testing.T) {
	if got := truncateMeta("短", 10); got != "短" {
		t.Errorf("short value unchanged, got %q", got)
	}
	long := make([]byte, 100)
	for i := range long {
		long[i] = 'a'
	}
	if got := truncateMeta(string(long), 10); len(got) != 10 {
		t.Errorf("truncated length = %d, want 10", len(got))
	}
}

func TestEmailRegex(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"test@example.com", true},
		{"user.name+tag@domain.co.uk", true},
		{"invalid", false},
		{"@example.com", false},
		{"test@", false},
		{"test@example", false},
	}

	for _, tt := range tests {
		got := emailRegex.MatchString(tt.email)
		if got != tt.valid {
			t.Errorf("emailRegex(%q) = %v, want %v", tt.email, got, tt.valid)
		}
	}
}
