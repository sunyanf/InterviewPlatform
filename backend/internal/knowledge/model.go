package knowledge

import "time"

// Document 知识文档（来源必须可追踪，见 AGENTS.md #14/#16）
type Document struct {
	ID            string                 `json:"id"`
	Title         string                 `json:"title"`
	Source        string                 `json:"source"`
	SourceURL     string                 `json:"source_url,omitempty"`
	SourceType    string                 `json:"source_type"`
	Version       int                    `json:"version"`
	EffectiveFrom *time.Time             `json:"effective_from,omitempty"`
	EffectiveTo   *time.Time             `json:"effective_to,omitempty"`
	Status        string                 `json:"status"`
	Metadata      map[string]interface{} `json:"metadata"`
	ChunkCount    int                    `json:"chunk_count"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// Chunk 知识分块（含向量，用于检索）
type Chunk struct {
	ID         string                 `json:"id"`
	DocumentID string                 `json:"document_id"`
	Seq        int                    `json:"seq"`
	Content    string                 `json:"content"`
	Metadata   map[string]interface{} `json:"metadata"`
	Version    int                    `json:"version"`
	CreatedAt  time.Time              `json:"created_at"`
}

// CreateDocumentRequest 创建（摄取）知识文档请求
type CreateDocumentRequest struct {
	Title         string                 `json:"title"`
	Content       string                 `json:"content"`
	Source        string                 `json:"source"`
	SourceURL     string                 `json:"source_url"`
	SourceType    string                 `json:"source_type"`    // manual / official / interview_bank / docs
	EffectiveFrom string                 `json:"effective_from"` // RFC3339，可选
	EffectiveTo   string                 `json:"effective_to"`   // RFC3339，可选
	Metadata      map[string]interface{} `json:"metadata"`       // {"domain","topic","difficulty","content_type"}
}

// SearchRequest 混合检索请求
type SearchRequest struct {
	Query  string `json:"query"`
	Domain string `json:"domain"` // 可选，按 metadata->>'domain' 过滤
	TopK   int    `json:"top_k"`
	// Caller 调用方标识（仅内部用于指标标签，不参与 JSON 协议）
	Caller string `json:"-"`
}

// RetrievedChunk 检索召回结果（保留 source/metadata，见 docs/RAG.md #6）
type RetrievedChunk struct {
	ChunkID    string                 `json:"chunk_id"`
	DocumentID string                 `json:"document_id"`
	Seq        int                    `json:"seq"`
	Content    string                 `json:"content"`
	Score      float64                `json:"score"`    // RRF 融合分数
	Channels   []string               `json:"channels"` // 命中的检索通道：vector / fts / keyword
	Metadata   map[string]interface{} `json:"metadata"`
}

// SearchResult 检索结果
type SearchResult struct {
	Query   string           `json:"query"`
	Results []RetrievedChunk `json:"results"`
}
