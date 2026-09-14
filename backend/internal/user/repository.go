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

	query := `
		INSERT INTO users (id, email, password_hash, nickname, avatar_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		u.ID, u.Email, u.PasswordHash, u.Nickname, u.AvatarURL, u.Status, u.CreatedAt, u.UpdatedAt,
	)
	return err
}

// GetByEmail 根据邮箱查询用户
func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password_hash, nickname, avatar_url, status, created_at, updated_at
		FROM users WHERE email = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Nickname, &u.AvatarURL, &u.Status, &u.CreatedAt, &u.UpdatedAt,
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
		SELECT id, email, password_hash, nickname, avatar_url, status, created_at, updated_at
		FROM users WHERE id = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Nickname, &u.AvatarURL, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}
