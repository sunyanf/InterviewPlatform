package user

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateRefreshToken 持久化新的 refresh token（存哈希）
func (r *Repository) CreateRefreshToken(ctx context.Context, t *RefreshToken) error {
	t.ID = uuid.New().String()
	now := time.Now()
	t.CreatedAt = now
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked_at, user_agent, client_ip, created_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.RevokedAt, t.UserAgent, t.ClientIP, t.CreatedAt, t.LastUsedAt)
	return err
}

// GetRefreshTokenByHash 按哈希查询 token（含已吊销/已过期记录，用于被盗检测）
func (r *Repository) GetRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error) {
	var t RefreshToken
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, revoked_at, user_agent, client_ip, created_at, last_used_at
		FROM refresh_tokens WHERE token_hash = $1
	`, hash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt,
		&t.UserAgent, &t.ClientIP, &t.CreatedAt, &t.LastUsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// RotateRefreshToken 原子旋转：仅当 token 未吊销且未过期时吊销旧行并返回其 user_id。
// 返回 ("", nil) 表示不存在/已吊销/已过期，调用方应再查 GetRefreshTokenByHash 区分被盗场景。
func (r *Repository) RotateRefreshToken(ctx context.Context, hash string) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW(), last_used_at = NOW()
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()
		RETURNING user_id
	`, hash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return userID, nil
}

// RevokeRefreshToken 吊销单个 token（幂等：已吊销不报错）
func (r *Repository) RevokeRefreshToken(ctx context.Context, hash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, hash)
	return err
}

// RevokeAllActiveTokens 吊销某用户全部活跃 token（refresh token 被盗用/重放时使用）
func (r *Repository) RevokeAllActiveTokens(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	return err
}

// DeleteExpiredRefreshTokens 物理删除过期 token（登录时顺带触发，无需常驻 goroutine）
func (r *Repository) DeleteExpiredRefreshTokens(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < NOW()`)
	return err
}
