package settings

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository 配置数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建配置 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetAll 查询全部配置项
func (r *Repository) GetAll(ctx context.Context) ([]Setting, error) {
	query := `SELECT key, value, updated_at FROM settings ORDER BY key`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Setting
	for rows.Next() {
		var s Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, nil
}

// Get 查询单个配置项
func (r *Repository) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&value)
	return value, err
}

// Upsert 插入或更新单个配置项
func (r *Repository) Upsert(ctx context.Context, key, value string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
		key, value)
	return err
}

// UpsertMany 批量插入或更新（事务）
func (r *Repository) UpsertMany(ctx context.Context, items map[string]string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for k, v := range items {
		_, err := tx.Exec(ctx,
			`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
			 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
			k, v)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// EnsureDefaults 对缺失的键插入空行（ON CONFLICT DO NOTHING）
func (r *Repository) EnsureDefaults(ctx context.Context, defaults map[string]string) error {
	for k, v := range defaults {
		_, err := r.db.Exec(ctx,
			`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
			 ON CONFLICT (key) DO NOTHING`,
			k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdatedAt 查询单个 key 的更新时间
func (r *Repository) UpdatedAt(ctx context.Context, key string) (time.Time, error) {
	var t time.Time
	err := r.db.QueryRow(ctx, `SELECT updated_at FROM settings WHERE key = $1`, key).Scan(&t)
	return t, err
}
