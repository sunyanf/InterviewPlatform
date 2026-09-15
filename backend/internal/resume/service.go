package resume

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"ai-interview-platform/internal/job"
	"ai-interview-platform/internal/task"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/llm"
	"ai-interview-platform/pkg/resumeparse"
	"ai-interview-platform/pkg/storage"
)

// 简历状态
const (
	StatusPending = "pending" // 已上传，等待解析
	StatusParsing = "parsing" // 解析任务排队/执行中
	StatusParsed  = "parsed"
	StatusFailed  = "failed"
)

// taskEnqueuer 任务入队能力（*task.Repository 实现；测试可替换）
type taskEnqueuer interface {
	Enqueue(ctx context.Context, req task.EnqueueRequest) (*task.Task, bool, error)
}

// Service 简历业务逻辑层
type Service struct {
	repo    *Repository
	storage storage.Storage
	llm     llm.Provider
	jobRepo *job.Repository
	tasks   taskEnqueuer
	log     *slog.Logger
}

// NewService 创建简历 Service
func NewService(repo *Repository, st storage.Storage, llmProv llm.Provider, jobRepo *job.Repository, tasks taskEnqueuer, log *slog.Logger) *Service {
	return &Service{
		repo:    repo,
		storage: st,
		llm:     llmProv,
		jobRepo: jobRepo,
		tasks:   tasks,
		log:     log,
	}
}

const maxFileSize = 10 * 1024 * 1024 // 10MB

var allowedFileTypes = map[string]bool{
	".txt":  true,
	".pdf":  true,
	".docx": true,
	".md":   true,
}

// Upload 上传简历文件
func (s *Service) Upload(ctx context.Context, userID, fileName string, fileSize int64, reader io.Reader) (*Resume, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	if !allowedFileTypes[ext] {
		return nil, apperrors.New("UNSUPPORTED_FILE_TYPE", "仅支持 txt、pdf、docx、md 格式", 400)
	}
	if fileSize > maxFileSize {
		return nil, apperrors.New("FILE_TOO_LARGE", "文件大小不能超过 10MB", 400)
	}

	// 生成存储 key
	storageKey := fmt.Sprintf("resumes/%s/%s%s", userID, uuid.New().String(), ext)

	// 上传到对象存储
	if err := s.storage.Upload(ctx, storageKey, reader, fileSize, "application/octet-stream"); err != nil {
		return nil, apperrors.Wrap("UPLOAD_FAILED", "文件上传失败", 500, err)
	}

	// 创建简历记录
	rs := &Resume{
		UserID:   userID,
		FileURL:  storageKey,
		FileName: fileName,
		FileType: ext,
		FileSize: fileSize,
		Skills:   []string{},
	}

	if err := s.repo.Create(ctx, rs); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "创建简历记录失败", 500, err)
	}

	return rs, nil
}

// RequestParse 提交简历解析异步任务（归属校验 + 幂等入队）。
// 同一简历已有排队/执行中的解析任务时复用原任务，不重复入队。
func (s *Service) RequestParse(ctx context.Context, userID, resumeID string) (*task.AcceptedView, error) {
	rs, err := s.repo.GetByID(ctx, resumeID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询简历失败", 500, err)
	}
	if rs == nil {
		return nil, apperrors.ErrNotFound
	}
	if rs.UserID != userID {
		return nil, apperrors.ErrForbidden
	}

	if err := s.repo.UpdateStatus(ctx, resumeID, StatusParsing); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "更新简历状态失败", 500, err)
	}

	t, _, err := s.tasks.Enqueue(ctx, task.EnqueueRequest{
		Type: task.TypeResumeParse,
		Payload: task.ResumeParsePayload{
			UserID:   userID,
			ResumeID: resumeID,
		},
		IdempotencyKey: "resume_parse:" + resumeID,
	})
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "创建解析任务失败", 500, err)
	}
	return &task.AcceptedView{TaskID: t.ID, Status: t.Status}, nil
}

// RunParseTask worker 任务处理器：解码负载后执行解析
func (s *Service) RunParseTask(ctx context.Context, raw json.RawMessage) error {
	var p task.ResumeParsePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("decode resume_parse payload: %w", err)
	}
	_, err := s.Parse(ctx, p.ResumeID)
	return err
}

// Parse 解析简历（提取文本 + LLM 结构化）；由 worker 调用，也可被同进程其他流程复用
func (s *Service) Parse(ctx context.Context, resumeID string) (*Resume, error) {
	rs, err := s.repo.GetByID(ctx, resumeID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询简历失败", 500, err)
	}
	if rs == nil {
		return nil, apperrors.ErrNotFound
	}

	// 从对象存储下载文件
	reader, err := s.storage.Download(ctx, rs.FileURL)
	if err != nil {
		return nil, apperrors.Wrap("DOWNLOAD_FAILED", "下载简历文件失败", 500, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, apperrors.Wrap("READ_FAILED", "读取简历文件失败", 500, err)
	}

	// 提取文本
	text, err := resumeparse.ExtractText(rs.FileName, data)
	if err != nil {
		_ = s.repo.UpdateParseResult(ctx, resumeID, "", nil, []string{}, StatusFailed)
		return nil, apperrors.Wrap("PARSE_FAILED", "简历文本提取失败", 400, err)
	}

	if strings.TrimSpace(text) == "" {
		_ = s.repo.UpdateParseResult(ctx, resumeID, "", nil, []string{}, StatusFailed)
		return nil, apperrors.New("EMPTY_CONTENT", "简历内容为空或无法提取", 400)
	}

	// 调用 LLM 结构化解析
	structured, err := s.parseWithLLM(ctx, text)
	if err != nil {
		_ = s.repo.UpdateParseResult(ctx, resumeID, text, nil, []string{}, StatusFailed)
		return nil, apperrors.Wrap("LLM_PARSE_FAILED", "简历结构化解析失败", 500, err)
	}

	// 更新解析结果
	skills := structured.Skills
	if skills == nil {
		skills = []string{}
	}
	if err := s.repo.UpdateParseResult(ctx, resumeID, text, structured, skills, StatusParsed); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "更新解析结果失败", 500, err)
	}

	rs.ParsedContent = text
	rs.StructuredData = structured
	rs.Skills = skills
	rs.Status = StatusParsed
	return rs, nil
}

// parseWithLLM 调用 LLM 将简历文本解析为结构化数据
func (s *Service) parseWithLLM(ctx context.Context, text string) (*StructuredResume, error) {
	prompt := fmt.Sprintf(`你是一个简历解析助手。请从以下简历文本中提取结构化信息，以 JSON 格式返回。

要求：
1. 只返回 JSON，不要包含任何解释文字
2. JSON 结构：{"name":"","email":"","phone":"","education":[{"school":"","major":"","degree":""}],"work_experience":[{"company":"","position":"","duration":""}],"skills":[],"summary":"","target_position":""}
3. skills 提取技术技能、工具、语言等关键词
4. 如果某项信息不存在，填空字符串或空数组

简历文本：
%s`, text)

	resp, err := s.llm.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "你是一个专业的简历解析助手，只输出 JSON。"},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1,
	})
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	// 清理 LLM 输出（可能包含 markdown 代码块标记）
	content := cleanJSON(resp.Content)

	// Schema 验证：解析为 StructuredResume
	var structured StructuredResume
	if err := json.Unmarshal([]byte(content), &structured); err != nil {
		return nil, fmt.Errorf("invalid llm json output: %w", err)
	}

	// 业务验证：skills 不能为空数组以外的非法值
	if structured.Skills == nil {
		structured.Skills = []string{}
	}

	return &structured, nil
}

// cleanJSON 清理 LLM 输出中的 markdown 标记
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	// 去除 ```json ... ``` 包裹
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// Get 查询简历详情
func (s *Service) Get(ctx context.Context, id string) (*Resume, error) {
	rs, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询简历失败", 500, err)
	}
	if rs == nil {
		return nil, apperrors.ErrNotFound
	}
	return rs, nil
}

// List 查询用户简历列表
func (s *Service) List(ctx context.Context, userID string) ([]Resume, error) {
	list, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询简历列表失败", 500, err)
	}
	return list, nil
}

// Match 岗位-简历匹配
func (s *Service) Match(ctx context.Context, resumeID, jobID string) (*MatchResult, error) {
	// 查询简历
	rs, err := s.repo.GetByID(ctx, resumeID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询简历失败", 500, err)
	}
	if rs == nil {
		return nil, apperrors.ErrNotFound
	}
	if rs.Status != StatusParsed {
		return nil, apperrors.New("RESUME_NOT_PARSED", "请先解析简历", 400)
	}

	// 查询岗位
	j, err := s.jobRepo.GetJobByID(ctx, jobID)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询岗位失败", 500, err)
	}
	if j == nil {
		return nil, apperrors.ErrNotFound
	}

	// 计算技能匹配
	resumeSkills := make(map[string]bool)
	for _, s := range rs.Skills {
		resumeSkills[strings.ToLower(s)] = true
	}

	var matched, missing []string
	jobSkills := append([]string{}, j.Skills...)
	// 也从 requirements 中提取技能关键词（简化：取 requirements 中的词）
	for _, req := range j.Requirements {
		jobSkills = append(jobSkills, extractSkillKeywords(req)...)
	}

	for _, skill := range jobSkills {
		skillLower := strings.ToLower(strings.TrimSpace(skill))
		if skillLower == "" {
			continue
		}
		if resumeSkills[skillLower] {
			matched = append(matched, skill)
		} else {
			missing = append(missing, skill)
		}
	}

	// 计算匹配分数（匹配技能数 / 岗位技能总数）
	total := len(matched) + len(missing)
	var score float64
	if total > 0 {
		score = float64(len(matched)) / float64(total) * 100
	}

	if matched == nil {
		matched = []string{}
	}
	if missing == nil {
		missing = []string{}
	}

	return &MatchResult{
		JobID:         j.ID,
		JobTitle:      j.Title,
		MatchScore:    score,
		MatchedSkills: matched,
		MissingSkills: missing,
	}, nil
}

// extractSkillKeywords 从需求描述中提取技能关键词（简化版）
func extractSkillKeywords(req string) []string {
	// 简单按常见分隔符拆分
	parts := strings.FieldsFunc(req, func(r rune) bool {
		return r == '、' || r == ',' || r == '，' || r == ';' || r == '；' || r == ' '
	})
	var keywords []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 && len(p) <= 30 {
			keywords = append(keywords, p)
		}
	}
	return keywords
}
