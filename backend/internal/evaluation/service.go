package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apperrors "ai-interview-platform/pkg/errors"

	"ai-interview-platform/internal/agent"
	"ai-interview-platform/internal/interview"
	"ai-interview-platform/internal/task"
)

// taskEnqueuer 任务入队能力（*task.Repository 实现；测试可替换）
type taskEnqueuer interface {
	Enqueue(ctx context.Context, req task.EnqueueRequest) (*task.Task, bool, error)
}

// Service 评估业务逻辑层
type Service struct {
	repo          *Repository
	interviewRepo *interview.Repository
	agent         *agent.Agent
	tasks         taskEnqueuer
	log           *slog.Logger
}

// NewService 创建评估 Service
func NewService(repo *Repository, interviewRepo *interview.Repository, agentSvc *agent.Agent, tasks taskEnqueuer, log *slog.Logger) *Service {
	return &Service{
		repo:          repo,
		interviewRepo: interviewRepo,
		agent:         agentSvc,
		tasks:         tasks,
		log:           log,
	}
}

// RequestEvaluate 提交评估异步任务（会话归属/状态前置校验 + 幂等入队）
func (s *Service) RequestEvaluate(ctx context.Context, userID, sessionID string) (*task.AcceptedView, error) {
	sess, err := s.interviewRepo.GetByID(ctx, sessionID)
	if err != nil {
		s.log.Error("get session failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询面试失败", 500, err)
	}
	if sess == nil {
		return nil, apperrors.New("INTERVIEW_SESSION_NOT_FOUND", "面试不存在", 404)
	}
	if sess.UserID != userID {
		return nil, apperrors.ErrForbidden
	}
	if sess.Status != interview.StatusCompleted {
		return nil, apperrors.New("SESSION_NOT_COMPLETED", "面试尚未完成，无法评估", 400)
	}

	t, _, err := s.tasks.Enqueue(ctx, task.EnqueueRequest{
		Type: task.TypeSessionEvaluation,
		Payload: task.SessionTaskPayload{
			UserID:    userID,
			SessionID: sessionID,
		},
		IdempotencyKey: "session_evaluation:" + sessionID,
	})
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "创建评估任务失败", 500, err)
	}
	return &task.AcceptedView{TaskID: t.ID, Status: t.Status}, nil
}

// RunEvaluationTask worker 任务处理器：解码负载后执行评估（Evaluate 内部仍做完整校验）
func (s *Service) RunEvaluationTask(ctx context.Context, raw json.RawMessage) error {
	var p task.SessionTaskPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("decode session_evaluation payload: %w", err)
	}
	_, err := s.Evaluate(ctx, p.UserID, p.SessionID)
	return err
}

// Evaluate 对已完成的面试执行评估（显式触发，可重跑覆盖）
// 流程：归属/状态校验 → 拉取问答 → Agent 维度评分（含证据）→ 确定性总分计算 → 落库
func (s *Service) Evaluate(ctx context.Context, userID, sessionID string) (*Evaluation, error) {
	if sessionID == "" {
		return nil, apperrors.ErrBadRequest
	}

	// 会话归属与状态校验
	sess, err := s.interviewRepo.GetByID(ctx, sessionID)
	if err != nil {
		s.log.Error("get session failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询面试失败", 500, err)
	}
	if sess == nil {
		return nil, apperrors.New("INTERVIEW_SESSION_NOT_FOUND", "面试不存在", 404)
	}
	if sess.UserID != userID {
		return nil, apperrors.ErrForbidden
	}
	if sess.Status != interview.StatusCompleted {
		return nil, apperrors.New("SESSION_NOT_COMPLETED",
			"面试尚未完成，无法评估", 400)
	}

	// 拉取整场问答（含回答与 AI 分析）
	questions, err := s.interviewRepo.ListQuestionsBySession(ctx, sessionID)
	if err != nil {
		s.log.Error("list questions failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询面试问答失败", 500, err)
	}

	var qa []agent.EvaluatedQA
	for _, q := range questions {
		if !q.Answered || q.Answer == nil {
			continue
		}
		qa = append(qa, agent.EvaluatedQA{
			Seq:            q.Seq,
			Question:       q.Question,
			QuestionType:   q.QuestionType,
			ExpectedPoints: q.ExpectedPoints,
			AnswerText:     q.Answer.TextContent,
			Analysis:       q.Answer.Analysis,
		})
	}
	if len(qa) == 0 {
		return nil, apperrors.New("NO_ANSWER_TO_EVALUATE", "该面试没有可评估的回答", 409)
	}

	// Agent 评估：产出维度分/证据/建议（失败可重跑，LLM 不算总分）
	output, err := s.agent.Evaluate(ctx, agent.EvaluateInput{
		JobTitle:      sess.JobTitle,
		InterviewType: sess.InterviewType,
		QA:            qa,
	})
	if err != nil {
		s.log.Error("agent evaluate failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("EVALUATION_FAILED", "生成评估失败，可稍后重试", 500, err)
	}

	// 确定性总分计算（维度分 × Rubric 权重）
	if !ValidateRubric(DefaultRubric) {
		s.log.Error("invalid default rubric")
		return nil, apperrors.New("INTERNAL_ERROR", "评分规则配置错误", 500)
	}
	total := CalculateTotalScore(output.Dimensions, DefaultRubric)

	evaluation := &Evaluation{
		SessionID:       sessionID,
		UserID:          userID,
		Dimensions:      output.Dimensions,
		Evidence:        output.Evidence,
		Recommendations: output.Recommendations,
		Rubric:          DefaultRubric,
		TotalScore:      total,
		PromptVersion:   output.PromptVersion,
		Model:           output.Model,
	}
	if err := s.repo.Upsert(ctx, evaluation); err != nil {
		s.log.Error("save evaluation failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "保存评估结果失败", 500, err)
	}

	s.log.Info("evaluation created", "session_id", sessionID, "total_score", total,
		"prompt_version", output.PromptVersion, "model", output.Model)
	return evaluation, nil
}

// Get 查询会话评估
func (s *Service) Get(ctx context.Context, userID, sessionID string) (*Evaluation, error) {
	if sessionID == "" {
		return nil, apperrors.ErrBadRequest
	}

	// 归属校验：评估跟随会话，跨用户返回 403
	sess, err := s.interviewRepo.GetByID(ctx, sessionID)
	if err != nil {
		s.log.Error("get session failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询面试失败", 500, err)
	}
	if sess == nil {
		return nil, apperrors.New("INTERVIEW_SESSION_NOT_FOUND", "面试不存在", 404)
	}
	if sess.UserID != userID {
		return nil, apperrors.ErrForbidden
	}

	e, err := s.repo.GetBySessionID(ctx, sessionID)
	if err != nil {
		s.log.Error("get evaluation failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询评估失败", 500, err)
	}
	if e == nil {
		return nil, apperrors.New("EVALUATION_NOT_FOUND", "该面试尚未评估", 404)
	}
	return e, nil
}

// List 查询当前用户的评估列表
func (s *Service) List(ctx context.Context, userID string) ([]Evaluation, error) {
	list, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		s.log.Error("list evaluations failed", "user_id", userID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询评估列表失败", 500, err)
	}
	return list, nil
}
