package realtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"ai-interview-platform/internal/agent"
	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/interview"
	ttssvc "ai-interview-platform/internal/tts"
	apperrors "ai-interview-platform/pkg/errors"
	"ai-interview-platform/pkg/jwt"
)

// ---------- fakes ----------

type fakeService struct {
	sess      *interview.Session
	getErr    error
	submitErr error
	lastReq   interview.SubmitAnswerRequest
}

func (f *fakeService) Get(ctx context.Context, userID, sessionID string) (*interview.Session, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.sess, nil
}

func (f *fakeService) SubmitAnswer(ctx context.Context, userID, sessionID string, req interview.SubmitAnswerRequest) (*interview.SubmitAnswerResult, error) {
	f.lastReq = req
	if f.submitErr != nil {
		return nil, f.submitErr
	}
	return &interview.SubmitAnswerResult{
		Answer:   &interview.Answer{ID: "a1", SessionID: sessionID, QuestionID: req.QuestionID, TextContent: req.TextContent},
		Analysis: &agent.AnswerAnalysis{Claims: []string{"c1"}},
		FollowUp: &interview.Question{ID: "q2", SessionID: sessionID, QuestionType: interview.QTypeFollowUp, Question: "追问？"},
	}, nil
}

type fakeAgent struct {
	deltas []string
	err    error
}

func (f *fakeAgent) ChatInterviewer(ctx context.Context, in agent.RealtimeChatInput) (<-chan agent.RealtimeChatEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	ch := make(chan agent.RealtimeChatEvent)
	go func() {
		defer close(ch)
		for _, d := range f.deltas {
			select {
			case <-ctx.Done():
				ch <- agent.RealtimeChatEvent{Err: ctx.Err()}
				return
			case ch <- agent.RealtimeChatEvent{Delta: d}:
			}
		}
	}()
	return ch, nil
}

type fakeSpeech struct {
	res     *ttssvc.SpeechResult
	err     error
	gotText string
}

func (f *fakeSpeech) SynthesizeForSession(ctx context.Context, userID, sessionID, text string) (*ttssvc.SpeechResult, error) {
	f.gotText = text
	if f.err != nil {
		return nil, f.err
	}
	return f.res, nil
}

// ---------- helpers ----------

func testJWTManager() *jwt.Manager {
	return jwt.NewManager(config.JWTConfig{Secret: "test-secret", ExpireTime: time.Hour, Issuer: "test"})
}

func newTestServer(svc InterviewService, ag ChatAgent) (*httptest.Server, *jwt.Manager) {
	return newTestServerWithSpeech(svc, ag, nil)
}

func newTestServerWithSpeech(svc InterviewService, ag ChatAgent, sp SpeechSynthesizer) (*httptest.Server, *jwt.Manager) {
	mgr := testJWTManager()
	h := NewHandler(mgr, svc, ag, sp, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := chi.NewRouter()
	r.Get("/api/v1/interviews/{id}/ws", h.HandleWS)
	return httptest.NewServer(r), mgr
}

func wsURL(srv *httptest.Server, sessionID, token string) string {
	u := strings.Replace(srv.URL, "http", "ws", 1) + "/api/v1/interviews/" + sessionID + "/ws"
	if token != "" {
		u += "?token=" + token
	}
	return u
}

func dial(t *testing.T, srv *httptest.Server, sessionID, token string) (*websocket.Conn, *http.Response) {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(srv, sessionID, token), nil)
	if err != nil {
		return nil, resp
	}
	return conn, resp
}

func mustDial(t *testing.T, srv *httptest.Server, sessionID, token string) *websocket.Conn {
	t.Helper()
	conn, _ := dial(t, srv, sessionID, token)
	if conn == nil {
		t.Fatalf("dial failed")
	}
	return conn
}

func readEnvelope(t *testing.T, conn *websocket.Conn) Envelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("unmarshal %q: %v", string(payload), err)
	}
	return env
}

func writeEnvelope(t *testing.T, conn *websocket.Conn, typ string, data any) {
	t.Helper()
	raw, _ := json.Marshal(data)
	if err := conn.WriteJSON(Envelope{Type: typ, Data: raw}); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func runningSession() *interview.Session {
	return &interview.Session{
		ID: "s1", UserID: "u1", JobTitle: "后端工程师",
		InterviewType: "technical", Status: interview.StatusRunning,
	}
}

// ---------- tests ----------

// TestWS_OriginWhitelist 白名单对 WebSocket 升级生效
func TestWS_OriginWhitelist(t *testing.T) {
	mgr := testJWTManager()
	h := NewHandler(mgr, &fakeService{sess: runningSession()}, &fakeAgent{}, nil,
		[]string{"https://app.example.com"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := chi.NewRouter()
	r.Get("/api/v1/interviews/{id}/ws", h.HandleWS)
	srv := httptest.NewServer(r)
	defer srv.Close()

	token, _ := mgr.Generate("u1", "a@b.com")
	url := wsURL(srv, "s1", token)

	// 命中白名单：升级成功
	allowed := http.Header{}
	allowed.Set("Origin", "https://app.example.com")
	conn, _, err := websocket.DefaultDialer.Dial(url, allowed)
	if err != nil {
		t.Fatalf("whitelisted origin should upgrade: %v", err)
	}
	conn.Close()

	// 非白名单：服务端 403，握手失败
	blocked := http.Header{}
	blocked.Set("Origin", "https://evil.example.com")
	_, resp, err := websocket.DefaultDialer.Dial(url, blocked)
	if err == nil {
		t.Fatal("cross-origin dial must fail")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403 for non-whitelisted origin, got %+v", resp)
	}
}

func TestWS_MissingToken(t *testing.T) {
	srv, _ := newTestServer(&fakeService{sess: runningSession()}, &fakeAgent{})
	defer srv.Close()

	_, resp := dial(t, srv, "s1", "")
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestWS_InvalidToken(t *testing.T) {
	srv, _ := newTestServer(&fakeService{sess: runningSession()}, &fakeAgent{})
	defer srv.Close()

	_, resp := dial(t, srv, "s1", "garbage.token.here")
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestWS_ForbiddenSession(t *testing.T) {
	svc := &fakeService{getErr: apperrors.ErrForbidden}
	srv, mgr := newTestServer(svc, &fakeAgent{})
	defer srv.Close()

	token, _ := mgr.Generate("u1", "a@b.com")
	_, resp := dial(t, srv, "s1", token)
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", resp)
	}
}

func TestWS_SnapshotPingAnswerChatFlow(t *testing.T) {
	svc := &fakeService{sess: runningSession()}
	ag := &fakeAgent{deltas: []string{"你好", "，请继续"}}
	srv, mgr := newTestServer(svc, ag)
	defer srv.Close()

	token, _ := mgr.Generate("u1", "a@b.com")
	conn := mustDial(t, srv, "s1", token)
	defer conn.Close()

	// 1. 连接即快照
	env := readEnvelope(t, conn)
	if env.Type != typeSnapshot {
		t.Fatalf("first event = %s, want snapshot", env.Type)
	}
	var sess interview.Session
	if err := json.Unmarshal(env.Data, &sess); err != nil || sess.ID != "s1" || sess.Status != interview.StatusRunning {
		t.Fatalf("bad snapshot: %v %s", err, env.Data)
	}

	// 2. ping → pong
	writeEnvelope(t, conn, typePing, struct{}{})
	if env := readEnvelope(t, conn); env.Type != typePong {
		t.Fatalf("got %s, want pong", env.Type)
	}

	// 3. answer → answer_saved / analysis / follow_up
	writeEnvelope(t, conn, typeAnswer, AnswerMessage{QuestionID: "q1", TextContent: "goroutine 是轻量级协程", DurationMs: 3000})
	want := []string{typeAnswerSaved, typeAnalysis, typeFollowUp}
	for _, typ := range want {
		if env := readEnvelope(t, conn); env.Type != typ {
			t.Fatalf("got %s, want %s", env.Type, typ)
		}
	}
	if svc.lastReq.QuestionID != "q1" || svc.lastReq.DurationMs != 3000 {
		t.Fatalf("service got unexpected request: %+v", svc.lastReq)
	}

	// 4. chat → chat_delta * n + chat_done
	writeEnvelope(t, conn, typeChat, ChatMessage{
		Message: "能提示一下吗？",
		History: []ChatTurn{{Role: "user", Content: "上一轮"}, {Role: "assistant", Content: "上轮回复"}},
	})
	var gotDeltas strings.Builder
	for {
		env := readEnvelope(t, conn)
		if env.Type == typeChatDone {
			break
		}
		if env.Type != typeChatDelta {
			t.Fatalf("got %s, want chat_delta", env.Type)
		}
		var d ChatDeltaData
		json.Unmarshal(env.Data, &d)
		gotDeltas.WriteString(d.Delta)
	}
	if gotDeltas.String() != "你好，请继续" {
		t.Fatalf("deltas = %q", gotDeltas.String())
	}

	// 5. 未知消息类型 → error 事件且连接保持
	writeEnvelope(t, conn, "frobnicate", struct{}{})
	if env := readEnvelope(t, conn); env.Type != typeError {
		t.Fatalf("got %s, want error", env.Type)
	}
}

func TestWS_AnswerBusinessError(t *testing.T) {
	svc := &fakeService{sess: runningSession(), submitErr: apperrors.New("ALREADY_ANSWERED", "该问题已回答过", 409)}
	srv, mgr := newTestServer(svc, &fakeAgent{})
	defer srv.Close()

	token, _ := mgr.Generate("u1", "a@b.com")
	conn := mustDial(t, srv, "s1", token)
	defer conn.Close()
	if env := readEnvelope(t, conn); env.Type != typeSnapshot {
		t.Fatalf("want snapshot, got %s", env.Type)
	}

	writeEnvelope(t, conn, typeAnswer, AnswerMessage{QuestionID: "q1", TextContent: "回答"})
	env := readEnvelope(t, conn)
	if env.Type != typeError {
		t.Fatalf("got %s, want error", env.Type)
	}
	var ed ErrorData
	json.Unmarshal(env.Data, &ed)
	if ed.Code != "ALREADY_ANSWERED" {
		t.Fatalf("error code = %s", ed.Code)
	}
}

func TestWS_ChatRejectsNonRunningSession(t *testing.T) {
	sess := runningSession()
	sess.Status = interview.StatusCompleted
	srv, mgr := newTestServer(&fakeService{sess: sess}, &fakeAgent{})
	defer srv.Close()

	token, _ := mgr.Generate("u1", "a@b.com")
	conn := mustDial(t, srv, "s1", token)
	defer conn.Close()
	readEnvelope(t, conn) // snapshot

	writeEnvelope(t, conn, typeChat, ChatMessage{Message: "hi"})
	env := readEnvelope(t, conn)
	var ed ErrorData
	json.Unmarshal(env.Data, &ed)
	if env.Type != typeError || ed.Code != "SESSION_NOT_RUNNING" {
		t.Fatalf("got type=%s data=%s", env.Type, env.Data)
	}
}

func TestWS_ChatEmptyMessage(t *testing.T) {
	srv, mgr := newTestServer(&fakeService{sess: runningSession()}, &fakeAgent{})
	defer srv.Close()

	token, _ := mgr.Generate("u1", "a@b.com")
	conn := mustDial(t, srv, "s1", token)
	defer conn.Close()
	readEnvelope(t, conn)

	writeEnvelope(t, conn, typeChat, ChatMessage{Message: "   "})
	env := readEnvelope(t, conn)
	var ed ErrorData
	json.Unmarshal(env.Data, &ed)
	if ed.Code != "EMPTY_MESSAGE" {
		t.Fatalf("got %s", ed.Code)
	}
}

func TestWS_ReconnectReplacesOldConnection(t *testing.T) {
	svc := &fakeService{sess: runningSession()}
	ag := &fakeAgent{deltas: []string{"x"}}
	srv, mgr := newTestServer(svc, ag)
	defer srv.Close()

	token, _ := mgr.Generate("u1", "a@b.com")

	connA := mustDial(t, srv, "s1", token)
	defer connA.Close()
	if env := readEnvelope(t, connA); env.Type != typeSnapshot {
		t.Fatalf("A want snapshot, got %s", env.Type)
	}

	connB := mustDial(t, srv, "s1", token)
	defer connB.Close()
	if env := readEnvelope(t, connB); env.Type != typeSnapshot {
		t.Fatalf("B want snapshot, got %s", env.Type)
	}

	// 旧连接 A 应收到自定义 close 帧（被顶替）
	_ = connA.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := connA.ReadMessage()
	ce, ok := err.(*websocket.CloseError)
	if !ok || ce.Code != closeReplaced {
		t.Fatalf("A want close %d, got %v", closeReplaced, err)
	}
}

// ---------- TTS ----------

func TestWS_ChatWithTTS(t *testing.T) {
	ag := &fakeAgent{deltas: []string{"你好", "，请介绍项目"}}
	sp := &fakeSpeech{res: &ttssvc.SpeechResult{
		DownloadURL: "https://signed/x.wav", Format: "wav", ContentType: "audio/wav",
		SizeBytes: 128, Cached: true, DownloadExpireSeconds: 900,
	}}
	srv, mgr := newTestServerWithSpeech(&fakeService{sess: runningSession()}, ag, sp)
	defer srv.Close()

	token, _ := mgr.Generate("u1", "a@b.com")
	conn := mustDial(t, srv, "s1", token)
	defer conn.Close()
	readEnvelope(t, conn) // snapshot

	writeEnvelope(t, conn, typeChat, ChatMessage{Message: "开始吧", TTS: true})

	// deltas...
	var gotDeltas strings.Builder
	for {
		env := readEnvelope(t, conn)
		if env.Type == typeChatSpeech {
			var data ChatSpeechData
			if err := json.Unmarshal(env.Data, &data); err != nil {
				t.Fatalf("speech data: %v", err)
			}
			if data.DownloadURL != "https://signed/x.wav" || data.Format != "wav" || data.DownloadExpireSeconds != 900 {
				t.Fatalf("speech data = %+v", data)
			}
			break
		}
		if env.Type != typeChatDelta {
			t.Fatalf("got %s before chat_speech", env.Type)
		}
		var d ChatDeltaData
		json.Unmarshal(env.Data, &d)
		gotDeltas.WriteString(d.Delta)
	}
	if gotDeltas.String() != "你好，请介绍项目" {
		t.Fatalf("deltas = %q", gotDeltas.String())
	}
	// 合成入参为完整回复（非分片）
	if sp.gotText != "你好，请介绍项目" {
		t.Fatalf("speech input = %q", sp.gotText)
	}
	// chat_speech 之后必须以 chat_done 终结
	if env := readEnvelope(t, conn); env.Type != typeChatDone {
		t.Fatalf("got %s, want chat_done", env.Type)
	}
}

func TestWS_ChatTTSUnavailable(t *testing.T) {
	ag := &fakeAgent{deltas: []string{"回复"}}
	// newTestServer 不注入 speech
	srv, mgr := newTestServer(&fakeService{sess: runningSession()}, ag)
	defer srv.Close()

	token, _ := mgr.Generate("u1", "a@b.com")
	conn := mustDial(t, srv, "s1", token)
	defer conn.Close()
	readEnvelope(t, conn)

	writeEnvelope(t, conn, typeChat, ChatMessage{Message: "hi", TTS: true})
	env := readEnvelope(t, conn) // chat_delta
	if env.Type != typeChatDelta {
		t.Fatalf("got %s, want chat_delta", env.Type)
	}
	env = readEnvelope(t, conn) // error（未启用 TTS）
	var ed ErrorData
	json.Unmarshal(env.Data, &ed)
	if env.Type != typeError || ed.Code != "TTS_UNAVAILABLE" {
		t.Fatalf("got type=%s code=%s", env.Type, ed.Code)
	}
	// 语音失败不影响本轮终结
	if env := readEnvelope(t, conn); env.Type != typeChatDone {
		t.Fatalf("got %s, want chat_done", env.Type)
	}
}

func TestWS_ChatTTSFailure(t *testing.T) {
	ag := &fakeAgent{deltas: []string{"回复"}}
	sp := &fakeSpeech{err: apperrors.New("TTS_SYNTHESIZE_FAILED", "语音合成失败", 502)}
	srv, mgr := newTestServerWithSpeech(&fakeService{sess: runningSession()}, ag, sp)
	defer srv.Close()

	token, _ := mgr.Generate("u1", "a@b.com")
	conn := mustDial(t, srv, "s1", token)
	defer conn.Close()
	readEnvelope(t, conn)

	writeEnvelope(t, conn, typeChat, ChatMessage{Message: "hi", TTS: true})
	readEnvelope(t, conn) // chat_delta
	env := readEnvelope(t, conn)
	var ed ErrorData
	json.Unmarshal(env.Data, &ed)
	if env.Type != typeError || ed.Code != "TTS_SYNTHESIZE_FAILED" {
		t.Fatalf("got type=%s data=%s", env.Type, env.Data)
	}
	if env := readEnvelope(t, conn); env.Type != typeChatDone {
		t.Fatalf("got %s, want chat_done", env.Type)
	}
}
