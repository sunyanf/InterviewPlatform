package interview

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ai-interview-platform/internal/agent"
	"ai-interview-platform/internal/job"
	"ai-interview-platform/internal/resume"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/metrics"
)

// AgentService Agent 模块对 Interview 模块暴露的能力接口
// （模块间通过 Service 接口交互，见 docs/architecture/MODULE_BOUNDARIES.md）
type AgentService interface {
	PlanQuestions(ctx context.Context, in agent.PlanQuestionsInput) ([]agent.PlannedQuestion, error)
	AnalyzeAnswer(ctx context.Context, in agent.AnalyzeAnswerInput) (*agent.AnswerAnalysis, error)
	DecideFollowUp(ctx context.Context, in agent.FollowUpInput) (*agent.FollowUpDecision, error)
	OpeningMessage(ctx context.Context, in agent.OpeningInput) (string, error)
}

// KnowledgeSnippet 出题用的知识片段（RAG 检索召回）
type KnowledgeSnippet struct {
	Content string `json:"content"`
	Source  string `json:"source"`
}

// KnowledgeRetriever 知识检索能力接口（由 knowledge 模块实现，保持模块边界解耦）
type KnowledgeRetriever interface {
	// RetrieveForQuery 按查询检索知识片段，返回最多 topK 条
	RetrieveForQuery(ctx context.Context, query string, topK int) ([]KnowledgeSnippet, error)
}

// Service 面试业务逻辑层
type Service struct {
	repo      *Repository
	jobRepo   *job.Repository
	resumeSvc *resume.Service
	agent     AgentService
	knowledge KnowledgeRetriever // 可选，nil 时不出题不接知识库
	log       *slog.Logger
}

// NewService 创建面试 Service
func NewService(repo *Repository, jobRepo *job.Repository, resumeSvc *resume.Service, agentSvc AgentService, knowledge KnowledgeRetriever, log *slog.Logger) *Service {
	return &Service{
		repo:      repo,
		jobRepo:   jobRepo,
		resumeSvc: resumeSvc,
		agent:     agentSvc,
		knowledge: knowledge,
		log:       log,
	}
}

// allowedTransitions 状态机合法转换表（状态只能由业务代码控制，见 ADR-0004）
var allowedTransitions = map[string][]string{
	StatusInit:       {StatusReady, StatusRunning, StatusFailed},
	StatusReady:      {StatusRunning, StatusFailed},
	StatusRunning:    {StatusPaused, StatusFinishing, StatusCompleted, StatusFailed},
	StatusPaused:     {StatusRunning, StatusFailed},
	StatusFinishing:  {StatusEvaluating, StatusCompleted, StatusFailed},
	StatusEvaluating: {StatusCompleted, StatusFailed},
}

// canTransition 判断状态转换是否合法（纯函数，便于测试）
func canTransition(from, to string) bool {
	for _, t := range allowedTransitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// Create 创建面试会话
func (s *Service) Create(ctx context.Context, userID string, req CreateSessionRequest) (*Session, error) {
	// 校验岗位存在
	j, err := s.jobRepo.GetJobByID(ctx, req.JobID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询岗位失败", 500, err)
	}
	if j == nil {
		return nil, apperrors.New("JOB_NOT_FOUND", "岗位不存在", 404)
	}

	// 校验简历（可选，若提供必须属于当前用户且已解析）
	if req.ResumeID != "" {
		rs, err := s.resumeSvc.Get(ctx, req.ResumeID)
		if err != nil {
			return nil, err
		}
		if rs == nil {
			return nil, apperrors.New("RESUME_NOT_FOUND", "简历不存在", 404)
		}
		if rs.UserID != userID {
			return nil, apperrors.ErrForbidden
		}
	}

	// 校验类型与模式
	interviewType := req.InterviewType
	if interviewType == "" {
		interviewType = "technical"
	}
	if !validInterviewTypes[interviewType] {
		return nil, apperrors.New("INVALID_INTERVIEW_TYPE", "面试类型必须为 technical、behavioral 或 mixed", 400)
	}
	mode := req.Mode
	if mode == "" {
		mode = "text"
	}
	if !validModes[mode] {
		return nil, apperrors.New("INVALID_MODE", "面试模式必须为 text 或 voice", 400)
	}

	// 配置默认值
	questionCount := req.QuestionCount
	if questionCount < 1 || questionCount > 20 {
		questionCount = 5
	}
	durationMins := req.DurationMins
	if durationMins < 5 || durationMins > 120 {
		durationMins = 30
	}

	sess := &Session{
		UserID:        userID,
		JobID:         req.JobID,
		ResumeID:      req.ResumeID,
		InterviewType: interviewType,
		Mode:          mode,
		Config: SessionConfig{
			QuestionCount:   questionCount,
			DurationMinutes: durationMins,
		},
	}

	if err := s.repo.Create(ctx, sess); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "创建面试失败", 500, err)
	}

	return sess, nil
}

// Start 开始面试：状态机 INIT/READY → RUNNING，Agent 出题 + 面试官开场白
func (s *Service) Start(ctx context.Context, userID, sessionID string) (*Session, error) {
	sess, err := s.getOwnedSession(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	// 状态机校验：业务代码控制状态转换
	if !canTransition(sess.Status, StatusRunning) {
		return nil, apperrors.New("INVALID_STATE_TRANSITION",
			fmt.Sprintf("当前状态 %s 不允许开始面试", sess.Status), 400)
	}

	// 收集岗位信息
	j, err := s.jobRepo.GetJobByID(ctx, sess.JobID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询岗位失败", 500, err)
	}
	if j == nil {
		return nil, apperrors.New("JOB_NOT_FOUND", "岗位不存在", 404)
	}

	// 收集简历技能（可选）
	resumeSkills := []string{}
	if sess.ResumeID != "" {
		rs, err := s.resumeSvc.Get(ctx, sess.ResumeID)
		if err == nil && rs != nil && len(rs.Skills) > 0 {
			resumeSkills = rs.Skills
		}
	}

	// RAG 知识检索（尽力而为：失败/超时降级为无知识出题，不阻塞开考）
	var knowledgeRefs []string
	if s.knowledge != nil {
		knowledgeQuery := strings.Join(append([]string{j.Title}, j.Skills...), " ")
		// 检索设置超时：知识检索不应无限阻塞开考
		retrievalCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		snippets, err := s.knowledge.RetrieveForQuery(retrievalCtx, knowledgeQuery, 5)
		cancel()
		if err != nil {
			s.log.Warn("knowledge retrieval failed", "session_id", sessionID, "error", err)
		} else {
			for _, sn := range snippets {
				knowledgeRefs = append(knowledgeRefs, sn.Content)
			}
			s.log.Info("knowledge retrieved for session", "session_id", sessionID, "snippets", len(knowledgeRefs))
		}
	}

	// Agent 出题规划（LLM 只生成内容，不决定状态）
	planned, err := s.agent.PlanQuestions(ctx, agent.PlanQuestionsInput{
		JobTitle:        j.Title,
		JobSkills:       j.Skills,
		JobRequirements: j.Requirements,
		ResumeSkills:    resumeSkills,
		InterviewType:   sess.InterviewType,
		Count:           sess.Config.QuestionCount,
		Knowledge:       knowledgeRefs,
	})
	if err != nil {
		metrics.AgentFailures.Inc("plan_questions")
		s.log.Error("plan questions failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("QUESTION_GENERATION_FAILED", "生成面试问题失败", 500, err)
	}
	if len(planned) == 0 {
		return nil, apperrors.New("NO_QUESTIONS_GENERATED", "未能生成面试问题", 500)
	}

	questions := make([]Question, len(planned))
	for i, p := range planned {
		questions[i] = Question{
			Seq:            i + 1,
			QuestionType:   p.Type,
			Question:       p.Question,
			Difficulty:     p.Difficulty,
			Source:         "llm",
			ExpectedPoints: p.ExpectedPoints,
		}
	}

	if err := s.repo.CreateQuestions(ctx, sess.ID, questions); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "保存面试问题失败", 500, err)
	}

	// Agent 面试官开场白（尽力而为，失败不阻塞开考）
	opening, err := s.agent.OpeningMessage(ctx, agent.OpeningInput{
		JobTitle:      j.Title,
		InterviewType: sess.InterviewType,
		FirstQuestion: planned[0].Question,
	})
	if err != nil {
		metrics.AgentFailures.Inc("opening")
		s.log.Warn("opening message failed", "session_id", sessionID, "error", err)
	} else if err := s.repo.UpdateSessionMetadata(ctx, sess.ID, map[string]interface{}{
		"opening_message": opening,
	}); err != nil {
		s.log.Warn("save opening message failed", "session_id", sessionID, "error", err)
	} else {
		sess.Metadata = map[string]interface{}{"opening_message": opening}
	}

	now := time.Now()
	if err := s.repo.UpdateStatus(ctx, sess.ID, StatusRunning, &now, nil); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "更新面试状态失败", 500, err)
	}

	sess.Status = StatusRunning
	sess.StartedAt = &now
	sess.Questions = questions
	return sess, nil
}

// Get 查询面试详情（含问题、回答与分析）
func (s *Service) Get(ctx context.Context, userID, sessionID string) (*Session, error) {
	sess, err := s.getOwnedSession(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	title, err := s.repo.GetJobTitle(ctx, sess.JobID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询岗位失败", 500, err)
	}
	sess.JobTitle = title

	questions, err := s.repo.ListQuestionsBySession(ctx, sess.ID)
	if err != nil {
		s.log.Error("list questions failed", "session_id", sess.ID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询面试问题失败", 500, err)
	}
	if questions == nil {
		questions = []Question{}
	}
	sess.Questions = questions

	answered, err := s.repo.CountAnswered(ctx, sess.ID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "统计回答数失败", 500, err)
	}
	sess.AnsweredCount = answered

	return sess, nil
}

// List 查询用户的面试列表
func (s *Service) List(ctx context.Context, userID string) ([]Session, error) {
	list, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询面试列表失败", 500, err)
	}
	if list == nil {
		list = []Session{}
	}
	return list, nil
}

// SubmitAnswer 提交回答：保存回答 → Agent 分析 → Agent 追问决策
// Agent 只产出内容，是否保存分析/创建追问由业务代码决定
func (s *Service) SubmitAnswer(ctx context.Context, userID, sessionID string, req SubmitAnswerRequest) (*SubmitAnswerResult, error) {
	sess, err := s.getOwnedSession(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	// 只有进行中的会话才能回答
	if sess.Status != StatusRunning {
		return nil, apperrors.New("INVALID_STATE_TRANSITION",
			fmt.Sprintf("当前状态 %s 不允许提交回答", sess.Status), 400)
	}

	if req.QuestionID == "" {
		return nil, apperrors.New("MISSING_QUESTION_ID", "请提供问题 ID", 400)
	}
	if strings.TrimSpace(req.TextContent) == "" {
		return nil, apperrors.New("EMPTY_ANSWER", "回答内容不能为空", 400)
	}

	// 问题必须属于该会话
	q, err := s.repo.GetQuestionByID(ctx, req.QuestionID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询问题失败", 500, err)
	}
	if q == nil || q.SessionID != sess.ID {
		return nil, apperrors.New("QUESTION_NOT_IN_SESSION", "问题不属于当前面试", 400)
	}

	answer := &Answer{
		SessionID:   sess.ID,
		QuestionID:  q.ID,
		InputType:   "text",
		TextContent: req.TextContent,
		DurationMs:  req.DurationMs,
	}
	if err := s.repo.CreateAnswer(ctx, answer); err != nil {
		if errors.Is(err, ErrDuplicateAnswer) {
			return nil, apperrors.New("ALREADY_ANSWERED", "该问题已回答过", 409)
		}
		return nil, apperrors.Wrap("INTERNAL_ERROR", "保存回答失败", 500, err)
	}

	// Agent 回答分析（尽力而为：回答已保存，分析失败/超时不阻塞提交）
	// 单独加超时，避免模型长时间无响应时拖住整条提交流程
	var analysis *agent.AnswerAnalysis
	analyzeCtx, analyzeCancel := context.WithTimeout(ctx, analyzeAnswerTimeout)
	analysis, err = s.agent.AnalyzeAnswer(analyzeCtx, agent.AnalyzeAnswerInput{
		Question:       q.Question,
		ExpectedPoints: q.ExpectedPoints,
		AnswerText:     req.TextContent,
		Difficulty:     q.Difficulty,
	})
	analyzeCancel()
	if err != nil {
		metrics.AgentFailures.Inc("analyze_answer")
		s.log.Warn("analyze answer failed", "session_id", sess.ID, "question_id", q.ID, "error", err)
	} else if err := s.repo.UpdateAnswerAnalysis(ctx, answer.ID, analysis); err != nil {
		s.log.Warn("update answer analysis failed", "answer_id", answer.ID, "error", err)
	}
	answer.Analysis = analysis

	// Agent 追问决策（仅普通问题可追问，避免追问链无限延伸）
	var followUp *Question
	if analysis != nil && q.QuestionType != QTypeFollowUp {
		followUpCtx, followUpCancel := context.WithTimeout(ctx, decideFollowUpTimeout)
		decision, err := s.agent.DecideFollowUp(followUpCtx, agent.FollowUpInput{
			Question:       q.Question,
			ExpectedPoints: q.ExpectedPoints,
			AnswerText:     req.TextContent,
			Analysis:       analysis,
		})
		followUpCancel()
		if err != nil {
			metrics.AgentFailures.Inc("decide_followup")
			s.log.Warn("decide follow-up failed", "session_id", sess.ID, "error", err)
		} else if decision.ShouldFollowUp {
			// 业务代码决定创建追问问题
			maxSeq, err := s.repo.GetMaxSeq(ctx, sess.ID)
			if err != nil {
				s.log.Warn("get max seq failed", "session_id", sess.ID, "error", err)
			} else {
				fuQuestions := []Question{{
					Seq:            maxSeq + 1,
					QuestionType:   QTypeFollowUp,
					Question:       decision.Question,
					Difficulty:     q.Difficulty,
					Source:         "llm",
					ExpectedPoints: []string{},
					Metadata: map[string]interface{}{
						"parent_question_id": q.ID,
						"target_gap":         decision.TargetGap,
					},
				}}
				if err := s.repo.CreateQuestions(ctx, sess.ID, fuQuestions); err != nil {
					s.log.Warn("create follow-up question failed", "session_id", sess.ID, "error", err)
				} else {
					followUp = &fuQuestions[0]
				}
			}
		}
	}

	return &SubmitAnswerResult{
		Answer:   answer,
		Analysis: analysis,
		FollowUp: followUp,
	}, nil
}

// Finish 结束面试：状态机 RUNNING → COMPLETED
func (s *Service) Finish(ctx context.Context, userID, sessionID string) (*Session, error) {
	sess, err := s.getOwnedSession(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	if !canTransition(sess.Status, StatusCompleted) {
		return nil, apperrors.New("INVALID_STATE_TRANSITION",
			fmt.Sprintf("当前状态 %s 不允许结束面试", sess.Status), 400)
	}

	now := time.Now()
	if err := s.repo.UpdateStatus(ctx, sess.ID, StatusCompleted, nil, &now); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "更新面试状态失败", 500, err)
	}

	sess.Status = StatusCompleted
	sess.EndedAt = &now
	metrics.InterviewsCompleted.Inc()
	return sess, nil
}

// getOwnedSession 查询会话并校验归属
func (s *Service) getOwnedSession(ctx context.Context, userID, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, apperrors.ErrBadRequest
	}

	sess, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询面试失败", 500, err)
	}
	if sess == nil {
		return nil, apperrors.New("INTERVIEW_SESSION_NOT_FOUND", "面试不存在", 404)
	}
	if sess.UserID != userID {
		return nil, apperrors.ErrForbidden
	}
	return sess, nil
}
