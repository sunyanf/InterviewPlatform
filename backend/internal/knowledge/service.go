package knowledge

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	apperrors "ai-interview-platform/pkg/errors"

	"ai-interview-platform/pkg/embedding"
	"ai-interview-platform/pkg/metrics"
)

// rrfK RRF 常数（标准取值 60）
const rrfK = 60

// Service 知识库业务逻辑层：文档摄取（Chunk + Embedding）与混合检索
type Service struct {
	repo     *Repository
	embedder embedding.Embedder
	log      *slog.Logger
}

// NewService 创建知识库 Service
func NewService(repo *Repository, embedder embedding.Embedder, log *slog.Logger) *Service {
	return &Service{repo: repo, embedder: embedder, log: log}
}

// 合法的知识来源类型
var validSourceTypes = map[string]bool{"manual": true, "official": true, "interview_bank": true, "docs": true}

// CreateDocument 摄取文档：校验 → 分块 → 向量化 → 入库
func (s *Service) CreateDocument(ctx context.Context, req CreateDocumentRequest) (*Document, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, apperrors.New("INVALID_TITLE", "文档标题不能为空", 400)
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, apperrors.New("INVALID_CONTENT", "文档内容不能为空", 400)
	}
	if strings.TrimSpace(req.Source) == "" {
		return nil, apperrors.New("INVALID_SOURCE", "来源（source）不能为空，禁止来源不明的知识", 400)
	}
	sourceType := req.SourceType
	if sourceType == "" {
		sourceType = "manual"
	}
	if !validSourceTypes[sourceType] {
		return nil, apperrors.New("INVALID_SOURCE_TYPE", "source_type 必须为 manual、official、interview_bank 或 docs", 400)
	}

	effectiveFrom, err := parseOptionalTime(req.EffectiveFrom)
	if err != nil {
		return nil, apperrors.New("INVALID_EFFECTIVE_FROM", "effective_from 必须为 RFC3339 时间格式", 400)
	}
	effectiveTo, err := parseOptionalTime(req.EffectiveTo)
	if err != nil {
		return nil, apperrors.New("INVALID_EFFECTIVE_TO", "effective_to 必须为 RFC3339 时间格式", 400)
	}
	if effectiveFrom != nil && effectiveTo != nil && effectiveTo.Before(*effectiveFrom) {
		return nil, apperrors.New("INVALID_EFFECTIVE_RANGE", "effective_to 不能早于 effective_from", 400)
	}

	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}

	// 分块
	chunks := ChunkText(req.Content)
	if len(chunks) == 0 {
		return nil, apperrors.New("INVALID_CONTENT", "文档内容无法生成有效分块", 400)
	}

	// 向量化
	vectors, err := s.embedder.Embed(ctx, chunks)
	if err != nil {
		s.log.Error("embed chunks failed", "title", req.Title, "error", err)
		return nil, apperrors.Wrap("EMBEDDING_FAILED", "生成分块向量失败", 500, err)
	}
	if err := s.checkDimensions(vectors); err != nil {
		return nil, err
	}

	// 构造文档与分块（分块 metadata 冗余文档来源信息，保证召回结果可追踪）
	doc := &Document{
		Title:         strings.TrimSpace(req.Title),
		Source:        strings.TrimSpace(req.Source),
		SourceURL:     strings.TrimSpace(req.SourceURL),
		SourceType:    sourceType,
		EffectiveFrom: effectiveFrom,
		EffectiveTo:   effectiveTo,
		Metadata:      req.Metadata,
	}
	chunkModels := make([]Chunk, len(chunks))
	for i, content := range chunks {
		chunkMeta := map[string]interface{}{
			"document_title": doc.Title,
			"source":         doc.Source,
			"source_type":    doc.SourceType,
			"chunk_seq":      i + 1,
		}
		for k, v := range doc.Metadata { // domain / topic / difficulty / content_type
			chunkMeta[k] = v
		}
		if doc.SourceURL != "" {
			chunkMeta["source_url"] = doc.SourceURL
		}
		chunkModels[i] = Chunk{
			Seq:      i + 1,
			Content:  content,
			Metadata: chunkMeta,
		}
	}

	if err := s.repo.CreateDocumentWithChunks(ctx, doc, chunkModels, vectors); err != nil {
		s.log.Error("create document failed", "title", req.Title, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "保存知识文档失败", 500, err)
	}
	doc.ChunkCount = len(chunkModels)

	s.log.Info("knowledge document ingested", "document_id", doc.ID, "title", doc.Title,
		"chunks", len(chunkModels), "embedder", s.embedder.Name())
	return doc, nil
}

// GetDocument 查询文档详情
func (s *Service) GetDocument(ctx context.Context, id string) (*Document, error) {
	if id == "" {
		return nil, apperrors.ErrBadRequest
	}
	d, err := s.repo.GetDocumentByID(ctx, id)
	if err != nil {
		s.log.Error("get document failed", "id", id, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询知识文档失败", 500, err)
	}
	if d == nil {
		return nil, apperrors.ErrNotFound
	}
	return d, nil
}

// ListDocuments 分页查询文档列表
func (s *Service) ListDocuments(ctx context.Context, page, pageSize int) ([]Document, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	list, total, err := s.repo.ListDocuments(ctx, page, pageSize)
	if err != nil {
		s.log.Error("list documents failed", "error", err)
		return nil, 0, apperrors.Wrap("INTERNAL_ERROR", "查询知识文档列表失败", 500, err)
	}
	return list, total, nil
}

// DeleteDocument 删除文档
func (s *Service) DeleteDocument(ctx context.Context, id string) error {
	if id == "" {
		return apperrors.ErrBadRequest
	}
	if err := s.repo.DeleteDocument(ctx, id); err != nil {
		s.log.Error("delete document failed", "id", id, "error", err)
		return apperrors.Wrap("INTERNAL_ERROR", "删除知识文档失败", 500, err)
	}
	return nil
}

// Search 混合检索：向量 + FTS + 关键词 → RRF 融合
func (s *Service) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	start := time.Now()
	caller := req.Caller
	if caller == "" {
		caller = "api"
	}
	defer func() {
		metrics.RAGDuration.Observe(time.Since(start).Seconds(), caller)
	}()

	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, apperrors.New("INVALID_QUERY", "检索内容不能为空", 400)
	}
	topK := req.TopK
	if topK < 1 || topK > 50 {
		topK = 5
	}
	recallK := topK * 4 // 每个通道召回量，融合后截断

	// 向量通道
	vec, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		s.log.Error("embed query failed", "error", err)
		return nil, apperrors.Wrap("EMBEDDING_FAILED", "生成查询向量失败", 500, err)
	}
	if err := s.checkDimensions(vec); err != nil {
		return nil, err
	}
	vectorHits, err := s.repo.SearchVector(ctx, vec[0], req.Domain, recallK)
	if err != nil {
		s.log.Error("vector search failed", "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "向量检索失败", 500, err)
	}

	// FTS 通道
	ftsHits, err := s.repo.SearchFTS(ctx, query, req.Domain, recallK)
	if err != nil {
		s.log.Error("fts search failed", "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "全文检索失败", 500, err)
	}

	// 关键词通道（中文兜底）
	kwHits, err := s.repo.SearchKeyword(ctx, keywordsForSearch(query), req.Domain, recallK)
	if err != nil {
		s.log.Error("keyword search failed", "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "关键词检索失败", 500, err)
	}

	fused := FuseRRF(
		[]string{"vector", "fts", "keyword"},
		[][]scoredChunk{vectorHits, ftsHits, kwHits},
		topK,
	)

	s.log.Info("hybrid search done", "query", query, "domain", req.Domain, "caller", caller,
		"vector_hits", len(vectorHits), "fts_hits", len(ftsHits), "keyword_hits", len(kwHits), "fused", len(fused))

	metrics.RAGQueries.Inc(caller, boolLabel(len(fused) > 0))
	return &SearchResult{Query: query, Results: fused}, nil
}

// boolLabel 指标标签布尔值
func boolLabel(hit bool) string {
	if hit {
		return "true"
	}
	return "false"
}

// FuseRRF RRF（Reciprocal Rank Fusion）融合多通道检索结果（纯函数，便于测试）
// score = Σ 1/(rrfK + rank)，rank 从 1 开始
func FuseRRF(channels []string, hitLists [][]scoredChunk, topK int) []RetrievedChunk {
	scores := map[string]*RetrievedChunk{}
	for ci, hits := range hitLists {
		if ci >= len(channels) {
			break
		}
		channel := channels[ci]
		for rank, hit := range hits {
			rc, ok := scores[hit.ID]
			if !ok {
				rc = &RetrievedChunk{
					ChunkID:    hit.ID,
					DocumentID: hit.DocumentID,
					Seq:        hit.Seq,
					Content:    hit.Content,
					Metadata:   hit.Metadata,
					Channels:   []string{},
				}
				scores[hit.ID] = rc
			}
			rc.Score += 1.0 / float64(rrfK+rank+1)
			rc.Channels = append(rc.Channels, channel)
		}
	}

	// 按 RRF 分数降序（同分按 ChunkID 保证确定性）
	fused := make([]RetrievedChunk, 0, len(scores))
	for _, rc := range scores {
		fused = append(fused, *rc)
	}
	sort.Slice(fused, func(i, j int) bool {
		if fused[i].Score != fused[j].Score {
			return fused[i].Score > fused[j].Score
		}
		return fused[i].ChunkID < fused[j].ChunkID
	})
	if len(fused) > topK {
		fused = fused[:topK]
	}
	return fused
}

// splitQueryTokens 将查询拆分为关键词 token（中文按标点/空白粗分）
func splitQueryTokens(query string) []string {
	splitFn := func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '、', '，', '。', '；', '：', ',', '.', ';', ':', '?', '？', '!', '！':
			return true
		}
		return false
	}
	return strings.FieldsFunc(query, splitFn)
}

// keywordsForSearch 将查询拆分为检索关键词：
// CJK token 追加相邻二元组改善部分匹配，过滤过短关键词并去重
func keywordsForSearch(query string) []string {
	var kws []string
	seen := map[string]bool{}
	add := func(kw string) {
		kw = strings.ToLower(kw)
		if len(kw) >= 2 && !seen[kw] {
			seen[kw] = true
			kws = append(kws, kw)
		}
	}
	for _, tok := range splitQueryTokens(query) {
		add(tok)
		for _, bg := range cjkBigrams(tok) {
			add(bg)
		}
	}
	return kws
}

// cjkBigrams 提取 token 中相邻 CJK 字符二元组
// 例："调度模型" → ["调度", "度模", "模型"]，使部分短语也能命中
func cjkBigrams(token string) []string {
	runes := []rune(token)
	var out []string
	for i := 0; i+1 < len(runes); i++ {
		if isCJK(runes[i]) && isCJK(runes[i+1]) {
			out = append(out, string(runes[i:i+2]))
		}
	}
	return out
}

// isCJK 判断是否为常用 CJK 统一表意文字
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF)
}

// checkDimensions 校验向量维度与 Embedder 声明一致
// （外部 Provider 输出属于系统边界，必须验证，见 AGENTS.md #11）
func (s *Service) checkDimensions(vectors [][]float32) error {
	for _, v := range vectors {
		if len(v) != s.embedder.Dimensions() {
			s.log.Error("embedding dimension mismatch", "provider", s.embedder.Name(),
				"got", len(v), "want", s.embedder.Dimensions())
			return apperrors.New("EMBEDDING_DIMENSION_MISMATCH",
				fmt.Sprintf("Embedding 维度不一致：provider=%s 返回 %d 维，期望 %d 维",
					s.embedder.Name(), len(v), s.embedder.Dimensions()), 500)
		}
	}
	return nil
}

// parseOptionalTime 解析可选的 RFC3339 时间字符串
func parseOptionalTime(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, fmt.Errorf("invalid time %q: %w", s, err)
	}
	return &t, nil
}
