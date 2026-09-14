package user

import (
	"context"
	"regexp"

	"golang.org/x/crypto/bcrypt"

	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/jwt"
)

// Service 用户业务逻辑层
type Service struct {
	repo   *Repository
	jwtMgr *jwt.Manager
}

// NewService 创建用户 Service
func NewService(repo *Repository, jwtMgr *jwt.Manager) *Service {
	return &Service{repo: repo, jwtMgr: jwtMgr}
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Register 用户注册
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*User, error) {
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
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "创建用户失败", 500, err)
	}

	u.PasswordHash = ""
	return u, nil
}

// Login 用户登录
func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
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

	token, err := s.jwtMgr.Generate(u.ID, u.Email)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "生成 Token 失败", 500, err)
	}

	u.PasswordHash = ""
	return &LoginResponse{Token: token, User: u}, nil
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
