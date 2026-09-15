package report

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"ai-interview-platform/internal/agent"
	"ai-interview-platform/internal/evaluation"
	"ai-interview-platform/internal/interview"
	"ai-interview-platform/internal/task"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/metrics"
)

// taskEnqueuer 任务入队能力（*task.Repository 实现；测试可替换）
type taskEnqueuer interface {
	Enqueue(ctx context.Context, req task.EnqueueRequest) (*task.Task, bool, error)
}

// Service 报告业务逻辑层
type Service struct {
	repo          *Repository
	evalRepo      *evaluation.Repository
	interviewRepo *interview.Repository
	agent         *agent.Agent
	tasks         taskEnqueuer
	log           *slog.Logger
}

// NewService 创建报告 Service
func NewService(repo *Repository, evalRepo *evaluation.Repository, interviewRepo *interview.Repository, agentSvc *agent.Agent, tasks taskEnqueuer, log *slog.Logger) *Service {
	return &Service{
		repo:          repo,
		evalRepo:      evalRepo,
		interviewRepo: interviewRepo,
		agent:         agentSvc,
		tasks:         tasks,
		log:           log,
	}
}

// RequestGenerate 提交报告生成异步任务（会话归属/状态/评估前置校验 + 幂等入队）
func (s *Service) RequestGenerate(ctx context.Context, userID, sessionID string) (*task.AcceptedView, error) {
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
		return nil, apperrors.New("SESSION_NOT_COMPLETED", "面试尚未完成，无法生成报告", 400)
	}

	eval, err := s.evalRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		s.log.Error("get evaluation failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询评估失败", 500, err)
	}
	if eval == nil {
		return nil, apperrors.New("EVALUATION_REQUIRED", "该面试尚未评估，请先完成评估", 404)
	}

	t, _, err := s.tasks.Enqueue(ctx, task.EnqueueRequest{
		Type: task.TypeReportGeneration,
		Payload: task.SessionTaskPayload{
			UserID:    userID,
			SessionID: sessionID,
		},
		IdempotencyKey: "report_generation:" + sessionID,
	})
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "创建报告任务失败", 500, err)
	}
	return &task.AcceptedView{TaskID: t.ID, Status: t.Status}, nil
}

// RunGenerateTask worker 任务处理器：解码负载后生成报告（Generate 内部仍做完整校验）
func (s *Service) RunGenerateTask(ctx context.Context, raw json.RawMessage) error {
	var p task.SessionTaskPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("decode report_generation payload: %w", err)
	}
	_, err := s.Generate(ctx, p.UserID, p.SessionID)
	return err
}

// Generate 生成面试报告（显式触发，可重跑覆盖；前置条件：评估已完成）
// 数据流：评估（确定性总分/维度分）→ 能力画像 + 历史对比（确定性计算）
//
//	→ 知识缺口聚合（确定性）→ LLM 学习计划（只产建议，不算分）→ 落库
func (s *Service) Generate(ctx context.Context, userID, sessionID string) (*Report, error) {
	if sessionID == "" {
		return nil, apperrors.New("BAD_REQUEST", "会话 ID 不能为空", 400)
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
		return nil, apperrors.New("SESSION_NOT_COMPLETED", "面试尚未完成，无法生成报告", 400)
	}

	// 评估必须已存在：报告基于评估结果，不重复评估
	eval, err := s.evalRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		s.log.Error("get evaluation failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询评估失败", 500, err)
	}
	if eval == nil {
		return nil, apperrors.New("EVALUATION_REQUIRED", "该面试尚未评估，请先完成评估", 404)
	}

	// 能力画像 + 历史对比（确定性）
	histories, err := s.evalRepo.ListByUserAndType(ctx, userID, sess.InterviewType, sessionID, historyWindow)
	if err != nil {
		s.log.Error("list history evaluations failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询历史评估失败", 500, err)
	}
	history := BuildHistoryComparison(*eval, histories)

	// 知识缺口聚合（确定性，来自已存储的回答分析）
	questions, err := s.interviewRepo.ListQuestionsBySession(ctx, sessionID)
	if err != nil {
		s.log.Error("list questions failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询面试问答失败", 500, err)
	}
	var views []*analysisView
	for _, q := range questions {
		if q.Answer == nil || q.Answer.Analysis == nil {
			continue
		}
		views = append(views, &analysisView{KnowledgeGaps: q.Answer.Analysis.KnowledgeGaps})
	}
	knowledgeGaps := AggregateKnowledgeGaps(views)

	// LLM 学习计划（只产建议内容；失败可重跑）
	planOutput, err := s.agent.LearningPlan(ctx, agent.LearningPlannerInput{
		JobTitle:            sess.JobTitle,
		InterviewType:       sess.InterviewType,
		Dimensions:          eval.Dimensions,
		Evidence:            eval.Evidence,
		KnowledgeGaps:       knowledgeGaps,
		EvalRecommendations: eval.Recommendations,
		HistorySummary:      historySummary(history),
	})
	if err != nil {
		metrics.AgentFailures.Inc("learning_plan")
		s.log.Error("agent learning plan failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("REPORT_FAILED", "生成学习计划失败，可稍后重试", 500, err)
	}

	rp := &Report{
		SessionID:  sessionID,
		UserID:     userID,
		TotalScore: eval.TotalScore,
		CapabilityProfile: CapabilityProfile{
			Dimensions: eval.Dimensions,
			TotalScore: eval.TotalScore,
			History:    history,
		},
		Strengths:     planOutput.Strengths,
		Weaknesses:    planOutput.Weaknesses,
		KnowledgeGaps: knowledgeGaps,
		LearningPlan: LearningPlan{
			FocusAreas:   planOutput.FocusAreas,
			NextTraining: planOutput.NextTraining,
		},
		PromptVersion: planOutput.PromptVersion,
		Model:         planOutput.Model,
	}
	if err := s.repo.Upsert(ctx, rp); err != nil {
		s.log.Error("save report failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "保存报告失败", 500, err)
	}

	s.log.Info("report generated", "session_id", sessionID, "total_score", rp.TotalScore,
		"history_count", historyCount(history), "prompt_version", rp.PromptVersion)
	return rp, nil
}

// Get 查询会话报告
func (s *Service) Get(ctx context.Context, userID, sessionID string) (*Report, error) {
	if sessionID == "" {
		return nil, apperrors.New("BAD_REQUEST", "会话 ID 不能为空", 400)
	}

	// 归属校验：报告跟随会话，跨用户返回 403
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

	rp, err := s.repo.GetBySessionID(ctx, sessionID)
	if err != nil {
		s.log.Error("get report failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询报告失败", 500, err)
	}
	if rp == nil {
		return nil, apperrors.New("REPORT_NOT_FOUND", "该面试尚未生成报告", 404)
	}
	return rp, nil
}

// List 查询当前用户的报告列表
func (s *Service) List(ctx context.Context, userID string) ([]Report, error) {
	list, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		s.log.Error("list reports failed", "user_id", userID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询报告列表失败", 500, err)
	}
	return list, nil
}

// historySummary 历史对比摘要（供 LLM prompt 使用，确定性文本）
func historySummary(h *HistoryComparison) string {
	if h == nil {
		return ""
	}
	return fmt.Sprintf("近 %d 场同类型面试平均总分 %.2f，本次 %.2f（%.2f）",
		h.ComparedCount, h.AvgTotalScore, h.AvgTotalScore+h.DeltaTotalScore, h.DeltaTotalScore)
}

// historyCount 历史对比场次数（nil 安全）
func historyCount(h *HistoryComparison) int {
	if h == nil {
		return 0
	}
	return h.ComparedCount
}
