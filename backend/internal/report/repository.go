package report

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository 报告数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建报告 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Upsert 写入报告（session_id 唯一，重跑覆盖）
func (r *Repository) Upsert(ctx context.Context, rp *Report) error {
	rp.ID = uuid.New().String()
	now := time.Now()
	rp.CreatedAt = now
	rp.UpdatedAt = now

	profileJSON, _ := json.Marshal(rp.CapabilityProfile)
	strengthsJSON := marshalListOrEmpty(rp.Strengths)
	weaknessesJSON := marshalListOrEmpty(rp.Weaknesses)
	gapsJSON := marshalListOrEmpty(rp.KnowledgeGaps)
	planJSON, _ := json.Marshal(rp.LearningPlan)

	return r.db.QueryRow(ctx,
		`INSERT INTO reports (id, session_id, user_id, total_score, capability_profile, strengths, weaknesses, knowledge_gaps, learning_plan, prompt_version, model, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10, $11, $12, $13)
		 ON CONFLICT (session_id) DO UPDATE SET
		   user_id = EXCLUDED.user_id,
		   total_score = EXCLUDED.total_score,
		   capability_profile = EXCLUDED.capability_profile,
		   strengths = EXCLUDED.strengths,
		   weaknesses = EXCLUDED.weaknesses,
		   knowledge_gaps = EXCLUDED.knowledge_gaps,
		   learning_plan = EXCLUDED.learning_plan,
		   prompt_version = EXCLUDED.prompt_version,
		   model = EXCLUDED.model,
		   updated_at = EXCLUDED.updated_at
		 RETURNING id, created_at, updated_at`,
		rp.ID, rp.SessionID, rp.UserID, rp.TotalScore, string(profileJSON),
		strengthsJSON, weaknessesJSON, gapsJSON, string(planJSON),
		rp.PromptVersion, rp.Model, rp.CreatedAt, rp.UpdatedAt,
	).Scan(&rp.ID, &rp.CreatedAt, &rp.UpdatedAt)
}

// GetBySessionID 按会话查询报告
func (r *Repository) GetBySessionID(ctx context.Context, sessionID string) (*Report, error) {
	var rp Report
	var profileJSON, strengthsJSON, weaknessesJSON, gapsJSON, planJSON string

	err := r.db.QueryRow(ctx,
		`SELECT id, session_id, user_id, total_score, capability_profile::text, strengths::text, weaknesses::text,
		        knowledge_gaps::text, learning_plan::text, prompt_version, model, created_at, updated_at
		 FROM reports WHERE session_id = $1`, sessionID,
	).Scan(&rp.ID, &rp.SessionID, &rp.UserID, &rp.TotalScore, &profileJSON, &strengthsJSON, &weaknessesJSON,
		&gapsJSON, &planJSON, &rp.PromptVersion, &rp.Model, &rp.CreatedAt, &rp.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(profileJSON), &rp.CapabilityProfile)
	_ = json.Unmarshal([]byte(strengthsJSON), &rp.Strengths)
	_ = json.Unmarshal([]byte(weaknessesJSON), &rp.Weaknesses)
	_ = json.Unmarshal([]byte(gapsJSON), &rp.KnowledgeGaps)
	_ = json.Unmarshal([]byte(planJSON), &rp.LearningPlan)
	return &rp, nil
}

// ListByUserID 查询用户的报告列表
func (r *Repository) ListByUserID(ctx context.Context, userID string) ([]Report, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, session_id, user_id, total_score, capability_profile::text, strengths::text, weaknesses::text,
		        knowledge_gaps::text, learning_plan::text, prompt_version, model, created_at, updated_at
		 FROM reports WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Report
	for rows.Next() {
		var rp Report
		var profileJSON, strengthsJSON, weaknessesJSON, gapsJSON, planJSON string
		if err := rows.Scan(&rp.ID, &rp.SessionID, &rp.UserID, &rp.TotalScore, &profileJSON, &strengthsJSON,
			&weaknessesJSON, &gapsJSON, &planJSON, &rp.PromptVersion, &rp.Model, &rp.CreatedAt, &rp.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(profileJSON), &rp.CapabilityProfile)
		_ = json.Unmarshal([]byte(strengthsJSON), &rp.Strengths)
		_ = json.Unmarshal([]byte(weaknessesJSON), &rp.Weaknesses)
		_ = json.Unmarshal([]byte(gapsJSON), &rp.KnowledgeGaps)
		_ = json.Unmarshal([]byte(planJSON), &rp.LearningPlan)
		list = append(list, rp)
	}
	return list, rows.Err()
}

// marshalListOrEmpty 序列化字符串列表，nil 时序列化为 []
func marshalListOrEmpty(items []string) string {
	if items == nil {
		return "[]"
	}
	data, _ := json.Marshal(items)
	return string(data)
}
