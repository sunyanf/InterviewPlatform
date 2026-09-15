-- 答案语音表：一题一份录音（重传覆盖）
-- 转写文本与表达分析结果落库；日志不输出音频/转写内容（AGENTS.md #17）
CREATE TABLE IF NOT EXISTS answer_audios (
    id           VARCHAR(64)  PRIMARY KEY,
    session_id   VARCHAR(64)  NOT NULL REFERENCES interview_sessions(id) ON DELETE CASCADE,
    question_id  VARCHAR(64)  NOT NULL UNIQUE REFERENCES interview_questions(id) ON DELETE CASCADE,
    user_id      VARCHAR(64)  NOT NULL,
    object_key   VARCHAR(256) NOT NULL,
    format       VARCHAR(16)  NOT NULL,
    size_bytes   BIGINT       NOT NULL,
    duration_ms  BIGINT       NOT NULL DEFAULT 0,
    transcript   TEXT         NOT NULL DEFAULT '',
    language     VARCHAR(16)  NOT NULL DEFAULT '',
    status       VARCHAR(16)  NOT NULL DEFAULT 'uploaded',
    analysis     JSONB,
    asr_provider VARCHAR(32)  NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_answer_audios_user ON answer_audios (user_id, created_at DESC);
