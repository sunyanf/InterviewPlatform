package audio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"ai-interview-platform/internal/agent"
	"ai-interview-platform/internal/interview"
	"ai-interview-platform/pkg/asr"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/storage"
)

// 上传限制（与简历模块一致的 10MB 约束）
const (
	maxAudioSize = 10 * 1024 * 1024
	// maxDurationMs 上限 30 分钟（客户端上报时长不可信，仅做钳制）
	maxDurationMs = int64(30 * 60 * 1000)
	// signedURLExpire 音频临时下载链接有效期
	signedURLExpire = 15 * time.Minute
)

// Service 语音业务逻辑层
type Service struct {
	repo          *Repository
	interviewRepo *interview.Repository
	storage       storage.Storage
	asr           asr.ASR
	agent         *agent.Agent
	log           *slog.Logger
	// asrLanguage ASR 默认语言
	asrLanguage string
}

// NewService 创建语音 Service
func NewService(repo *Repository, interviewRepo *interview.Repository, st storage.Storage, asrProv asr.ASR, agentSvc *agent.Agent, asrLanguage string, log *slog.Logger) *Service {
	return &Service{
		repo:          repo,
		interviewRepo: interviewRepo,
		storage:       st,
		asr:           asrProv,
		agent:         agentSvc,
		asrLanguage:   asrLanguage,
		log:           log,
	}
}

// Upload 上传答案语音（会话必须 RUNNING，重传覆盖）
func (s *Service) Upload(ctx context.Context, userID, questionID, filename string, size int64, reader io.Reader, durationMs int64) (*Audio, error) {
	// 题目归属与状态校验
	q, err := s.interviewRepo.GetQuestionByID(ctx, questionID)
	if err != nil {
		s.log.Error("get question failed", "question_id", questionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询题目失败", 500, err)
	}
	if q == nil {
		return nil, apperrors.New("QUESTION_NOT_FOUND", "题目不存在", 404)
	}

	sess, err := s.interviewRepo.GetByID(ctx, q.SessionID)
	if err != nil {
		s.log.Error("get session failed", "session_id", q.SessionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询面试失败", 500, err)
	}
	if sess == nil {
		return nil, apperrors.New("INTERVIEW_SESSION_NOT_FOUND", "面试不存在", 404)
	}
	if sess.UserID != userID {
		return nil, apperrors.ErrForbidden
	}
	if sess.Status != interview.StatusRunning {
		return nil, apperrors.New("SESSION_NOT_RUNNING", "面试进行中才能上传语音", 400)
	}

	// 格式白名单
	format, err := DetectFormat(filename)
	if err != nil {
		return nil, apperrors.New("INVALID_FORMAT", "不支持的音频格式，仅支持 wav/mp3/m4a/webm/ogg", 400)
	}
	// 大小校验（Handler 已有 MaxBytesReader，此处双保险）
	if size <= 0 || size > maxAudioSize {
		return nil, apperrors.New("INVALID_SIZE", "音频大小必须在 1B-10MB 之间", 400)
	}
	// 客户端上报时长不可信，仅钳制
	if durationMs < 0 {
		durationMs = 0
	}
	if durationMs > maxDurationMs {
		durationMs = maxDurationMs
	}

	// 上传对象存储（同 key 覆盖旧录音）
	objectKey := fmt.Sprintf("answers/%s/%s.%s", q.SessionID, questionID, format)
	if err := s.storage.Upload(ctx, objectKey, reader, size, ContentType(format)); err != nil {
		s.log.Error("upload audio to storage failed", "question_id", questionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "存储音频失败", 500, err)
	}

	a := &Audio{
		SessionID:  q.SessionID,
		QuestionID: questionID,
		UserID:     userID,
		ObjectKey:  objectKey,
		Format:     format,
		SizeBytes:  size,
		DurationMs: durationMs,
		Status:     StatusUploaded,
	}
	if err := s.repo.Upsert(ctx, a); err != nil {
		s.log.Error("save audio record failed", "question_id", questionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "保存语音记录失败", 500, err)
	}

	s.log.Info("audio uploaded", "question_id", questionID, "format", format, "size", size, "duration_ms", durationMs)
	return a, nil
}

// Transcribe 转写语音为文本（ASR，可重跑）
func (s *Service) Transcribe(ctx context.Context, userID, questionID string) (*Audio, error) {
	a, err := s.getOwnedAudio(ctx, userID, questionID)
	if err != nil {
		return nil, err
	}

	// 从对象存储读取音频
	rc, err := s.storage.Download(ctx, a.ObjectKey)
	if err != nil {
		s.log.Error("download audio failed", "question_id", questionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "读取音频失败", 500, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		s.log.Error("read audio data failed", "question_id", questionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "读取音频失败", 500, err)
	}

	// ASR 转写（日志不输出转写内容）
	tr, err := s.asr.Transcribe(ctx, data, fmt.Sprintf("%s.%s", questionID, a.Format), asr.TranscribeOptions{Language: s.asrLanguage})
	if err != nil {
		s.log.Error("asr transcribe failed", "question_id", questionID, "provider", s.asr.Name(), "error", err)
		return nil, apperrors.Wrap("ASR_FAILED", "语音转写失败，可稍后重试", 500, err)
	}
	if tr.Text == "" {
		return nil, apperrors.New("ASR_EMPTY_TRANSCRIPT", "转写结果为空，请检查音频内容", 400)
	}

	// Provider 返回时长优先，否则保留客户端上报时长
	if tr.DurationMs > 0 {
		a.DurationMs = tr.DurationMs
	}
	a.Transcript = tr.Text
	a.Language = tr.Language
	a.ASRProvider = s.asr.Name()
	if err := s.repo.UpdateTranscription(ctx, a.ID, tr.Text, tr.Language, s.asr.Name()); err != nil {
		s.log.Error("save transcript failed", "question_id", questionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "保存转写结果失败", 500, err)
	}
	a.Status = StatusTranscribed

	s.log.Info("audio transcribed", "question_id", questionID, "provider", s.asr.Name(), "chars", len([]rune(tr.Text)))
	return a, nil
}

// Analyze 语音表达分析（确定性指标 + LLM 定性建议，可重跑）
func (s *Service) Analyze(ctx context.Context, userID, questionID string) (*AudioView, error) {
	a, err := s.getOwnedAudio(ctx, userID, questionID)
	if err != nil {
		return nil, err
	}
	if a.Transcript == "" {
		return nil, apperrors.New("NOT_TRANSCRIBED", "请先完成语音转写", 400)
	}

	// 确定性指标
	q, err := s.interviewRepo.GetQuestionByID(ctx, questionID)
	if err != nil || q == nil {
		s.log.Error("get question failed", "question_id", questionID, "error", err)
		return nil, apperrors.New("QUESTION_NOT_FOUND", "题目不存在", 404)
	}
	metrics := ComputeSpeechMetrics(a.Transcript, a.DurationMs)

	// LLM 表达分析（只产建议，不算分）
	analysis, err := s.agent.AnalyzeSpeech(ctx, q.Question, a.Transcript, agent.SpeechMetricsInput{
		DurationSec:    float64(a.DurationMs) / 1000.0,
		CharsPerMinute: metrics.CharsPerMinute,
		Pace:           metrics.Pace,
		FillerCount:    metrics.FillerCount,
		FillerDetail:   metrics.FillerDetail,
	})
	if err != nil {
		s.log.Error("speech analysis failed", "question_id", questionID, "error", err)
		return nil, apperrors.Wrap("ANALYSIS_FAILED", "语音分析失败，可稍后重试", 500, err)
	}

	if err := s.repo.UpdateAnalysis(ctx, a.ID, analysis); err != nil {
		s.log.Error("save speech analysis failed", "question_id", questionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "保存分析结果失败", 500, err)
	}
	a.Analysis = analysis
	a.Status = StatusAnalyzed

	s.log.Info("audio analyzed", "question_id", questionID, "pace", metrics.Pace, "filler_count", metrics.FillerCount)
	return s.buildView(ctx, a, metrics), nil
}

// Get 查询语音详情（含指标与临时下载 URL）
func (s *Service) Get(ctx context.Context, userID, questionID string) (*AudioView, error) {
	a, err := s.getOwnedAudio(ctx, userID, questionID)
	if err != nil {
		return nil, err
	}
	metrics := ComputeSpeechMetrics(a.Transcript, a.DurationMs)
	return s.buildView(ctx, a, metrics), nil
}

// getOwnedAudio 查询并校验语音归属（跨用户 403）
func (s *Service) getOwnedAudio(ctx context.Context, userID, questionID string) (*Audio, error) {
	if questionID == "" {
		return nil, apperrors.New("BAD_REQUEST", "题目 ID 不能为空", 400)
	}
	a, err := s.repo.GetByQuestionID(ctx, questionID)
	if err != nil {
		s.log.Error("get audio failed", "question_id", questionID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询语音失败", 500, err)
	}
	if a == nil {
		return nil, apperrors.New("AUDIO_NOT_FOUND", "该题目尚未上传语音", 404)
	}
	if a.UserID != userID {
		return nil, apperrors.ErrForbidden
	}
	return a, nil
}

// buildView 构建语音详情视图（含临时下载 URL）
func (s *Service) buildView(ctx context.Context, a *Audio, metrics SpeechMetrics) *AudioView {
	url, err := s.storage.GetSignedURL(ctx, a.ObjectKey, signedURLExpire)
	if err != nil {
		s.log.Warn("get signed url failed", "question_id", a.QuestionID, "error", err)
		url = ""
	}
	return &AudioView{
		Audio:          a,
		Metrics:        metrics,
		DownloadURL:    url,
		DownloadExpire: int(signedURLExpire.Seconds()),
	}
}
