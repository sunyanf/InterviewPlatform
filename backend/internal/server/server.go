package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
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
	"ai-interview-platform/internal/task"
	ttssvc "ai-interview-platform/internal/tts"
	"ai-interview-platform/internal/user"
	"ai-interview-platform/pkg/asr"
	"ai-interview-platform/pkg/embedding"
	"ai-interview-platform/pkg/jwt"
	"ai-interview-platform/pkg/llm"
	"ai-interview-platform/pkg/metrics"
	"ai-interview-platform/pkg/storage"
	provider "ai-interview-platform/pkg/tts"
)

// Server HTTP 服务器
type Server struct {
	cfg          *config.Config
	db           *pgxpool.Pool
	log          *slog.Logger
	jwtMgr       *jwt.Manager
	storage      storage.Storage
	llm          llm.Provider
	embedder     embedding.Embedder
	asr          asr.ASR
	ttsProv      provider.TTS
	resumeSvc    *resume.Service
	evalSvc      *evaluation.Service
	reportSvc    *report.Service
	taskRepo     *task.Repository
	runner       *task.Runner
	authLimit    *appmiddleware.Limiter
	apiLimit     *appmiddleware.Limiter
	agent        *agent.Agent
	knowledgeSvc *knowledge.Service
	http         *http.Server
}

// New 创建 Server
func New(cfg *config.Config, db *pgxpool.Pool, log *slog.Logger, jwtMgr *jwt.Manager, st storage.Storage, llmProv llm.Provider) *Server {
	// LLM 指标装饰：调用量/延迟/token/估算费用（不改变语义，透传流式能力）
	llmProv = llm.NewMeteredProvider(llmProv, cfg.LLM.PriceInputPer1K, cfg.LLM.PriceOutputPer1K)

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

	// 共享的任务仓储与业务 Service（HTTP handler 与 worker 共用同一组实例）
	s.taskRepo = task.NewRepository(db)

	// 共享的知识库 Service（出题/评估 RAG 检索与知识管理 handler 共用）
	knowledgeRepo := knowledge.NewRepository(db)
	s.knowledgeSvc = knowledge.NewService(knowledgeRepo, embedder, log)

	// 共享的简历 Service（面试模块依赖）
	resumeRepo := resume.NewRepository(db)
	jobRepo := job.NewRepository(db)
	s.resumeSvc = resume.NewService(resumeRepo, st, llmProv, jobRepo, s.taskRepo, log)

	// 共享的 Agent（AI 编排）
	s.agent = agent.New(llmProv, log)

	// 评估/报告 Service 单例（HTTP 入队与 worker 执行共用）
	interviewRepo := interview.NewRepository(db)
	evalRepo := evaluation.NewRepository(db)
	s.evalSvc = evaluation.NewService(evalRepo, interviewRepo, s.agent,
		evaluationKnowledge{s.knowledgeSvc}, s.taskRepo, log)
	s.reportSvc = report.NewService(
		report.NewRepository(db), evalRepo, interviewRepo, s.agent, s.taskRepo, log)

	// 异步任务 worker：注册三类长耗时 LLM 任务处理器
	s.runner = task.NewRunner(s.taskRepo, task.Config{
		Workers:         cfg.Task.Workers,
		PollInterval:    cfg.Task.PollInterval,
		LeaseTimeout:    cfg.Task.LeaseTimeout,
		ShutdownTimeout: cfg.Task.ShutdownTimeout,
	}, workerID(), log)
	s.runner.Register(task.TypeResumeParse, s.resumeSvc.RunParseTask)
	s.runner.Register(task.TypeSessionEvaluation, s.evalSvc.RunEvaluationTask)
	s.runner.Register(task.TypeReportGeneration, s.reportSvc.RunGenerateTask)
	// 简历解析任务重试耗尽后把简历状态回写 failed（避免卡在 parsing）
	s.runner.SetFailureHook(func(ctx context.Context, t *task.Task) {
		if t.Type != task.TypeResumeParse {
			return
		}
		var p task.ResumeParsePayload
		if err := json.Unmarshal(t.Payload, &p); err != nil || p.ResumeID == "" {
			return
		}
		if err := resumeRepo.UpdateStatus(ctx, p.ResumeID, resume.StatusFailed); err != nil {
			log.Error("mark resume failed after task exhaustion",
				"resume_id", p.ResumeID, "task_id", t.ID, "error", err)
		}
	})

	// 限流器：登录/注册/刷新按 IP 严格限流；其余 API 按用户（未登录按 IP）
	if cfg.Security.RateLimitEnabled {
		s.authLimit = appmiddleware.NewLimiter(cfg.Security.AuthRatePerMinute, cfg.Security.AuthRatePerMinute)
		s.apiLimit = appmiddleware.NewLimiter(cfg.Security.APIRatePerMinute, cfg.Security.APIRatePerMinute/2)
	}

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
	r.Use(appmiddleware.SecureHeaders)
	r.Use(appmiddleware.Logger(s.log))
	r.Use(appmiddleware.Metrics)
	r.Use(middleware.Recoverer)

	// 健康检查：healthz/livez 仅存活；readyz 校验依赖（PG/MinIO）
	r.Get("/healthz", livezHandler)
	r.Get("/livez", livezHandler)
	r.Get("/readyz", s.readinessHandler)

	// Prometheus 文本格式指标（无鉴权；生产由网络策略限制访问来源）
	r.Method(http.MethodGet, "/metrics", metrics.Handler())

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(appmiddleware.CORS(s.cfg.Server.AllowedOrigins))
		// 全局限流（未登录按 IP，登录后按用户）
		if s.apiLimit != nil {
			r.Use(appmiddleware.RateLimit(s.apiLimit, appmiddleware.KeyByUser))
		}

		// 公开路由（登录/注册/刷新按 IP 严格限流；JSON 体受限）
		authLimit := func(next http.Handler) http.Handler { return next }
		if s.authLimit != nil {
			authLimit = appmiddleware.RateLimit(s.authLimit, appmiddleware.KeyByIP)
		}
		authChain := []func(http.Handler) http.Handler{
			authLimit,
			appmiddleware.MaxBody(s.cfg.Security.MaxBodyBytes),
		}
		r.With(authChain...).Post("/auth/register", s.userHandler().Register)
		r.With(authChain...).Post("/auth/login", s.userHandler().Login)
		r.With(authChain...).Post("/auth/refresh", s.userHandler().Refresh)
		r.With(appmiddleware.MaxBody(s.cfg.Security.MaxBodyBytes)).
			Post("/auth/logout", s.userHandler().Logout)

		// 岗位公开接口
		r.Get("/jobs/categories", s.jobHandler().ListCategories)
		r.Get("/jobs", s.jobHandler().ListJobs)
		r.Get("/jobs/{id}", s.jobHandler().GetJob)

		// WebSocket 实时面试：浏览器 WS 无法设置 Authorization 头，
		// 由 handler 自行校验 query token，故不放在 Auth 中间件组内
		r.Get("/interviews/{id}/ws", s.realtimeHandler().HandleWS)

		// 上传类端点：multipart 放宽请求体上限（简历 / 录音 ≤10MB）
		r.With(appmiddleware.Auth(s.jwtMgr), appmiddleware.MaxBody(s.cfg.Security.MaxUploadBytes)).
			Post("/resumes", s.resumeHandler().Upload)
		r.With(appmiddleware.Auth(s.jwtMgr), appmiddleware.MaxBody(s.cfg.Security.MaxUploadBytes)).
			Post("/answers/{questionID}/audio", s.audioHandler().Upload)

		// 需要鉴权的路由（JSON 请求体受 MaxBodyBytes 限制）
		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.Auth(s.jwtMgr))
			r.Use(appmiddleware.MaxBody(s.cfg.Security.MaxBodyBytes))
			r.Get("/me", s.userHandler().Me)

			// 岗位管理
			r.Post("/jobs", s.jobHandler().CreateJob)

			// 简历
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

			// 异步任务状态查询
			r.Get("/tasks/{taskID}", s.taskHandler().Get)

			// 语音（答案录音；上传端点在组外，multipart 体更宽）
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
	svc := user.NewService(repo, s.jwtMgr, s.cfg.JWT.RefreshTTL, s.log)
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
	svc := interview.NewService(repo, jobRepo, s.resumeSvc, s.agent, knowledgeRetriever{s.knowledgeSvc}, s.log)
	return interview.NewHandler(svc)
}

// knowledgeHandler 初始化知识库 Handler
func (s *Server) knowledgeHandler() *knowledge.Handler {
	return knowledge.NewHandler(s.knowledgeSvc)
}

// evaluationHandler 初始化评估 Handler
func (s *Server) evaluationHandler() *evaluation.Handler {
	return evaluation.NewHandler(s.evalSvc)
}

// reportHandler 初始化报告 Handler
func (s *Server) reportHandler() *report.Handler {
	return report.NewHandler(s.reportSvc)
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
	return realtime.NewHandler(s.jwtMgr, svc, s.agent, speech, s.cfg.Server.AllowedOrigins, s.log)
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
	return interview.NewService(repo, jobRepo, s.resumeSvc, s.agent, knowledgeRetriever{s.knowledgeSvc}, s.log)
}

// knowledgeRetriever 将 knowledge.Service 适配为 interview.KnowledgeRetriever
// （interview 只依赖自己定义的最小接口，不依赖 knowledge 包内部类型）
type knowledgeRetriever struct {
	svc *knowledge.Service
}

// RetrieveForQuery 检索知识片段（出题链路）
func (a knowledgeRetriever) RetrieveForQuery(ctx context.Context, query string, topK int) ([]interview.KnowledgeSnippet, error) {
	result, err := a.svc.Search(ctx, knowledge.SearchRequest{
		Query: query, TopK: topK, Caller: "interview_planning",
	})
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

// evaluationKnowledge 将 knowledge.Service 适配为 evaluation 的 RAG 检索能力
type evaluationKnowledge struct {
	svc *knowledge.Service
}

// RetrieveForEvaluation 检索知识片段（评估链路，保留 ChunkID/Source 供 evidence.reference）
func (a evaluationKnowledge) RetrieveForEvaluation(ctx context.Context, query string, topK int) ([]evaluation.KnowledgeSnippet, error) {
	result, err := a.svc.Search(ctx, knowledge.SearchRequest{
		Query: query, TopK: topK, Caller: "session_evaluation",
	})
	if err != nil {
		return nil, err
	}
	snippets := make([]evaluation.KnowledgeSnippet, 0, len(result.Results))
	for _, rc := range result.Results {
		source, _ := rc.Metadata["source"].(string)
		snippets = append(snippets, evaluation.KnowledgeSnippet{
			ChunkID: rc.ChunkID,
			Content: rc.Content,
			Source:  source,
		})
	}
	return snippets, nil
}

// Start 启动服务器（含异步任务 worker 与限流清理）
func (s *Server) Start(baseCtx context.Context) error {
	// 多副本部署时通过 TASK_ENABLED=false 将实例作为纯 API 节点；
	// 整个部署至少要有一个实例启用 worker，否则异步任务无人领取
	if s.cfg.Task.Enabled {
		s.runner.Start(baseCtx)
	} else {
		s.log.Info("task worker disabled (TASK_ENABLED=false), serving HTTP only")
	}
	if s.authLimit != nil {
		s.authLimit.Start(baseCtx)
	}
	if s.apiLimit != nil {
		s.apiLimit.Start(baseCtx)
	}
	s.log.Info("server starting", "addr", s.http.Addr)
	return s.http.ListenAndServe()
}

// Shutdown 优雅关闭：先停 HTTP 接收，再等待在途任务与限流清理
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	httpErr := s.http.Shutdown(shutdownCtx)
	s.runner.Shutdown()
	if s.authLimit != nil {
		s.authLimit.Shutdown()
	}
	if s.apiLimit != nil {
		s.apiLimit.Shutdown()
	}
	return httpErr
}

// taskHandler 初始化任务查询 Handler
func (s *Server) taskHandler() *task.Handler {
	return task.NewHandler(s.taskRepo)
}

// workerID 生成进程内 worker 标识（锁/日志追踪用）
func workerID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return fmt.Sprintf("%s-%s", host, uuid.New().String()[:8])
}
