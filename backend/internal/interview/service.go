package interview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ai-interview-platform/internal/job"
	"ai-interview-platform/internal/resume"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/llm"
)

// Service 面试业务逻辑层
type Service struct {
	repo      *Repository
	jobRepo   *job.Repository
	resumeSvc *resume.Service
	llm       llm.Provider
	log       *slog.Logger
}

// NewService 创建面试 Service
func NewService(repo *Repository, jobRepo *job.Repository, resumeSvc *resume.Service, llmProv llm.Provider, log *slog.Logger) *Service {
	return &Service{
		repo:      repo,
		jobRepo:   jobRepo,
		resumeSvc: resumeSvc,
		llm:       llmProv,
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

// Start 开始面试：状态机 INIT/READY → RUNNING，并生成问题
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

	// 基础 AI 出题（LLM 只生成内容，不决定状态）
	questions, err := s.generateQuestions(ctx, sess)
	if err != nil {
		s.log.Error("generate questions failed", "session_id", sessionID, "error", err)
		return nil, apperrors.Wrap("QUESTION_GENERATION_FAILED", "生成面试问题失败", 500, err)
	}
	if len(questions) == 0 {
		return nil, apperrors.New("NO_QUESTIONS_GENERATED", "未能生成面试问题", 500)
	}

	if err := s.repo.CreateQuestions(ctx, sess.ID, questions); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "保存面试问题失败", 500, err)
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

// Get 查询面试详情（含问题和回答状态）
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

// SubmitAnswer 提交回答
func (s *Service) SubmitAnswer(ctx context.Context, userID, sessionID string, req SubmitAnswerRequest) (*Answer, error) {
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

	// 每个问题只能回答一次（数据库 UNIQUE 约束兜底）
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

	return answer, nil
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

// generatedQuestion LLM 生成的临时问题结构（AI 数据 Contract，见 docs/AI_DATA_CONTRACTS.md）
type generatedQuestion struct {
	Question       string   `json:"question"`
	Type           string   `json:"type"`
	Difficulty     string   `json:"difficulty"`
	ExpectedPoints []string `json:"expected_points"`
}

type generatedQuestionList struct {
	Questions []generatedQuestion `json:"questions"`
}

// generateQuestions 基础 AI 出题：根据岗位和简历生成问题
// LLM 输出必须经过 Schema 校验 + 业务校验，且不决定任何业务状态
func (s *Service) generateQuestions(ctx context.Context, sess *Session) ([]Question, error) {
	// 收集岗位信息
	j, err := s.jobRepo.GetJobByID(ctx, sess.JobID)
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	if j == nil {
		return nil, fmt.Errorf("job not found: %s", sess.JobID)
	}

	// 收集简历技能（可选）
	resumeSkills := ""
	if sess.ResumeID != "" {
		rs, err := s.resumeSvc.Get(ctx, sess.ResumeID)
		if err == nil && rs != nil && len(rs.Skills) > 0 {
			resumeSkills = strings.Join(rs.Skills, "、")
		}
	}

	prompt := fmt.Sprintf(`你是一个技术面试官。请根据以下信息生成 %d 道面试题。

岗位：%s
岗位技能要求：%s
岗位任职要求：%s
候选人技能：%s
面试类型：%s

要求：
1. 只返回 JSON，不要包含任何解释文字
2. JSON 结构：{"questions":[{"question":"","type":"","difficulty":"","expected_points":[""]}]}
3. type 只能是 technical、behavioral、project 之一
4. difficulty 只能是 easy、medium、hard 之一
5. expected_points 是该题的参考答案要点，2-5 条
6. 问题应循序渐进，覆盖岗位核心技能`,
		sess.Config.QuestionCount, j.Title, strings.Join(j.Skills, "、"),
		strings.Join(j.Requirements, "；"), resumeSkills, sess.InterviewType)

	resp, err := s.llm.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "你是一个专业的技术面试官，只输出 JSON。"},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	// Schema 校验
	content := cleanJSON(resp.Content)
	var parsed generatedQuestionList
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("invalid llm json output: %w", err)
	}

	// 业务校验 + 归一化
	validTypes := map[string]bool{QTypeTechnical: true, QTypeBehavioral: true, QTypeProject: true}
	validDifficulty := map[string]bool{"easy": true, "medium": true, "hard": true}

	var questions []Question
	for i, gq := range parsed.Questions {
		if strings.TrimSpace(gq.Question) == "" {
			continue
		}
		if !validTypes[gq.Type] {
			gq.Type = QTypeTechnical
		}
		if !validDifficulty[gq.Difficulty] {
			gq.Difficulty = "medium"
		}
		if gq.ExpectedPoints == nil {
			gq.ExpectedPoints = []string{}
		}

		questions = append(questions, Question{
			Seq:            i + 1,
			QuestionType:   gq.Type,
			Question:       strings.TrimSpace(gq.Question),
			Difficulty:     gq.Difficulty,
			Source:         "llm",
			ExpectedPoints: gq.ExpectedPoints,
		})
	}

	return questions, nil
}

// cleanJSON 清理 LLM 输出中的 markdown 标记
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
