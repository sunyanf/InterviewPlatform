package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"ai-interview-platform/internal/config"
	appmiddleware "ai-interview-platform/internal/middleware"
	"ai-interview-platform/internal/user"
	"ai-interview-platform/pkg/jwt"
)

// Server HTTP 服务器
type Server struct {
	cfg    *config.Config
	db     *pgxpool.Pool
	log    *slog.Logger
	jwtMgr *jwt.Manager
	http   *http.Server
}

// New 创建 Server
func New(cfg *config.Config, db *pgxpool.Pool, log *slog.Logger, jwtMgr *jwt.Manager) *Server {
	s := &Server{
		cfg:    cfg,
		db:     db,
		log:    log,
		jwtMgr: jwtMgr,
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
	r.Use(appmiddleware.Logger(s.log))
	r.Use(middleware.Recoverer)

	// 健康检查
	r.Get("/healthz", healthHandler)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// 公开路由
		r.Post("/auth/register", s.userHandler().Register)
		r.Post("/auth/login", s.userHandler().Login)

		// 需要鉴权的路由
		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.Auth(s.jwtMgr))
			r.Get("/me", s.userHandler().Me)
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
