package user

import (
	"context"
	"testing"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/pkg/jwt"
)

func newTestService() *Service {
	jwtMgr := jwt.NewManager(config.JWTConfig{
		Secret:     "test-secret",
		ExpireTime: 0,
	})
	// repo 为 nil，仅用于测试不触发 DB 调用的校验逻辑
	return NewService(nil, jwtMgr)
}

func TestRegister_InvalidEmail(t *testing.T) {
	svc := newTestService()
	_, err := svc.Register(context.Background(), RegisterRequest{
		Email:    "not-an-email",
		Password: "password123",
	})
	if err == nil {
		t.Error("expected error for invalid email")
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	svc := newTestService()
	_, err := svc.Register(context.Background(), RegisterRequest{
		Email:    "valid@example.com",
		Password: "123",
	})
	if err == nil {
		t.Error("expected error for short password")
	}
}

func TestLogin_EmptyCredentials(t *testing.T) {
	svc := newTestService()
	_, err := svc.Login(context.Background(), LoginRequest{
		Email:    "",
		Password: "",
	})
	if err == nil {
		t.Error("expected error for empty credentials")
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
