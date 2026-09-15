// Command ingest 将本地知识文件（.md/.txt）批量导入知识库。
//
// 每个文件成为一个知识文档，必须显式提供 -source（来源）；
// source_type/effective_from/effective_to 等溯源元数据一并落库
// （AGENTS.md #14/#16：来源可追踪，政策/题库类知识必须带生效时间）。
//
// 用法：
//
//	# 单文件
//	go run ./cmd/ingest -file ./notes/go-channel.md -source "Go 官方文档" \
//	  -source-type docs -source-url https://go.dev/ref/spec -effective-from 2026-01-01T00:00:00Z
//
//	# 目录递归（按文件名排序，仅处理 .md/.txt）
//	go run ./cmd/ingest -dir ./seed/knowledge -source "2026 公务员题库" \
//	  -source-type interview_bank -domain civil_service -topic 综合分析
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/database"
	"ai-interview-platform/internal/knowledge"
	"ai-interview-platform/pkg/embedding"
	"ai-interview-platform/pkg/logger"
)

func main() {
	file := flag.String("file", "", "单个知识文件路径（.md/.txt）")
	dir := flag.String("dir", "", "知识目录（递归处理 .md/.txt）")
	source := flag.String("source", "", "来源名称（必填，禁止来源不明的知识）")
	sourceType := flag.String("source-type", "manual", "来源类型：manual/official/interview_bank/docs")
	sourceURL := flag.String("source-url", "", "来源 URL（可选）")
	effectiveFrom := flag.String("effective-from", "", "生效时间 RFC3339（政策/题库类必填）")
	effectiveTo := flag.String("effective-to", "", "失效时间 RFC3339（可选）")
	domain := flag.String("domain", "", "metadata.domain（可选）")
	topic := flag.String("topic", "", "metadata.topic（可选）")
	difficulty := flag.String("difficulty", "", "metadata.difficulty（可选）")
	contentType := flag.String("content-type", "", "metadata.content_type（可选）")
	flag.Parse()

	if *source == "" {
		fatal("-source is required (knowledge without source is forbidden)")
	}
	if (*file == "" && *dir == "") || (*file != "" && *dir != "") {
		fatal("exactly one of -file or -dir must be specified")
	}

	paths, err := collectFiles(*file, *dir)
	if err != nil {
		fatal(err.Error())
	}
	if len(paths) == 0 {
		fatal("no .md/.txt files found")
	}

	cfg, err := config.Load()
	if err != nil {
		fatalf("load config: %v", err)
	}
	log := logger.New(cfg.Log.Level, cfg.Log.Format)

	ctx := context.Background()
	db, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		fatalf("connect database: %v", err)
	}
	defer db.Close()

	embedder, err := embedding.NewProvider(cfg.Embedding)
	if err != nil {
		fatalf("init embedder: %v", err)
	}
	svc := knowledge.NewService(knowledge.NewRepository(db), embedder, log)

	meta := map[string]interface{}{}
	for k, v := range map[string]string{
		"domain": *domain, "topic": *topic,
		"difficulty": *difficulty, "content_type": *contentType,
	} {
		if v != "" {
			meta[k] = v
		}
	}

	var succeeded, failed int
	for _, path := range paths {
		doc, err := ingestOne(ctx, log, svc, path, knowledge.CreateDocumentRequest{
			Source:        *source,
			SourceType:    *sourceType,
			SourceURL:     *sourceURL,
			EffectiveFrom: *effectiveFrom,
			EffectiveTo:   *effectiveTo,
			Metadata:      meta,
		})
		if err != nil {
			failed++
			log.Error("ingest failed", "path", path, "error", err)
			continue
		}
		succeeded++
		log.Info("ingested", "path", path, "document_id", doc.ID,
			"title", doc.Title, "chunks", doc.ChunkCount)
	}

	log.Info("ingest finished", "total", len(paths),
		"succeeded", succeeded, "failed", failed, "embedder", embedder.Name())
	if failed > 0 {
		os.Exit(1)
	}
}

// ingestOne 读取文件并创建知识文档（标题取 markdown 首个 H1，否则用文件名）
func ingestOne(ctx context.Context, log *slog.Logger, svc *knowledge.Service, path string, req knowledge.CreateDocumentRequest) (*knowledge.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, fmt.Errorf("file is empty")
	}
	req.Content = content
	req.Title = deriveTitle(path, content)
	return svc.CreateDocument(ctx, req)
}

// collectFiles 收集待导入文件：单文件直接返回，目录递归按路径排序
func collectFiles(file, dir string) ([]string, error) {
	if file != "" {
		if !isSupported(file) {
			return nil, fmt.Errorf("unsupported file type (only .md/.txt): %s", file)
		}
		return []string{file}, nil
	}

	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && isSupported(path) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk dir: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func isSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".txt"
}

// deriveTitle 从 markdown 首个 "# " 标题提取文档标题，否则用去掉扩展名的文件名
func deriveTitle(path, content string) string {
	if strings.EqualFold(filepath.Ext(path), ".md") {
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
		}
	}
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}

func fatalf(format string, args ...interface{}) {
	fatal(fmt.Sprintf(format, args...))
}
