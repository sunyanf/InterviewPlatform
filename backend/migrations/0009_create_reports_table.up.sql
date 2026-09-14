-- 报告表：一场面试一份报告（重跑覆盖）
-- 总分与能力画像为确定性计算（复制评估结果 + 历史对比），LLM 只产优点/不足/学习计划（AGENTS.md #15）
CREATE TABLE IF NOT EXISTS reports (
    id                 VARCHAR(64)  PRIMARY KEY,
    session_id         VARCHAR(64)  NOT NULL UNIQUE REFERENCES interview_sessions(id) ON DELETE CASCADE,
    user_id            VARCHAR(64)  NOT NULL,
    total_score        NUMERIC(5,2) NOT NULL,
    capability_profile JSONB        NOT NULL,
    strengths          JSONB        NOT NULL DEFAULT '[]',
    weaknesses         JSONB        NOT NULL DEFAULT '[]',
    knowledge_gaps     JSONB        NOT NULL DEFAULT '[]',
    learning_plan      JSONB        NOT NULL,
    prompt_version     VARCHAR(64)  NOT NULL,
    model              VARCHAR(64)  NOT NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reports_user ON reports (user_id, created_at DESC);
