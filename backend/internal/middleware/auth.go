package middleware

import (
	"context"
	"net/http"
	"strings"

	"ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/jwt"
	"ai-interview-platform/pkg/response"
)

type userCtxKey string

const (
	UserIDKey userCtxKey = "user_id"
	RoleKey   userCtxKey = "role"
)

// Auth JWT 鉴权中间件
func Auth(jwtMgr *jwt.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, errors.ErrUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				response.Error(w, errors.ErrInvalidToken)
				return
			}

			claims, err := jwtMgr.Parse(parts[1])
			if err != nil {
				response.Error(w, errors.ErrInvalidToken)
				return
			}

			role := claims.Role
			if role == "" {
				role = "user" // 旧 token 兜底
			}
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, RoleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID 从上下文中获取当前用户 ID
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

// GetUserRole 从上下文中获取当前用户角色
func GetUserRole(ctx context.Context) string {
	if role, ok := ctx.Value(RoleKey).(string); ok {
		return role
	}
	return "user"
}

// RequireAdmin 仅放行 role == "admin" 的请求
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetUserRole(r.Context()) != "admin" {
			response.Error(w, errors.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
