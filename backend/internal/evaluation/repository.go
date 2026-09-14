package evaluation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository 评估数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建评估 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Upsert 写入评估结果（session_id 唯一，重跑覆盖）
func (r *Repository) Upsert(ctx context.Context, e *Evaluation) error {
	e.ID = uuid.New().String()
	now := time.Now()
	e.CreatedAt = now
	e.UpdatedAt = now

	dimensionsJSON, _ := json.Marshal(e.Dimensions)
	evidenceJSON, _ := json.Marshal(e.Evidence)
	if e.Evidence == nil {
		evidenceJSON = []byte("[]")
	}
	recsJSON, _ := json.Marshal(e.Recommendations)
	if e.Recommendations == nil {
		recsJSON = []byte("[]")
	}
	rubricJSON, _ := json.Marshal(e.Rubric)

	return r.db.QueryRow(ctx,
		`INSERT INTO evaluations (id, session_id, user_id, dimensions, evidence, recommendations, rubric, total_score, prompt_version, model, created_at, updated_at)
		 VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8, $9, $10, $11, $12)
		 ON CONFLICT (session_id) DO UPDATE SET
		   user_id = EXCLUDED.user_id,
		   dimensions = EXCLUDED.dimensions,
		   evidence = EXCLUDED.evidence,
		   recommendations = EXCLUDED.recommendations,
		   rubric = EXCLUDED.rubric,
		   total_score = EXCLUDED.total_score,
		   prompt_version = EXCLUDED.prompt_version,
		   model = EXCLUDED.model,
		   updated_at = EXCLUDED.updated_at
		 RETURNING id, created_at, updated_at`,
		e.ID, e.SessionID, e.UserID, string(dimensionsJSON), string(evidenceJSON), string(recsJSON),
		string(rubricJSON), e.TotalScore, e.PromptVersion, e.Model, e.CreatedAt, e.UpdatedAt,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

// GetBySessionID 按会话查询评估
func (r *Repository) GetBySessionID(ctx context.Context, sessionID string) (*Evaluation, error) {
	var e Evaluation
	var dimensionsJSON, evidenceJSON, recsJSON, rubricJSON string

	err := r.db.QueryRow(ctx,
		`SELECT id, session_id, user_id, dimensions::text, evidence::text, recommendations::text, rubric::text,
		        total_score, prompt_version, model, created_at, updated_at
		 FROM evaluations WHERE session_id = $1`, sessionID,
	).Scan(&e.ID, &e.SessionID, &e.UserID, &dimensionsJSON, &evidenceJSON, &recsJSON, &rubricJSON,
		&e.TotalScore, &e.PromptVersion, &e.Model, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(dimensionsJSON), &e.Dimensions)
	_ = json.Unmarshal([]byte(evidenceJSON), &e.Evidence)
	_ = json.Unmarshal([]byte(recsJSON), &e.Recommendations)
	_ = json.Unmarshal([]byte(rubricJSON), &e.Rubric)
	return &e, nil
}

// ListByUserID 查询用户的评估列表
func (r *Repository) ListByUserID(ctx context.Context, userID string) ([]Evaluation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, session_id, user_id, dimensions::text, evidence::text, recommendations::text, rubric::text,
		        total_score, prompt_version, model, created_at, updated_at
		 FROM evaluations WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Evaluation
	for rows.Next() {
		var e Evaluation
		var dimensionsJSON, evidenceJSON, recsJSON, rubricJSON string
		if err := rows.Scan(&e.ID, &e.SessionID, &e.UserID, &dimensionsJSON, &evidenceJSON, &recsJSON, &rubricJSON,
			&e.TotalScore, &e.PromptVersion, &e.Model, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(dimensionsJSON), &e.Dimensions)
		_ = json.Unmarshal([]byte(evidenceJSON), &e.Evidence)
		_ = json.Unmarshal([]byte(recsJSON), &e.Recommendations)
		_ = json.Unmarshal([]byte(rubricJSON), &e.Rubric)
		list = append(list, e)
	}
	return list, rows.Err()
}
