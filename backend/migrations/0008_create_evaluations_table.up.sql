-- 评估结果表：一场面试一份评估（重跑覆盖）
-- 总分由业务代码按 Rubric 权重确定性计算，LLM 只产维度分/证据/建议（见 AGENTS.md #15）
CREATE TABLE IF NOT EXISTS evaluations (
    id             VARCHAR(64)  PRIMARY KEY,
    session_id     VARCHAR(64)  NOT NULL UNIQUE REFERENCES interview_sessions(id) ON DELETE CASCADE,
    user_id        VARCHAR(64)  NOT NULL,
    dimensions     JSONB        NOT NULL,
    evidence       JSONB        NOT NULL DEFAULT '[]',
    recommendations JSONB       NOT NULL DEFAULT '[]',
    rubric         JSONB        NOT NULL,
    total_score    NUMERIC(5,2) NOT NULL,
    prompt_version VARCHAR(64)  NOT NULL,
    model          VARCHAR(64)  NOT NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_evaluations_user ON evaluations (user_id, created_at DESC);
