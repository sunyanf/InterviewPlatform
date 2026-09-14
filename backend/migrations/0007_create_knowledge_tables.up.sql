-- RAG：知识文档表（来源必须可追踪，见 AGENTS.md #14/#16）
CREATE TABLE IF NOT EXISTS knowledge_documents (
    id             VARCHAR(64)  PRIMARY KEY,
    title          VARCHAR(255) NOT NULL,
    source         VARCHAR(64)  NOT NULL,                     -- 来源标识，禁止来源不明的内容
    source_url     VARCHAR(512),
    source_type    VARCHAR(32)  NOT NULL DEFAULT 'manual',    -- manual / official / interview_bank / docs
    version        INT          NOT NULL DEFAULT 1,
    effective_from TIMESTAMPTZ,                               -- 生效时间（政策/考试类知识必需）
    effective_to   TIMESTAMPTZ,                               -- 失效时间
    status         VARCHAR(32)  NOT NULL DEFAULT 'active',    -- active / archived
    metadata       JSONB        NOT NULL DEFAULT '{}',        -- {"domain","topic","difficulty","content_type"}
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_documents_status ON knowledge_documents(status);
CREATE INDEX IF NOT EXISTS idx_knowledge_documents_metadata ON knowledge_documents USING GIN(metadata);

-- 知识分块表（pgvector 向量 + 全文检索）
CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id           VARCHAR(64)  PRIMARY KEY,
    document_id  VARCHAR(64)  NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    seq          INT          NOT NULL,
    content      TEXT         NOT NULL,
    embedding    vector(1536) NOT NULL,
    metadata     JSONB        NOT NULL DEFAULT '{}',
    version      INT          NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(document_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_document ON knowledge_chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_embedding ON knowledge_chunks USING hnsw (embedding vector_cosine_ops);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_fts ON knowledge_chunks USING GIN (to_tsvector('simple', content));
