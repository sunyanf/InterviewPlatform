package job

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository 岗位数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建岗位 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateCategory 创建岗位分类
func (r *Repository) CreateCategory(ctx context.Context, c *Category) error {
	c.ID = uuid.New().String()
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now

	_, err := r.db.Exec(ctx,
		`INSERT INTO job_categories (id, name, description, sort_order, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		c.ID, c.Name, c.Description, c.SortOrder, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

// ListCategories 查询所有分类
func (r *Repository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, description, sort_order, created_at, updated_at
		 FROM job_categories ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// CreateJob 创建岗位
func (r *Repository) CreateJob(ctx context.Context, j *Job) error {
	j.ID = uuid.New().String()
	j.Status = "active"
	now := time.Now()
	j.CreatedAt = now
	j.UpdatedAt = now

	reqJSON, _ := json.Marshal(j.Requirements)
	skillsJSON, _ := json.Marshal(j.Skills)

	_, err := r.db.Exec(ctx,
		`INSERT INTO jobs (id, category_id, title, description, requirements, skills, source, source_url, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		j.ID, j.CategoryID, j.Title, j.Description, reqJSON, skillsJSON, j.Source, j.SourceURL, j.Status, j.CreatedAt, j.UpdatedAt,
	)
	return err
}

// GetJobByID 根据 ID 查询岗位
func (r *Repository) GetJobByID(ctx context.Context, id string) (*Job, error) {
	var j Job
	var reqJSON, skillsJSON string

	err := r.db.QueryRow(ctx,
		`SELECT id, category_id, title, COALESCE(description,''), requirements::text, skills::text, COALESCE(source,''), COALESCE(source_url,''), status, created_at, updated_at
		 FROM jobs WHERE id = $1`, id,
	).Scan(&j.ID, &j.CategoryID, &j.Title, &j.Description, &reqJSON, &skillsJSON, &j.Source, &j.SourceURL, &j.Status, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(reqJSON), &j.Requirements)
	_ = json.Unmarshal([]byte(skillsJSON), &j.Skills)
	return &j, nil
}

// ListJobs 分页查询岗位
func (r *Repository) ListJobs(ctx context.Context, categoryID string, page, pageSize int) ([]Job, int, error) {
	offset := (page - 1) * pageSize

	// 统计总数
	var total int
	countQuery := `SELECT COUNT(*) FROM jobs WHERE status = 'active'`
	countArgs := []interface{}{}
	if categoryID != "" {
		countQuery += ` AND category_id = $1`
		countArgs = append(countArgs, categoryID)
	}
	if err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 查询列表
	listQuery := `SELECT id, category_id, title, COALESCE(description,''), requirements::text, skills::text, COALESCE(source,''), COALESCE(source_url,''), status, created_at, updated_at
				  FROM jobs WHERE status = 'active'`
	listArgs := []interface{}{}
	paramIdx := 1
	if categoryID != "" {
		listQuery += ` AND category_id = $` + strconv.Itoa(paramIdx)
		listArgs = append(listArgs, categoryID)
		paramIdx++
	}
	listQuery += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(paramIdx) + ` OFFSET $` + strconv.Itoa(paramIdx+1)
	listArgs = append(listArgs, pageSize, offset)

	rows, err := r.db.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []Job
	for rows.Next() {
		var j Job
		var reqJSON, skillsJSON string
		if err := rows.Scan(&j.ID, &j.CategoryID, &j.Title, &j.Description, &reqJSON, &skillsJSON, &j.Source, &j.SourceURL, &j.Status, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal([]byte(reqJSON), &j.Requirements)
		_ = json.Unmarshal([]byte(skillsJSON), &j.Skills)
		list = append(list, j)
	}
	return list, total, rows.Err()
}
