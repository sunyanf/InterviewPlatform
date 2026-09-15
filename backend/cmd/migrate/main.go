// Command migrate 管理数据库迁移。
//
// 用法：
//
//	go run ./cmd/migrate up       应用全部待执行迁移（默认命令）
//	go run ./cmd/migrate status   查看当前版本与待执行迁移
//	go run ./cmd/migrate -dir ./migrations up   使用本地目录而非内嵌 SQL
//
// 迁移文件编译期内嵌进二进制（backend/migrations/embed.go），
// 容器部署时无需挂载 migrations 目录。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/database"
	dbmigrations "ai-interview-platform/migrations"
)

func main() {
	dir := flag.String("dir", "", "迁移文件目录（默认使用二进制内嵌 SQL）")
	flag.Parse()

	cmd := flag.Arg(0)
	if cmd == "" {
		cmd = "up"
	}
	if cmd != "up" && cmd != "status" {
		fmt.Fprintf(os.Stderr, "unknown command %q: use 'up' or 'status'\n", cmd)
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fatal("load config: %v", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		fatal("connect database: %v", err)
	}
	defer pool.Close()

	src := database.NewEmbedSource(dbmigrations.FS)
	if *dir != "" {
		src = database.NewDirSource(*dir)
	}
	mg := database.NewMigrator(pool, src)

	switch cmd {
	case "status":
		st, err := mg.Status(ctx)
		if err != nil {
			fatal("query status: %v", err)
		}
		fmt.Printf("current version: %04d\n", st.Current)
		if len(st.Pending) == 0 {
			fmt.Println("database is up to date")
			return
		}
		fmt.Printf("pending (%d):\n", len(st.Pending))
		for _, m := range st.Pending {
			fmt.Printf("  %s\n", m.Filename)
		}
	case "up":
		st, err := mg.Status(ctx)
		if err != nil {
			fatal("query status: %v", err)
		}
		if len(st.Pending) == 0 {
			log.Info("database is up to date", "current_version", fmt.Sprintf("%04d", st.Current))
			return
		}
		applied, err := mg.Up(ctx)
		if err != nil {
			fatal("migrate up: %v", err)
		}
		for _, m := range applied {
			log.Info("applied migration", "version", fmt.Sprintf("%04d", m.Version), "file", m.Filename)
		}
		log.Info("migrations complete", "applied", len(applied))
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", args...)
	os.Exit(1)
}
