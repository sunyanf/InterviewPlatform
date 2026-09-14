-- 面试会话表
CREATE TABLE IF NOT EXISTS interview_sessions (
    id             VARCHAR(64)  PRIMARY KEY,
    user_id        VARCHAR(64)  NOT NULL REFERENCES users(id),
    job_id         VARCHAR(64)  NOT NULL REFERENCES jobs(id),
    resume_id      VARCHAR(64)  REFERENCES resumes(id),
    interview_type VARCHAR(32)  NOT NULL DEFAULT 'technical', -- technical / behavioral / mixed
    mode           VARCHAR(32)  NOT NULL DEFAULT 'text',      -- text / voice
    status         VARCHAR(32)  NOT NULL DEFAULT 'INIT',      -- INIT / RUNNING / PAUSED / FINISHING / EVALUATING / COMPLETED / FAILED
    config         JSONB        NOT NULL DEFAULT '{}',        -- {"question_count":5,"duration_minutes":30}
    started_at     TIMESTAMPTZ,
    ended_at       TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_interview_sessions_user_id ON interview_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_interview_sessions_job_id ON interview_sessions(job_id);
CREATE INDEX IF NOT EXISTS idx_interview_sessions_status ON interview_sessions(status);

-- 面试问题表
CREATE TABLE IF NOT EXISTS interview_questions (
    id             VARCHAR(64)  PRIMARY KEY,
    session_id     VARCHAR(64)  NOT NULL REFERENCES interview_sessions(id) ON DELETE CASCADE,
    seq            INT          NOT NULL,
    question_type  VARCHAR(32)  NOT NULL DEFAULT 'technical', -- technical / behavioral / project / follow_up
    question       TEXT         NOT NULL,
    difficulty     VARCHAR(32)  NOT NULL DEFAULT 'medium',    -- easy / medium / hard
    source         VARCHAR(32)  NOT NULL DEFAULT 'llm',       -- llm / static
    expected_points JSONB       NOT NULL DEFAULT '[]',
    metadata       JSONB        NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(session_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_interview_questions_session_id ON interview_questions(session_id);

-- 面试回答表
CREATE TABLE IF NOT EXISTS interview_answers (
    id           VARCHAR(64)  PRIMARY KEY,
    session_id   VARCHAR(64)  NOT NULL REFERENCES interview_sessions(id) ON DELETE CASCADE,
    question_id  VARCHAR(64)  NOT NULL UNIQUE REFERENCES interview_questions(id) ON DELETE CASCADE,
    input_type   VARCHAR(32)  NOT NULL DEFAULT 'text', -- text / voice
    text_content TEXT         NOT NULL,
    media_url    VARCHAR(512),
    duration_ms  INT,
    transcript   TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_interview_answers_session_id ON interview_answers(session_id);
