package tts

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"ai-interview-platform/internal/interview"
	apperrors "ai-interview-platform/pkg/errors"
	provider "ai-interview-platform/pkg/tts"
)

// ---------- fakes ----------

type fakeInterviewSvc struct {
	sess *interview.Session
	err  error
}

func (f *fakeInterviewSvc) Get(ctx context.Context, userID, sessionID string) (*interview.Session, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.sess, nil
}

type fakeProvider struct {
	calls   int
	gotText string
	gotOpts provider.SynthesizeOptions
	speech  *provider.Speech
	err     error
}

func (f *fakeProvider) Synthesize(ctx context.Context, text string, opts provider.SynthesizeOptions) (*provider.Speech, error) {
	f.calls++
	f.gotText = text
	f.gotOpts = opts
	if f.err != nil {
		return nil, f.err
	}
	return f.speech, nil
}
func (f *fakeProvider) Name() string { return "fake" }

// fixedFake 模拟固定输出 WAV 的 Provider（如 mock）
type fixedFake struct {
	*fakeProvider
}

func (f *fixedFake) FixedFormat() string { return "wav" }

// memStorage 内存对象存储 fake
type memStorage struct {
	objects     map[string][]byte
	contentType map[string]string
	uploadCalls int
	existsErr   error
	uploadErr   error
}

func newMemStorage() *memStorage {
	return &memStorage{objects: map[string][]byte{}, contentType: map[string]string{}}
}

func (m *memStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if m.uploadErr != nil {
		return m.uploadErr
	}
	b, _ := io.ReadAll(reader)
	m.objects[key] = b
	m.contentType[key] = contentType
	m.uploadCalls++
	return nil
}
func (m *memStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	b, ok := m.objects[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(strings.NewReader(string(b))), nil
}
func (m *memStorage) GetSignedURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return "https://signed.example/" + key + "?exp=900", nil
}
func (m *memStorage) Exists(ctx context.Context, key string) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	_, ok := m.objects[key]
	return ok, nil
}
func (m *memStorage) Delete(ctx context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

// ---------- helpers ----------

func testService(svc InterviewService, prov provider.TTS, st *memStorage) *Service {
	return NewService(svc, prov, st, "tts-1", "alloy", "mp3",
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func sessionWithOpening() *interview.Session {
	return &interview.Session{
		ID: "s1", UserID: "u1",
		Metadata: map[string]interface{}{"opening_message": "你好，欢迎参加面试"},
		Questions: []interview.Question{
			{ID: "q1", Question: "请介绍 goroutine"},
		},
	}
}

// ---------- tests ----------

func TestOpening_SynthesizeOnCacheMiss(t *testing.T) {
	st := newMemStorage()
	prov := &fakeProvider{speech: &provider.Speech{Audio: []byte("MP3DATA"), Format: "mp3", ContentType: "audio/mpeg"}}
	svc := testService(&fakeInterviewSvc{sess: sessionWithOpening()}, prov, st)

	res, err := svc.SynthesizeOpening(context.Background(), "u1", "s1")
	if err != nil {
		t.Fatalf("synth: %v", err)
	}
	if res.Cached || prov.calls != 1 || st.uploadCalls != 1 {
		t.Fatalf("expected miss synth+upload: cached=%v calls=%d uploads=%d", res.Cached, prov.calls, st.uploadCalls)
	}
	if res.Format != "mp3" || res.ContentType != "audio/mpeg" || res.SizeBytes != 7 {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(res.DownloadURL, "tts-speech/fake/") || res.DownloadExpireSeconds != 900 {
		t.Fatalf("url/expire wrong: %s", res.DownloadURL)
	}
	if prov.gotOpts.Voice != "alloy" || prov.gotOpts.Format != "mp3" {
		t.Fatalf("opts = %+v", prov.gotOpts)
	}
}

func TestOpening_CacheHitSkipsProvider(t *testing.T) {
	st := newMemStorage()
	prov := &fakeProvider{speech: &provider.Speech{Audio: []byte("X"), Format: "mp3", ContentType: "audio/mpeg"}}
	svc := testService(&fakeInterviewSvc{sess: sessionWithOpening()}, prov, st)

	if _, err := svc.SynthesizeOpening(context.Background(), "u1", "s1"); err != nil {
		t.Fatalf("first: %v", err)
	}
	res, err := svc.SynthesizeOpening(context.Background(), "u1", "s1")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !res.Cached || prov.calls != 1 || st.uploadCalls != 1 {
		t.Fatalf("second call must hit cache: cached=%v calls=%d uploads=%d", res.Cached, prov.calls, st.uploadCalls)
	}
}

func TestOpening_NotAvailable(t *testing.T) {
	st := newMemStorage()
	svc := testService(&fakeInterviewSvc{sess: &interview.Session{ID: "s1"}}, &fakeProvider{}, st)

	_, err := svc.SynthesizeOpening(context.Background(), "u1", "s1")
	if ae := new(apperrors.AppError); !errors.As(err, &ae) || ae.Code != "OPENING_NOT_AVAILABLE" || ae.Status != 404 {
		t.Fatalf("want 404 OPENING_NOT_AVAILABLE, got %v", err)
	}
}

func TestQuestion_SuccessAndNotFound(t *testing.T) {
	st := newMemStorage()
	prov := &fakeProvider{speech: &provider.Speech{Audio: []byte("MP3"), Format: "mp3", ContentType: "audio/mpeg"}}
	svc := testService(&fakeInterviewSvc{sess: sessionWithOpening()}, prov, st)

	res, err := svc.SynthesizeQuestion(context.Background(), "u1", "s1", "q1")
	if err != nil {
		t.Fatalf("question synth: %v", err)
	}
	if prov.gotText != "请介绍 goroutine" || res.Cached {
		t.Fatalf("text=%q cached=%v", prov.gotText, res.Cached)
	}

	_, err = svc.SynthesizeQuestion(context.Background(), "u1", "s1", "missing")
	if ae := new(apperrors.AppError); !errors.As(err, &ae) || ae.Code != "QUESTION_NOT_FOUND" {
		t.Fatalf("want QUESTION_NOT_FOUND, got %v", err)
	}
}

func TestSynthesize_OwnershipErrorPropagates(t *testing.T) {
	st := newMemStorage()
	svc := testService(&fakeInterviewSvc{err: apperrors.ErrForbidden}, &fakeProvider{}, st)
	_, err := svc.SynthesizeForSession(context.Background(), "u2", "s1", "任意文本")
	if !errors.Is(err, apperrors.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
}

func TestSynthesize_EmptyAndTooLong(t *testing.T) {
	st := newMemStorage()
	svc := testService(&fakeInterviewSvc{sess: sessionWithOpening()}, &fakeProvider{}, st)

	if _, err := svc.SynthesizeForSession(context.Background(), "u1", "s1", "  "); err == nil {
		t.Fatal("empty text must fail")
	}
	long := strings.Repeat("字", maxSpeechRunes+1)
	_, err := svc.SynthesizeForSession(context.Background(), "u1", "s1", long)
	if ae := new(apperrors.AppError); !errors.As(err, &ae) || ae.Code != "SPEECH_TEXT_TOO_LONG" {
		t.Fatalf("want SPEECH_TEXT_TOO_LONG, got %v", err)
	}
}

func TestSynthesize_ProviderFormatFallback(t *testing.T) {
	st := newMemStorage()
	// 固定格式 Provider（mock 行为）：缓存身份直接用 wav
	prov := &fixedFake{&fakeProvider{speech: &provider.Speech{Audio: []byte("WAVDATA"), Format: "wav", ContentType: "audio/wav"}}}
	svc := testService(&fakeInterviewSvc{sess: sessionWithOpening()}, prov, st)

	res, err := svc.SynthesizeForSession(context.Background(), "u1", "s1", "面试官你好")
	if err != nil {
		t.Fatalf("synth: %v", err)
	}
	if res.Format != "wav" || res.ContentType != "audio/wav" {
		t.Fatalf("format = %s/%s", res.Format, res.ContentType)
	}
	if !strings.HasSuffix(strings.Split(res.DownloadURL, "?")[0], ".wav") {
		t.Fatalf("key must end .wav: %s", res.DownloadURL)
	}
	if prov.gotOpts.Format != "wav" {
		t.Fatalf("provider should be asked wav, got %s", prov.gotOpts.Format)
	}

	// 第二次：wav 键已存在 → 直接命中，不再合成
	res2, _ := svc.SynthesizeForSession(context.Background(), "u1", "s1", "面试官你好")
	if !res2.Cached || prov.calls != 1 {
		t.Fatalf("expected cache hit on actual-format key: cached=%v calls=%d", res2.Cached, prov.calls)
	}
}

func TestSynthesize_UnexpectedProviderFormat(t *testing.T) {
	// 防御路径：Provider 未声明固定格式却返回与请求不一致的格式
	// 首次按 wav 实际格式落盘；第二次仍会再合成一次（初始 mp3 键不存在），但结果仍正确
	st := newMemStorage()
	prov := &fakeProvider{speech: &provider.Speech{Audio: []byte("WAVDATA"), Format: "wav", ContentType: "audio/wav"}}
	svc := testService(&fakeInterviewSvc{sess: sessionWithOpening()}, prov, st)

	res, err := svc.SynthesizeForSession(context.Background(), "u1", "s1", "hi")
	if err != nil {
		t.Fatalf("synth: %v", err)
	}
	if res.Format != "wav" || !strings.HasSuffix(strings.Split(res.DownloadURL, "?")[0], ".wav") {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestSynthesize_ProviderErrorMapped(t *testing.T) {
	st := newMemStorage()
	prov := &fakeProvider{err: errors.New("boom")}
	svc := testService(&fakeInterviewSvc{sess: sessionWithOpening()}, prov, st)

	_, err := svc.SynthesizeForSession(context.Background(), "u1", "s1", "hi")
	if ae := new(apperrors.AppError); !errors.As(err, &ae) || ae.Code != "TTS_SYNTHESIZE_FAILED" || ae.Status != 502 {
		t.Fatalf("want 502 TTS_SYNTHESIZE_FAILED, got %v", err)
	}
}

func TestCacheKey_Namespacing(t *testing.T) {
	base := testService(&fakeInterviewSvc{sess: sessionWithOpening()},
		&fakeProvider{speech: &provider.Speech{Audio: []byte("X"), Format: "mp3", ContentType: "audio/mpeg"}}, newMemStorage())

	k1 := base.cacheKey("mp3", "同一段文本")

	otherVoice := NewService(base.interviewSvc, base.provider, base.storage, "tts-1", "nova", "mp3", base.log)
	otherFormat := NewService(base.interviewSvc, base.provider, base.storage, "tts-1", "alloy", "wav", base.log)
	otherModel := NewService(base.interviewSvc, base.provider, base.storage, "tts-1-hd", "alloy", "mp3", base.log)

	if k1 == otherVoice.cacheKey("mp3", "同一段文本") {
		t.Fatal("voice must be part of cache identity")
	}
	if k1 == otherFormat.cacheKey("wav", "同一段文本") {
		t.Fatal("format must be part of cache identity")
	}
	if k1 == otherModel.cacheKey("mp3", "同一段文本") {
		t.Fatal("model must be part of cache identity")
	}
	if k1 != base.cacheKey("mp3", "同一段文本") {
		t.Fatal("same inputs must yield same key")
	}
}
