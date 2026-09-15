package tts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"ai-interview-platform/internal/interview"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/storage"
	provider "ai-interview-platform/pkg/tts"
)

const (
	// maxSpeechRunes 单次合成文本上限（开场白 ≤100 字、实时对话 ≤150 字，2000 为硬上限）
	maxSpeechRunes = 2000
	// signedURLExpire 预签名下载 URL 有效期（与语音答案模块一致）
	signedURLExpire = 15 * time.Minute
	// synthTimeout 单次合成超时
	synthTimeout = 30 * time.Second
)

// InterviewService tts 依赖的面试查询最小接口（生产由 *interview.Service 满足）
type InterviewService interface {
	Get(ctx context.Context, userID, sessionID string) (*interview.Session, error)
}

// Service 面试官语音合成业务层
type Service struct {
	interviewSvc InterviewService
	provider     provider.TTS
	storage      storage.Storage
	log          *slog.Logger

	// 缓存命名空间要素（配置变更后不复用旧音频）
	model  string
	voice  string
	format string
}

// NewService 创建 TTS 业务 Service
func NewService(interviewSvc InterviewService, prov provider.TTS, st storage.Storage,
	model, voice, format string, log *slog.Logger) *Service {
	return &Service{
		interviewSvc: interviewSvc,
		provider:     prov,
		storage:      st,
		log:          log,
		model:        model,
		voice:        voice,
		format:       format,
	}
}

// SynthesizeOpening 合成面试开场白（metadata.opening_message）
func (s *Service) SynthesizeOpening(ctx context.Context, userID, sessionID string) (*SpeechResult, error) {
	sess, err := s.interviewSvc.Get(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	opening, _ := sess.Metadata["opening_message"].(string)
	if strings.TrimSpace(opening) == "" {
		return nil, apperrors.New("OPENING_NOT_AVAILABLE", "开场白不存在或尚未生成", http.StatusNotFound)
	}
	return s.synthesize(ctx, sess, opening)
}

// SynthesizeQuestion 合成指定问题的语音
func (s *Service) SynthesizeQuestion(ctx context.Context, userID, sessionID, questionID string) (*SpeechResult, error) {
	sess, err := s.interviewSvc.Get(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	var text string
	for _, q := range sess.Questions {
		if q.ID == questionID {
			text = q.Question
			break
		}
	}
	if text == "" {
		return nil, apperrors.New("QUESTION_NOT_FOUND", "问题不存在或不属于当前面试", http.StatusNotFound)
	}
	return s.synthesize(ctx, sess, text)
}

// SynthesizeForSession 供实时通道调用：会话归属已在连接时校验，
// 此处再次通过 InterviewService.Get 校验归属（防止重连后会话转手等边界），
// 并对文本施加统一上限。
func (s *Service) SynthesizeForSession(ctx context.Context, userID, sessionID, text string) (*SpeechResult, error) {
	sess, err := s.interviewSvc.Get(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	return s.synthesize(ctx, sess, text)
}

// synthesize 合成核心：内容寻址缓存 → Provider 合成 → 对象存储 → 预签名 URL
func (s *Service) synthesize(ctx context.Context, sess *interview.Session, text string) (*SpeechResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, apperrors.New("EMPTY_SPEECH_TEXT", "合成文本不能为空", http.StatusBadRequest)
	}
	if rc := utf8.RuneCountInString(text); rc > maxSpeechRunes {
		return nil, apperrors.New("SPEECH_TEXT_TOO_LONG",
			fmt.Sprintf("合成文本超过 %d 字上限", maxSpeechRunes), http.StatusBadRequest)
	}

	format := s.format
	// 固定输出格式的 Provider（mock）：以实际格式作为缓存身份，避免每次 miss
	if ff, ok := s.provider.(provider.FixedFormatProvider); ok {
		format = ff.FixedFormat()
	}
	key := s.cacheKey(format, text)

	cached, err := s.storage.Exists(ctx, key)
	if err != nil {
		s.log.Error("tts cache check failed", "session_id", sess.ID, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "检查语音缓存失败", http.StatusInternalServerError, err)
	}

	contentType := provider.FormatContentType(format)
	var size int64

	if !cached {
		synthCtx, cancel := context.WithTimeout(ctx, synthTimeout)
		speech, err := s.provider.Synthesize(synthCtx, text, provider.SynthesizeOptions{
			Voice:  s.voice,
			Format: format,
		})
		cancel()
		if err != nil {
			s.log.Error("tts synthesize failed", "session_id", sess.ID, "provider", s.provider.Name(), "error", err)
			return nil, apperrors.Wrap("TTS_SYNTHESIZE_FAILED", "语音合成失败", http.StatusBadGateway, err)
		}

		// Provider 实际输出格式可能与请求不同（如 mock 固定输出 wav），
		// 以实际格式重算缓存键，保证键扩展名/内容类型与音频一致
		if speech.Format != "" && speech.Format != format {
			format = speech.Format
			key = s.cacheKey(format, text)
			contentType = speech.ContentType
			if exists, exErr := s.storage.Exists(ctx, key); exErr == nil && exists {
				cached = true
			}
		} else {
			contentType = speech.ContentType
		}

		if !cached {
			size = int64(len(speech.Audio))
			if err := s.storage.Upload(ctx, key, bytes.NewReader(speech.Audio), size, contentType); err != nil {
				s.log.Error("tts upload failed", "session_id", sess.ID, "key", key, "error", err)
				return nil, apperrors.Wrap("INTERNAL_ERROR", "保存语音失败", http.StatusInternalServerError, err)
			}
		}
	}

	url, err := s.storage.GetSignedURL(ctx, key, signedURLExpire)
	if err != nil {
		s.log.Error("tts presign failed", "session_id", sess.ID, "key", key, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "生成语音下载链接失败", http.StatusInternalServerError, err)
	}

	s.log.Info("tts speech ready",
		"session_id", sess.ID, "provider", s.provider.Name(),
		"chars", utf8.RuneCountInString(text), "cached", cached, "format", format)

	return &SpeechResult{
		Text:                  text,
		DownloadURL:           url,
		Format:                format,
		ContentType:           contentType,
		SizeBytes:             size,
		Cached:                cached,
		DownloadExpireSeconds: int(signedURLExpire / time.Second),
	}, nil
}

// cacheKey 内容寻址对象键：provider/model/voice/format + 文本哈希
// 任意一项变化都会产生新对象，配置变更不会返回旧音色/旧格式音频
func (s *Service) cacheKey(format, text string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		s.provider.Name(), s.model, s.voice, format, text,
	}, "\x00")))
	return fmt.Sprintf("tts-speech/%s/%s.%s", s.provider.Name(), hex.EncodeToString(sum[:]), format)
}
