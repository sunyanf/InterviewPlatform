package resume

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository 简历数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建简历 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create 创建简历记录
func (r *Repository) Create(ctx context.Context, rs *Resume) error {
	rs.ID = uuid.New().String()
	rs.Status = "pending"
	now := time.Now()
	rs.CreatedAt = now
	rs.UpdatedAt = now

	skillsJSON, _ := json.Marshal(rs.Skills)
	if rs.Skills == nil {
		skillsJSON = []byte("[]")
	}

	structuredJSON, _ := json.Marshal(rs.StructuredData)
	if rs.StructuredData == nil {
		structuredJSON = []byte("{}")
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO resumes (id, user_id, file_url, file_name, file_type, file_size, parsed_content, structured_data, skills, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		rs.ID, rs.UserID, rs.FileURL, rs.FileName, rs.FileType, rs.FileSize,
		rs.ParsedContent, structuredJSON, skillsJSON, rs.Status, rs.CreatedAt, rs.UpdatedAt,
	)
	return err
}

// GetByID 根据 ID 查询简历
func (r *Repository) GetByID(ctx context.Context, id string) (*Resume, error) {
	var rs Resume
	var structuredJSON, skillsJSON string

	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, file_url, file_name, file_type, file_size, COALESCE(parsed_content,''), structured_data::text, skills::text, status, created_at, updated_at
		 FROM resumes WHERE id = $1`, id,
	).Scan(&rs.ID, &rs.UserID, &rs.FileURL, &rs.FileName, &rs.FileType, &rs.FileSize,
		&rs.ParsedContent, &structuredJSON, &skillsJSON, &rs.Status, &rs.CreatedAt, &rs.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(structuredJSON), &rs.StructuredData)
	_ = json.Unmarshal([]byte(skillsJSON), &rs.Skills)
	return &rs, nil
}

// ListByUserID 查询用户的所有简历
func (r *Repository) ListByUserID(ctx context.Context, userID string) ([]Resume, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, file_url, file_name, file_type, file_size, COALESCE(parsed_content,''), structured_data::text, skills::text, status, created_at, updated_at
		 FROM resumes WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Resume
	for rows.Next() {
		var rs Resume
		var structuredJSON, skillsJSON string
		if err := rows.Scan(&rs.ID, &rs.UserID, &rs.FileURL, &rs.FileName, &rs.FileType, &rs.FileSize,
			&rs.ParsedContent, &structuredJSON, &skillsJSON, &rs.Status, &rs.CreatedAt, &rs.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(structuredJSON), &rs.StructuredData)
		_ = json.Unmarshal([]byte(skillsJSON), &rs.Skills)
		list = append(list, rs)
	}
	return list, rows.Err()
}

// UpdateParseResult 更新解析结果
func (r *Repository) UpdateParseResult(ctx context.Context, id, parsedContent string, structuredData any, skills []string, status string) error {
	structuredJSON, _ := json.Marshal(structuredData)
	skillsJSON, _ := json.Marshal(skills)
	if skills == nil {
		skillsJSON = []byte("[]")
	}

	_, err := r.db.Exec(ctx,
		`UPDATE resumes SET parsed_content = $1, structured_data = $2, skills = $3, status = $4, updated_at = NOW()
		 WHERE id = $5`,
		parsedContent, structuredJSON, skillsJSON, status, id,
	)
	return err
}
