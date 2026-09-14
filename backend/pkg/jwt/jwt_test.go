package jwt

import (
	"testing"
	"time"

	"ai-interview-platform/internal/config"
)

func TestGenerateAndParse(t *testing.T) {
	mgr := NewManager(config.JWTConfig{
		Secret:     "test-secret",
		ExpireTime: time.Hour,
		Issuer:     "test-issuer",
	})

	token, err := mgr.Generate("user-123", "test@example.com")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}

	claims, err := mgr.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected user_id user-123, got %s", claims.UserID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", claims.Email)
	}
}

func TestParse_InvalidToken(t *testing.T) {
	mgr := NewManager(config.JWTConfig{
		Secret:     "test-secret",
		ExpireTime: time.Hour,
	})

	_, err := mgr.Parse("invalid-token-string")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestParse_WrongSecret(t *testing.T) {
	mgr1 := NewManager(config.JWTConfig{Secret: "secret-1", ExpireTime: time.Hour})
	mgr2 := NewManager(config.JWTConfig{Secret: "secret-2", ExpireTime: time.Hour})

	token, err := mgr1.Generate("user-1", "a@b.com")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	_, err = mgr2.Parse(token)
	if err == nil {
		t.Error("expected error when parsing token with wrong secret")
	}
}
