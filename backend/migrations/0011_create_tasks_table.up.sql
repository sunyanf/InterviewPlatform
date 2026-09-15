-- 异步任务表：简历解析 / 评估 / 报告生成等长耗时 LLM 调用任务化
-- 多 worker 通过 FOR UPDATE SKIP LOCKED 并发认领；幂等键防止重复入队
CREATE TABLE IF NOT EXISTS tasks (
    id              VARCHAR(64)  PRIMARY KEY,
    task_type       VARCHAR(64)  NOT NULL,
    payload         JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending',
    priority        INT          NOT NULL DEFAULT 0,
    attempts        INT          NOT NULL DEFAULT 0,
    max_attempts    INT          NOT NULL DEFAULT 3,
    run_after       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    locked_by       VARCHAR(64)  NOT NULL DEFAULT '',
    locked_at       TIMESTAMPTZ,
    last_error      TEXT         NOT NULL DEFAULT '',
    idempotency_key VARCHAR(160) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 认领索引：待执行 + 到期任务，按优先级与创建时间排序
CREATE INDEX IF NOT EXISTS idx_tasks_claim ON tasks (status, run_after);

-- 幂等：同一幂等键只允许一个 pending/running 任务（succeeded/failed 可重新入队）
CREATE UNIQUE INDEX IF NOT EXISTS uq_tasks_idempotency
    ON tasks (idempotency_key)
    WHERE idempotency_key <> '' AND status IN ('pending', 'running');
