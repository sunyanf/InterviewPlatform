package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/database"
	"ai-interview-platform/internal/server"
	"ai-interview-platform/pkg/jwt"
	"ai-interview-platform/pkg/logger"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	// 初始化日志
	log := logger.New(cfg.Log.Level, cfg.Log.Format)

	// 初始化数据库
	ctx := context.Background()
	db, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		log.Error("connect database failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 初始化 JWT
	jwtMgr := jwt.NewManager(cfg.JWT)

	// 初始化 HTTP Server
	srv := server.New(cfg, db, log, jwtMgr)

	// 启动服务（goroutine）
	go func() {
		if err := srv.Start(); err != nil {
			log.Error("server stopped", "error", err)
		}
	}()

	log.Info("server started", "port", cfg.Server.Port, "env", cfg.App.Env)

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server shutdown failed", "error", err)
	}
	log.Info("server stopped")
}
