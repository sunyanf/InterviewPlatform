-- 更优答案示范结果表：回答一经提交不可变，对应示范结果持久化后永久复用（缓存不过期），
-- 避免同一题重复调用 LLM（成本与延迟优化，见 interview.Service.CoachQuestion）。
CREATE TABLE IF NOT EXISTS answer_coachings (
    id              VARCHAR(64)  PRIMARY KEY,
    session_id      VARCHAR(64)  NOT NULL REFERENCES interview_sessions(id) ON DELETE CASCADE,
    question_id     VARCHAR(64)  NOT NULL REFERENCES interview_questions(id) ON DELETE CASCADE,
    improved_answer TEXT         NOT NULL,
    key_changes     JSONB        NOT NULL DEFAULT '[]'::jsonb,
    prompt_version  TEXT         NOT NULL,
    model           TEXT         NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_answer_coachings_question UNIQUE (question_id)
);

-- 按会话查询示范结果
CREATE INDEX IF NOT EXISTS idx_answer_coachings_session ON answer_coachings (session_id);
