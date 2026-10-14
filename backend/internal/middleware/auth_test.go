package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// withRole 构造带角色上下文的请求（模拟 Auth 中间件之后的状态）
func withRole(role string) *http.Request {
	ctx := context.Background()
	if role != "" {
		ctx = context.WithValue(ctx, RoleKey, role)
	}
	return httptest.NewRequest(http.MethodGet, "/x", nil).WithContext(ctx)
}

func TestRequireAdmin_AdminAllowed(t *testing.T) {
	called := false
	h := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withRole("admin"))

	if !called {
		t.Fatal("admin request should reach next handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d", rec.Code)
	}
}

func TestRequireAdmin_UserForbidden(t *testing.T) {
	called := false
	h := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withRole("user"))

	if called {
		t.Fatal("ordinary user must not reach next handler")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for user role, got %d", rec.Code)
	}
}

// 旧 token / 异常上下文没有角色时，GetUserRole 兜底为 user，必须拒绝
func TestRequireAdmin_MissingRoleForbidden(t *testing.T) {
	h := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request without role must not reach next handler")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withRole(""))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when role missing, got %d", rec.Code)
	}
}

func TestGetUserRole_DefaultsToUser(t *testing.T) {
	if got := GetUserRole(context.Background()); got != "user" {
		t.Fatalf("expected default role user, got %q", got)
	}
	if got := GetUserID(context.Background()); got != "" {
		t.Fatalf("expected empty user id from empty context, got %q", got)
	}
}
