package interview

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDuplicateAnswer 该问题已回答过
var ErrDuplicateAnswer = errors.New("duplicate answer")

// Repository 面试数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建面试 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create 创建面试会话
func (r *Repository) Create(ctx context.Context, s *Session) error {
	s.ID = uuid.New().String()
	s.Status = StatusInit
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now

	configJSON, _ := json.Marshal(s.Config)

	_, err := r.db.Exec(ctx,
		`INSERT INTO interview_sessions (id, user_id, job_id, resume_id, interview_type, mode, status, config, created_at, updated_at)
		 VALUES ($1, $2, $3, NULLIF($4,''), $5, $6, $7, $8, $9, $10)`,
		s.ID, s.UserID, s.JobID, s.ResumeID, s.InterviewType, s.Mode, s.Status, configJSON, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

// GetByID 根据 ID 查询会话
func (r *Repository) GetByID(ctx context.Context, id string) (*Session, error) {
	var s Session
	var configJSON, metadataJSON string

	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, job_id, COALESCE(resume_id,''), interview_type, mode, status, config::text, metadata::text,
		        started_at, ended_at, created_at, updated_at
		 FROM interview_sessions WHERE id = $1`, id,
	).Scan(&s.ID, &s.UserID, &s.JobID, &s.ResumeID, &s.InterviewType, &s.Mode, &s.Status, &configJSON, &metadataJSON,
		&s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(configJSON), &s.Config)
	_ = json.Unmarshal([]byte(metadataJSON), &s.Metadata)
	return &s, nil
}

// ListByUserID 查询用户的面试列表
func (r *Repository) ListByUserID(ctx context.Context, userID string) ([]Session, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, job_id, COALESCE(resume_id,''), interview_type, mode, status, config::text, metadata::text,
		        started_at, ended_at, created_at, updated_at
		 FROM interview_sessions WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Session
	for rows.Next() {
		var s Session
		var configJSON, metadataJSON string
		if err := rows.Scan(&s.ID, &s.UserID, &s.JobID, &s.ResumeID, &s.InterviewType, &s.Mode, &s.Status, &configJSON, &metadataJSON,
			&s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(configJSON), &s.Config)
		_ = json.Unmarshal([]byte(metadataJSON), &s.Metadata)
		list = append(list, s)
	}
	return list, rows.Err()
}

// UpdateStatus 更新会话状态（状态机唯一入口）
func (r *Repository) UpdateStatus(ctx context.Context, id, status string, startedAt, endedAt *time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE interview_sessions SET status = $1,
		        started_at = COALESCE($2, started_at),
		        ended_at = COALESCE($3, ended_at),
		        updated_at = NOW()
		 WHERE id = $4`,
		status, startedAt, endedAt, id,
	)
	return err
}

// SetJobTitle 填充岗位标题（查询辅助）
func (r *Repository) GetJobTitle(ctx context.Context, jobID string) (string, error) {
	var title string
	err := r.db.QueryRow(ctx, `SELECT title FROM jobs WHERE id = $1`, jobID).Scan(&title)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return title, nil
}

// CreateQuestions 批量创建问题
func (r *Repository) CreateQuestions(ctx context.Context, sessionID string, questions []Question) error {
	batch := &pgx.Batch{}
	for i := range questions {
		q := &questions[i]
		q.ID = uuid.New().String()
		q.SessionID = sessionID
		if q.CreatedAt.IsZero() {
			q.CreatedAt = time.Now()
		}
		expectedJSON, _ := json.Marshal(q.ExpectedPoints)
		if q.ExpectedPoints == nil {
			expectedJSON = []byte("[]")
		}
		metadataJSON, _ := json.Marshal(q.Metadata)
		if q.Metadata == nil {
			metadataJSON = []byte("{}")
		}
		batch.Queue(
			`INSERT INTO interview_questions (id, session_id, seq, question_type, question, difficulty, source, expected_points, metadata, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			q.ID, q.SessionID, q.Seq, q.QuestionType, q.Question, q.Difficulty, q.Source, expectedJSON, metadataJSON, q.CreatedAt,
		)
	}
	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for range questions {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// GetMaxSeq 查询会话当前最大问题序号
func (r *Repository) GetMaxSeq(ctx context.Context, sessionID string) (int, error) {
	var maxSeq int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM interview_questions WHERE session_id = $1`, sessionID,
	).Scan(&maxSeq)
	return maxSeq, err
}

// UpdateSessionMetadata 更新会话元数据（如开场白）
func (r *Repository) UpdateSessionMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	metadataJSON, _ := json.Marshal(metadata)
	_, err := r.db.Exec(ctx,
		`UPDATE interview_sessions SET metadata = $1, updated_at = NOW() WHERE id = $2`,
		metadataJSON, id,
	)
	return err
}

// UpdateAnswerAnalysis 更新回答的 AI 分析结果
func (r *Repository) UpdateAnswerAnalysis(ctx context.Context, answerID string, analysis any) error {
	analysisJSON, _ := json.Marshal(analysis)
	_, err := r.db.Exec(ctx,
		`UPDATE interview_answers SET analysis = $1 WHERE id = $2`,
		analysisJSON, answerID,
	)
	return err
}

// ListQuestionsBySession 查询会话的所有问题（含回答状态）
func (r *Repository) ListQuestionsBySession(ctx context.Context, sessionID string) ([]Question, error) {
	rows, err := r.db.Query(ctx,
		`SELECT q.id, q.session_id, q.seq, q.question_type, q.question, q.difficulty, q.source, q.expected_points::text, q.metadata::text, q.created_at,
		        a.id, a.input_type, a.text_content, COALESCE(a.duration_ms,0), COALESCE(a.analysis::text,'{}'), a.created_at
		 FROM interview_questions q
		 LEFT JOIN interview_answers a ON a.question_id = q.id
		 WHERE q.session_id = $1 ORDER BY q.seq`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Question
	for rows.Next() {
		var q Question
		var expectedJSON, metadataJSON string
		var answerID, inputType, textContent *string
		var answerDurationMs int
		var answerAnalysisJSON string
		var answerCreatedAt *time.Time

		if err := rows.Scan(&q.ID, &q.SessionID, &q.Seq, &q.QuestionType, &q.Question, &q.Difficulty,
			&q.Source, &expectedJSON, &metadataJSON, &q.CreatedAt,
			&answerID, &inputType, &textContent, &answerDurationMs, &answerAnalysisJSON, &answerCreatedAt); err != nil {
			return nil, err
		}

		_ = json.Unmarshal([]byte(expectedJSON), &q.ExpectedPoints)
		_ = json.Unmarshal([]byte(metadataJSON), &q.Metadata)

		if answerID != nil {
			q.Answered = true
			q.AnswerID = *answerID
			q.Answer = &Answer{
				ID:          *answerID,
				SessionID:   sessionID,
				QuestionID:  q.ID,
				InputType:   derefOr(inputType, "text"),
				TextContent: derefOr(textContent, ""),
				DurationMs:  answerDurationMs,
				CreatedAt:   *answerCreatedAt,
			}
			_ = json.Unmarshal([]byte(answerAnalysisJSON), &q.Answer.Analysis)
		}
		list = append(list, q)
	}
	return list, rows.Err()
}

// GetQuestionByID 查询单个问题
func (r *Repository) GetQuestionByID(ctx context.Context, id string) (*Question, error) {
	var q Question
	var expectedJSON, metadataJSON string

	err := r.db.QueryRow(ctx,
		`SELECT id, session_id, seq, question_type, question, difficulty, source, expected_points::text, metadata::text, created_at
		 FROM interview_questions WHERE id = $1`, id,
	).Scan(&q.ID, &q.SessionID, &q.Seq, &q.QuestionType, &q.Question, &q.Difficulty,
		&q.Source, &expectedJSON, &metadataJSON, &q.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(expectedJSON), &q.ExpectedPoints)
	_ = json.Unmarshal([]byte(metadataJSON), &q.Metadata)
	return &q, nil
}

// CreateAnswer 创建回答
func (r *Repository) CreateAnswer(ctx context.Context, a *Answer) error {
	a.ID = uuid.New().String()
	a.CreatedAt = time.Now()

	if err := r.db.QueryRow(ctx,
		`INSERT INTO interview_answers (id, session_id, question_id, input_type, text_content, duration_ms, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at`,
		a.ID, a.SessionID, a.QuestionID, a.InputType, a.TextContent, a.DurationMs, a.CreatedAt,
	).Scan(&a.CreatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateAnswer
		}
		return err
	}
	return nil
}

// CountAnswered 统计会话已回答问题数
func (r *Repository) CountAnswered(ctx context.Context, sessionID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM interview_answers WHERE session_id = $1`, sessionID,
	).Scan(&count)
	return count, err
}

func derefOr(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}
