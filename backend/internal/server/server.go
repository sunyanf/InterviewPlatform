package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"ai-interview-platform/internal/agent"
	"ai-interview-platform/internal/audio"
	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/evaluation"
	"ai-interview-platform/internal/interview"
	"ai-interview-platform/internal/job"
	"ai-interview-platform/internal/knowledge"
	appmiddleware "ai-interview-platform/internal/middleware"
	"ai-interview-platform/internal/realtime"
	"ai-interview-platform/internal/report"
	"ai-interview-platform/internal/resume"
	ttssvc "ai-interview-platform/internal/tts"
	"ai-interview-platform/internal/user"
	"ai-interview-platform/pkg/asr"
	"ai-interview-platform/pkg/embedding"
	"ai-interview-platform/pkg/jwt"
	"ai-interview-platform/pkg/llm"
	"ai-interview-platform/pkg/storage"
	provider "ai-interview-platform/pkg/tts"
)

// Server HTTP 服务器
type Server struct {
	cfg       *config.Config
	db        *pgxpool.Pool
	log       *slog.Logger
	jwtMgr    *jwt.Manager
	storage   storage.Storage
	llm       llm.Provider
	embedder  embedding.Embedder
	asr       asr.ASR
	ttsProv   provider.TTS
	resumeSvc *resume.Service
	agent     *agent.Agent
	http      *http.Server
}

// New 创建 Server
func New(cfg *config.Config, db *pgxpool.Pool, log *slog.Logger, jwtMgr *jwt.Manager, st storage.Storage, llmProv llm.Provider) *Server {
	s := &Server{
		cfg:     cfg,
		db:      db,
		log:     log,
		jwtMgr:  jwtMgr,
		storage: st,
		llm:     llmProv,
	}

	// 初始化 Embedding Provider（RAG）
	embedder, err := embedding.NewProvider(cfg.Embedding)
	if err != nil {
		log.Error("init embedding provider failed", "error", err)
		panic(err)
	}
	s.embedder = embedder
	log.Info("embedding provider initialized", "provider", embedder.Name(), "dimensions", embedder.Dimensions())

	// 初始化 ASR Provider（语音转写）
	asrProv, err := asr.NewASR(cfg.ASR)
	if err != nil {
		log.Error("init asr provider failed", "error", err)
		panic(err)
	}
	s.asr = asrProv
	log.Info("asr provider initialized", "provider", asrProv.Name())

	// 初始化 TTS Provider（面试官语音合成）
	ttsProv, err := provider.NewTTS(cfg.TTS)
	if err != nil {
		log.Error("init tts provider failed", "error", err)
		panic(err)
	}
	s.ttsProv = ttsProv
	log.Info("tts provider initialized", "provider", ttsProv.Name(),
		"model", cfg.TTS.Model, "voice", cfg.TTS.Voice, "format", cfg.TTS.Format)

	// 共享的简历 Service（面试模块依赖）
	resumeRepo := resume.NewRepository(db)
	jobRepo := job.NewRepository(db)
	s.resumeSvc = resume.NewService(resumeRepo, st, llmProv, jobRepo, log)

	// 共享的 Agent（AI 编排）
	s.agent = agent.New(llmProv, log)

	r := s.routes()

	s.http = &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return s
}

// routes 注册路由
func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	// 全局中间件
	r.Use(appmiddleware.RequestID)
	r.Use(appmiddleware.Logger(s.log))
	r.Use(middleware.Recoverer)

	// 健康检查
	r.Get("/healthz", healthHandler)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// 公开路由
		r.Post("/auth/register", s.userHandler().Register)
		r.Post("/auth/login", s.userHandler().Login)

		// 岗位公开接口
		r.Get("/jobs/categories", s.jobHandler().ListCategories)
		r.Get("/jobs", s.jobHandler().ListJobs)
		r.Get("/jobs/{id}", s.jobHandler().GetJob)

		// WebSocket 实时面试：浏览器 WS 无法设置 Authorization 头，
		// 由 handler 自行校验 query token，故不放在 Auth 中间件组内
		r.Get("/interviews/{id}/ws", s.realtimeHandler().HandleWS)

		// 需要鉴权的路由
		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.Auth(s.jwtMgr))
			r.Get("/me", s.userHandler().Me)

			// 岗位管理
			r.Post("/jobs", s.jobHandler().CreateJob)

			// 简历
			r.Post("/resumes", s.resumeHandler().Upload)
			r.Get("/resumes", s.resumeHandler().List)
			r.Get("/resumes/{id}", s.resumeHandler().Get)
			r.Post("/resumes/{id}/parse", s.resumeHandler().Parse)
			r.Post("/resumes/{id}/match", s.resumeHandler().Match)

			// 面试
			r.Post("/interviews", s.interviewHandler().Create)
			r.Get("/interviews", s.interviewHandler().List)
			r.Get("/interviews/{id}", s.interviewHandler().Get)
			r.Post("/interviews/{id}/start", s.interviewHandler().Start)
			r.Post("/interviews/{id}/answer", s.interviewHandler().SubmitAnswer)
			r.Post("/interviews/{id}/finish", s.interviewHandler().Finish)

			// 面试官语音（TTS）
			r.Get("/interviews/{id}/speech/opening", s.ttsHandler().Opening)
			r.Get("/interviews/{id}/questions/{questionID}/speech", s.ttsHandler().Question)

			// 知识库（RAG）
			r.Post("/knowledge/documents", s.knowledgeHandler().Create)
			r.Get("/knowledge/documents", s.knowledgeHandler().List)
			r.Get("/knowledge/documents/{id}", s.knowledgeHandler().Get)
			r.Delete("/knowledge/documents/{id}", s.knowledgeHandler().Delete)
			r.Post("/knowledge/search", s.knowledgeHandler().Search)

			// 评估
			r.Post("/evaluations/sessions/{sessionID}", s.evaluationHandler().Evaluate)
			r.Get("/evaluations/sessions/{sessionID}", s.evaluationHandler().Get)
			r.Get("/evaluations", s.evaluationHandler().List)

			// 报告
			r.Post("/reports/sessions/{sessionID}", s.reportHandler().Generate)
			r.Get("/reports/sessions/{sessionID}", s.reportHandler().Get)
			r.Get("/reports", s.reportHandler().List)

			// 语音（答案录音）
			r.Post("/answers/{questionID}/audio", s.audioHandler().Upload)
			r.Post("/answers/{questionID}/audio/transcribe", s.audioHandler().Transcribe)
			r.Post("/answers/{questionID}/audio/analyze", s.audioHandler().Analyze)
			r.Get("/answers/{questionID}/audio", s.audioHandler().Get)
		})
	})

	return r
}

// userHandler 初始化用户 Handler
func (s *Server) userHandler() *user.Handler {
	repo := user.NewRepository(s.db)
	svc := user.NewService(repo, s.jwtMgr)
	return user.NewHandler(svc)
}

// jobHandler 初始化岗位 Handler
func (s *Server) jobHandler() *job.Handler {
	repo := job.NewRepository(s.db)
	svc := job.NewService(repo, s.log)
	return job.NewHandler(svc)
}

// resumeHandler 初始化简历 Handler
func (s *Server) resumeHandler() *resume.Handler {
	return resume.NewHandler(s.resumeSvc)
}

// interviewHandler 初始化面试 Handler
func (s *Server) interviewHandler() *interview.Handler {
	repo := interview.NewRepository(s.db)
	jobRepo := job.NewRepository(s.db)
	knowledgeRepo := knowledge.NewRepository(s.db)
	knowledgeSvc := knowledge.NewService(knowledgeRepo, s.embedder, s.log)
	svc := interview.NewService(repo, jobRepo, s.resumeSvc, s.agent, knowledgeRetriever{knowledgeSvc}, s.log)
	return interview.NewHandler(svc)
}

// knowledgeHandler 初始化知识库 Handler
func (s *Server) knowledgeHandler() *knowledge.Handler {
	repo := knowledge.NewRepository(s.db)
	svc := knowledge.NewService(repo, s.embedder, s.log)
	return knowledge.NewHandler(svc)
}

// evaluationHandler 初始化评估 Handler
func (s *Server) evaluationHandler() *evaluation.Handler {
	evalRepo := evaluation.NewRepository(s.db)
	interviewRepo := interview.NewRepository(s.db)
	svc := evaluation.NewService(evalRepo, interviewRepo, s.agent, s.log)
	return evaluation.NewHandler(svc)
}

// reportHandler 初始化报告 Handler
func (s *Server) reportHandler() *report.Handler {
	evalRepo := evaluation.NewRepository(s.db)
	interviewRepo := interview.NewRepository(s.db)
	svc := report.NewService(report.NewRepository(s.db), evalRepo, interviewRepo, s.agent, s.log)
	return report.NewHandler(svc)
}

// audioHandler 初始化语音 Handler
func (s *Server) audioHandler() *audio.Handler {
	interviewRepo := interview.NewRepository(s.db)
	repo := audio.NewRepository(s.db)
	svc := audio.NewService(repo, interviewRepo, s.storage, s.asr, s.agent, s.cfg.ASR.Language, s.log)
	return audio.NewHandler(svc)
}

// realtimeHandler 初始化 WebSocket 实时面试 Handler
func (s *Server) realtimeHandler() *realtime.Handler {
	svc := s.newInterviewService()
	speech := ttssvc.NewService(svc, s.ttsProv, s.storage,
		s.cfg.TTS.Model, s.cfg.TTS.Voice, s.cfg.TTS.Format, s.log)
	return realtime.NewHandler(s.jwtMgr, svc, s.agent, speech, s.log)
}

// ttsHandler 初始化面试官语音（TTS）Handler
func (s *Server) ttsHandler() *ttssvc.Handler {
	svc := s.newInterviewService()
	speech := ttssvc.NewService(svc, s.ttsProv, s.storage,
		s.cfg.TTS.Model, s.cfg.TTS.Voice, s.cfg.TTS.Format, s.log)
	return ttssvc.NewHandler(speech)
}

// newInterviewService 构造面试 Service（含 RAG 检索器）
func (s *Server) newInterviewService() *interview.Service {
	repo := interview.NewRepository(s.db)
	jobRepo := job.NewRepository(s.db)
	knowledgeRepo := knowledge.NewRepository(s.db)
	knowledgeSvc := knowledge.NewService(knowledgeRepo, s.embedder, s.log)
	return interview.NewService(repo, jobRepo, s.resumeSvc, s.agent, knowledgeRetriever{knowledgeSvc}, s.log)
}

// knowledgeRetriever 将 knowledge.Service 适配为 interview.KnowledgeRetriever
// （interview 只依赖自己定义的最小接口，不依赖 knowledge 包内部类型）
type knowledgeRetriever struct {
	svc *knowledge.Service
}

// RetrieveForQuery 检索知识片段
func (a knowledgeRetriever) RetrieveForQuery(ctx context.Context, query string, topK int) ([]interview.KnowledgeSnippet, error) {
	result, err := a.svc.Search(ctx, knowledge.SearchRequest{Query: query, TopK: topK})
	if err != nil {
		return nil, err
	}
	snippets := make([]interview.KnowledgeSnippet, 0, len(result.Results))
	for _, rc := range result.Results {
		source, _ := rc.Metadata["source"].(string)
		snippets = append(snippets, interview.KnowledgeSnippet{
			Content: rc.Content,
			Source:  source,
		})
	}
	return snippets, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	s.log.Info("server starting", "addr", s.http.Addr)
	return s.http.ListenAndServe()
}

// Shutdown 优雅关闭
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.http.Shutdown(shutdownCtx)
}

// healthHandler 健康检查
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
