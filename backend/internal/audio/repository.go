package audio

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ai-interview-platform/internal/agent"
)

// Repository 语音数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建语音 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Upsert 写入语音记录（question_id 唯一，重传覆盖并重置转写/分析）
func (r *Repository) Upsert(ctx context.Context, a *Audio) error {
	a.ID = uuid.New().String()
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now

	return r.db.QueryRow(ctx,
		`INSERT INTO answer_audios (id, session_id, question_id, user_id, object_key, format, size_bytes, duration_ms,
		                            transcript, language, status, analysis, asr_provider, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '', '', $9, NULL, '', $10, $11)
		 ON CONFLICT (question_id) DO UPDATE SET
		   user_id = EXCLUDED.user_id,
		   object_key = EXCLUDED.object_key,
		   format = EXCLUDED.format,
		   size_bytes = EXCLUDED.size_bytes,
		   duration_ms = EXCLUDED.duration_ms,
		   transcript = '',
		   language = '',
		   status = EXCLUDED.status,
		   analysis = NULL,
		   asr_provider = '',
		   updated_at = EXCLUDED.updated_at
		 RETURNING id, created_at, updated_at`,
		a.ID, a.SessionID, a.QuestionID, a.UserID, a.ObjectKey, a.Format, a.SizeBytes, a.DurationMs,
		a.Status, a.CreatedAt, a.UpdatedAt,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

// GetByQuestionID 按题目查询语音
func (r *Repository) GetByQuestionID(ctx context.Context, questionID string) (*Audio, error) {
	var a Audio
	var analysisJSON *string

	err := r.db.QueryRow(ctx,
		`SELECT id, session_id, question_id, user_id, object_key, format, size_bytes, duration_ms,
		        transcript, language, status, analysis::text, asr_provider, created_at, updated_at
		 FROM answer_audios WHERE question_id = $1`, questionID,
	).Scan(&a.ID, &a.SessionID, &a.QuestionID, &a.UserID, &a.ObjectKey, &a.Format, &a.SizeBytes, &a.DurationMs,
		&a.Transcript, &a.Language, &a.Status, &analysisJSON, &a.ASRProvider, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if analysisJSON != nil && *analysisJSON != "" && *analysisJSON != "null" {
		var sa agent.SpeechAnalysis
		if err := json.Unmarshal([]byte(*analysisJSON), &sa); err == nil {
			a.Analysis = &sa
		}
	}
	return &a, nil
}

// UpdateTranscription 更新转写结果
func (r *Repository) UpdateTranscription(ctx context.Context, id, transcript, language, asrProvider string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE answer_audios SET transcript = $2, language = $3, status = $4, asr_provider = $5, updated_at = NOW()
		 WHERE id = $1`, id, transcript, language, StatusTranscribed, asrProvider)
	return err
}

// UpdateAnalysis 更新表达分析结果
func (r *Repository) UpdateAnalysis(ctx context.Context, id string, analysis any) error {
	data, err := json.Marshal(analysis)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`UPDATE answer_audios SET analysis = $2::jsonb, status = $3, updated_at = NOW()
		 WHERE id = $1`, id, string(data), StatusAnalyzed)
	return err
}
