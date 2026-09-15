package user

import "time"

// RefreshToken 刷新令牌持久化模型。
// 安全约束：数据库只保存原始 token 的 SHA-256 哈希，原始随机串仅在响应中出现一次。
type RefreshToken struct {
	ID         string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	UserAgent  string
	ClientIP   string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// RefreshRequest 刷新令牌请求
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutRequest 登出请求（吊销提交的 refresh token）
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// TokenMeta 签发时记录的客户端信息（审计/异常检测用，不参与鉴权）
type TokenMeta struct {
	UserAgent string
	ClientIP  string
}
