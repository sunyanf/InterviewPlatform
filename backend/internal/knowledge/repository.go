package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ai-interview-platform/pkg/embedding"
)

// scoredChunk 带原始通道分数的检索中间结果
type scoredChunk struct {
	Chunk
	score float64
}

// Repository 知识库数据访问层
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository 创建知识库 Repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateDocumentWithChunks 事务写入文档与分块（含向量）
func (r *Repository) CreateDocumentWithChunks(ctx context.Context, doc *Document, chunks []Chunk, vectors [][]float32) error {
	if len(chunks) != len(vectors) {
		return errors.New("chunks and vectors length mismatch")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	doc.ID = uuid.New().String()
	doc.Version = 1
	doc.Status = "active"
	now := time.Now()
	doc.CreatedAt = now
	doc.UpdatedAt = now

	metaJSON, _ := json.Marshal(doc.Metadata)
	_, err = tx.Exec(ctx,
		`INSERT INTO knowledge_documents (id, title, source, source_url, source_type, version, effective_from, effective_to, status, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12)`,
		doc.ID, doc.Title, doc.Source, nullIfEmpty(doc.SourceURL), doc.SourceType,
		doc.Version, doc.EffectiveFrom, doc.EffectiveTo, doc.Status, string(metaJSON), doc.CreatedAt, doc.UpdatedAt)
	if err != nil {
		return err
	}

	batch := &pgx.Batch{}
	for i, c := range chunks {
		c.ID = uuid.New().String()
		c.DocumentID = doc.ID
		c.Version = doc.Version
		c.CreatedAt = now
		chunks[i] = c

		metaJSON, _ := json.Marshal(c.Metadata)
		batch.Queue(
			`INSERT INTO knowledge_chunks (id, document_id, seq, content, embedding, metadata, version, created_at)
			 VALUES ($1, $2, $3, $4, $5::vector, $6::jsonb, $7, $8)`,
			c.ID, c.DocumentID, c.Seq, c.Content, embedding.FormatVector(vectors[i]), string(metaJSON), c.Version, c.CreatedAt)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetDocumentByID 根据 ID 查询文档（含分块数）
func (r *Repository) GetDocumentByID(ctx context.Context, id string) (*Document, error) {
	var d Document
	var metaJSON string

	err := r.db.QueryRow(ctx,
		`SELECT d.id, d.title, d.source, COALESCE(d.source_url,''), d.source_type, d.version,
		        d.effective_from, d.effective_to, d.status, d.metadata::text, d.created_at, d.updated_at,
		        (SELECT COUNT(*) FROM knowledge_chunks c WHERE c.document_id = d.id) AS chunk_count
		 FROM knowledge_documents d WHERE d.id = $1`, id,
	).Scan(&d.ID, &d.Title, &d.Source, &d.SourceURL, &d.SourceType, &d.Version,
		&d.EffectiveFrom, &d.EffectiveTo, &d.Status, &metaJSON, &d.CreatedAt, &d.UpdatedAt, &d.ChunkCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(metaJSON), &d.Metadata)
	return &d, nil
}

// ListDocuments 分页查询文档（含分块数）
func (r *Repository) ListDocuments(ctx context.Context, page, pageSize int) ([]Document, int, error) {
	offset := (page - 1) * pageSize

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM knowledge_documents`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx,
		`SELECT d.id, d.title, d.source, COALESCE(d.source_url,''), d.source_type, d.version,
		        d.effective_from, d.effective_to, d.status, d.metadata::text, d.created_at, d.updated_at,
		        (SELECT COUNT(*) FROM knowledge_chunks c WHERE c.document_id = d.id) AS chunk_count
		 FROM knowledge_documents d
		 ORDER BY d.created_at DESC LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []Document
	for rows.Next() {
		var d Document
		var metaJSON string
		if err := rows.Scan(&d.ID, &d.Title, &d.Source, &d.SourceURL, &d.SourceType, &d.Version,
			&d.EffectiveFrom, &d.EffectiveTo, &d.Status, &metaJSON, &d.CreatedAt, &d.UpdatedAt, &d.ChunkCount); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal([]byte(metaJSON), &d.Metadata)
		list = append(list, d)
	}
	return list, total, rows.Err()
}

// DeleteDocument 删除文档（分块级联删除）
func (r *Repository) DeleteDocument(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM knowledge_documents WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// 检索公共过滤条件：仅启用状态 + 生效时间内的知识（过期内容不作为当前标准，见 AGENTS.md #16）
const activeDocFilter = `JOIN knowledge_documents d ON d.id = c.document_id
 WHERE d.status = 'active'
   AND (d.effective_from IS NULL OR d.effective_from <= NOW())
   AND (d.effective_to IS NULL OR d.effective_to > NOW())`

// SearchVector 向量检索通道（余弦距离）
func (r *Repository) SearchVector(ctx context.Context, vec []float32, domain string, limit int) ([]scoredChunk, error) {
	query := `SELECT c.id, c.document_id, c.seq, c.content, c.metadata::text, c.version, c.created_at,
	                 1 - (c.embedding <=> $1::vector) AS score
	          FROM knowledge_chunks c ` + activeDocFilter
	args := []interface{}{embedding.FormatVector(vec)}
	if domain != "" {
		query += ` AND c.metadata->>'domain' = $2`
		args = append(args, domain)
	}
	query += ` ORDER BY c.embedding <=> $1::vector LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	return r.scanScored(ctx, query, args)
}

// SearchFTS 全文检索通道（PostgreSQL FTS，'simple' 配置）
func (r *Repository) SearchFTS(ctx context.Context, queryText, domain string, limit int) ([]scoredChunk, error) {
	query := `SELECT c.id, c.document_id, c.seq, c.content, c.metadata::text, c.version, c.created_at,
	                 ts_rank(to_tsvector('simple', c.content), websearch_to_tsquery('simple', $1)) AS score
	          FROM knowledge_chunks c ` + activeDocFilter + `
	            AND to_tsvector('simple', c.content) @@ websearch_to_tsquery('simple', $1)`
	args := []interface{}{queryText}
	if domain != "" {
		query += ` AND c.metadata->>'domain' = $2`
		args = append(args, domain)
	}
	query += ` ORDER BY score DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	return r.scanScored(ctx, query, args)
}

// SearchKeyword 关键词检索通道（ILIKE，'simple' 不分词中文时的实用兜底）
func (r *Repository) SearchKeyword(ctx context.Context, tokens []string, domain string, limit int) ([]scoredChunk, error) {
	// 过滤空 token 并转小写
	var kws []string
	for _, t := range tokens {
		if t = strings.ToLower(strings.TrimSpace(t)); len(t) >= 2 {
			kws = append(kws, t)
		}
	}
	if len(kws) == 0 {
		return nil, nil
	}

	query := `SELECT c.id, c.document_id, c.seq, c.content, c.metadata::text, c.version, c.created_at,
	                 (SELECT COUNT(*) FROM unnest($1::text[]) AS t WHERE position(t IN lower(c.content)) > 0)::float AS score
	          FROM knowledge_chunks c ` + activeDocFilter + `
	            AND EXISTS (SELECT 1 FROM unnest($1::text[]) AS t WHERE position(t IN lower(c.content)) > 0)`
	args := []interface{}{kws}
	if domain != "" {
		query += ` AND c.metadata->>'domain' = $2`
		args = append(args, domain)
	}
	query += ` ORDER BY score DESC, c.created_at ASC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	return r.scanScored(ctx, query, args)
}

// scanScored 统一扫描检索结果
func (r *Repository) scanScored(ctx context.Context, query string, args []interface{}) ([]scoredChunk, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []scoredChunk
	for rows.Next() {
		var sc scoredChunk
		var metaJSON string
		if err := rows.Scan(&sc.ID, &sc.DocumentID, &sc.Seq, &sc.Content, &metaJSON, &sc.Version, &sc.CreatedAt, &sc.score); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(metaJSON), &sc.Metadata)
		list = append(list, sc)
	}
	return list, rows.Err()
}

// nullIfEmpty 空字符串转 NULL
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
