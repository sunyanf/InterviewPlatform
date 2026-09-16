package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/jwt"
)

// Service 用户业务逻辑层
type Service struct {
	repo       *Repository
	jwtMgr     *jwt.Manager
	refreshTTL time.Duration
	log        *slog.Logger
}

// NewService 创建用户 Service
func NewService(repo *Repository, jwtMgr *jwt.Manager, refreshTTL time.Duration, log *slog.Logger) *Service {
	return &Service{repo: repo, jwtMgr: jwtMgr, refreshTTL: refreshTTL, log: log}
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Register 用户注册（注册即登录：返回令牌对）
func (s *Service) Register(ctx context.Context, req RegisterRequest, meta TokenMeta) (*LoginResponse, error) {
	if req.Email == "" || !emailRegex.MatchString(req.Email) {
		return nil, apperrors.New("INVALID_EMAIL", "邮箱格式不正确", 400)
	}
	if len(req.Password) < 6 {
		return nil, apperrors.New("INVALID_PASSWORD", "密码长度不能少于 6 位", 400)
	}

	// 检查邮箱是否已注册
	existing, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询用户失败", 500, err)
	}
	if existing != nil {
		return nil, apperrors.ErrEmailAlreadyExists
	}

	// 哈希密码
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "密码加密失败", 500, err)
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Email
	}

	u := &User{
		Email:        req.Email,
		PasswordHash: string(hash),
		Nickname:     nickname,
		Role:         RoleUser,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "创建用户失败", 500, err)
	}

	u.PasswordHash = ""
	return s.issueTokens(ctx, u, meta)
}

// Login 用户登录
func (s *Service) Login(ctx context.Context, req LoginRequest, meta TokenMeta) (*LoginResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, apperrors.New("INVALID_INPUT", "邮箱和密码不能为空", 400)
	}

	u, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询用户失败", 500, err)
	}
	if u == nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	u.PasswordHash = ""
	return s.issueTokens(ctx, u, meta)
}

// Refresh 使用 refresh token 旋转出新的令牌对。
// 安全策略：旧 token 一次性有效；已吊销 token 被再次使用视为被盗，吊销该用户全部活跃 token。
func (s *Service) Refresh(ctx context.Context, rawToken string, meta TokenMeta) (*LoginResponse, error) {
	if rawToken == "" {
		return nil, apperrors.ErrInvalidToken
	}

	hash := hashToken(rawToken)

	// 原子旋转：未吊销且未过期才会成功
	userID, err := s.repo.RotateRefreshToken(ctx, hash)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "刷新令牌失败", 500, err)
	}

	if userID == "" {
		// 区分：被盗重放（已吊销）vs 不存在/已过期
		old, lookupErr := s.repo.GetRefreshTokenByHash(ctx, hash)
		if lookupErr != nil {
			return nil, apperrors.Wrap("INTERNAL_ERROR", "查询刷新令牌失败", 500, lookupErr)
		}
		if old != nil && old.RevokedAt != nil {
			if revokeErr := s.repo.RevokeAllActiveTokens(ctx, old.UserID); revokeErr != nil {
				s.log.Error("revoke all tokens after refresh token reuse failed",
					"user_id", old.UserID, "error", revokeErr)
			}
			s.log.Warn("refresh token reuse detected; revoked all tokens",
				"user_id", old.UserID, "client_ip", meta.ClientIP)
		}
		return nil, apperrors.ErrInvalidToken
	}

	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询用户失败", 500, err)
	}
	if u == nil || u.Status != "active" {
		return nil, apperrors.ErrInvalidToken
	}
	u.PasswordHash = ""
	return s.issueTokens(ctx, u, meta)
}

// Logout 吊销提交的 refresh token（幂等）
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	if err := s.repo.RevokeRefreshToken(ctx, hashToken(rawToken)); err != nil {
		return apperrors.Wrap("INTERNAL_ERROR", "登出失败", 500, err)
	}
	return nil
}

// GetProfile 获取用户资料
func (s *Service) GetProfile(ctx context.Context, userID string) (*User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询用户失败", 500, err)
	}
	if u == nil {
		return nil, apperrors.ErrNotFound
	}
	u.PasswordHash = ""
	return u, nil
}

// issueTokens 生成 access + refresh 令牌对并持久化 refresh token 哈希
func (s *Service) issueTokens(ctx context.Context, u *User, meta TokenMeta) (*LoginResponse, error) {
	access, err := s.jwtMgr.Generate(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "生成 Token 失败", 500, err)
	}

	rawRefresh, err := generateRawToken()
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "生成刷新令牌失败", 500, err)
	}

	rt := &RefreshToken{
		UserID:    u.ID,
		TokenHash: hashToken(rawRefresh),
		ExpiresAt: time.Now().Add(s.refreshTTL),
		UserAgent: truncateMeta(meta.UserAgent, 256),
		ClientIP:  truncateMeta(meta.ClientIP, 64),
	}
	if err := s.repo.CreateRefreshToken(ctx, rt); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "保存刷新令牌失败", 500, err)
	}

	// 顺带清理过期 token，失败不影响主流程
	if err := s.repo.DeleteExpiredRefreshTokens(ctx); err != nil {
		s.log.Debug("delete expired refresh tokens failed", "error", err)
	}

	return &LoginResponse{
		Token:        access,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(s.jwtMgr.TTL().Seconds()),
		User:         u,
	}, nil
}

// generateRawToken 生成 256bit 随机 refresh token（hex 编码，64 字符）
func generateRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashToken 对原始 token 做 SHA-256（数据库不存明文）
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func truncateMeta(v string, max int) string {
	if len(v) <= max {
		return v
	}
	return v[:max]
}
