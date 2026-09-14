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
	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/interview"
	"ai-interview-platform/internal/job"
	appmiddleware "ai-interview-platform/internal/middleware"
	"ai-interview-platform/internal/resume"
	"ai-interview-platform/internal/user"
	"ai-interview-platform/pkg/jwt"
	"ai-interview-platform/pkg/llm"
	"ai-interview-platform/pkg/storage"
)

// Server HTTP 服务器
type Server struct {
	cfg       *config.Config
	db        *pgxpool.Pool
	log       *slog.Logger
	jwtMgr    *jwt.Manager
	storage   storage.Storage
	llm       llm.Provider
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
	svc := interview.NewService(repo, jobRepo, s.resumeSvc, s.agent, s.log)
	return interview.NewHandler(svc)
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
