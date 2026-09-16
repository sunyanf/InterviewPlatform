package user

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository 用户数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建用户 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create 创建用户
func (r *Repository) Create(ctx context.Context, u *User) error {
	u.ID = uuid.New().String()
	u.Status = "active"
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now

	if u.Role == "" {
		u.Role = RoleUser
	}
	query := `
		INSERT INTO users (id, email, password_hash, nickname, avatar_url, status, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		u.ID, u.Email, u.PasswordHash, u.Nickname, u.AvatarURL, u.Status, u.Role, u.CreatedAt, u.UpdatedAt,
	)
	return err
}

// GetByEmail 根据邮箱查询用户
func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password_hash, nickname, avatar_url, status, role, created_at, updated_at
		FROM users WHERE email = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Nickname, &u.AvatarURL, &u.Status, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// GetByID 根据 ID 查询用户
func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, email, password_hash, nickname, avatar_url, status, role, created_at, updated_at
		FROM users WHERE id = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Nickname, &u.AvatarURL, &u.Status, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// ListAll 分页查询全部用户（admin 用，password_hash 返回空串）
func (r *Repository) ListAll(ctx context.Context, page, pageSize int) ([]*User, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, email, '', nickname, avatar_url, status, role, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Nickname, &u.AvatarURL, &u.Status, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, &u)
	}
	return users, total, nil
}

// UpdateStatus 更新用户状态（active/disabled）
func (r *Repository) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}
