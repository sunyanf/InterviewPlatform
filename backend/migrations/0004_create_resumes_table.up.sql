-- 简历表
CREATE TABLE IF NOT EXISTS resumes (
    id              VARCHAR(64) PRIMARY KEY,
    user_id         VARCHAR(64) NOT NULL REFERENCES users(id),
    file_url        VARCHAR(512) NOT NULL,
    file_name       VARCHAR(255) NOT NULL,
    file_type       VARCHAR(32)  NOT NULL,
    file_size       BIGINT       NOT NULL DEFAULT 0,
    parsed_content  TEXT,
    structured_data JSONB        NOT NULL DEFAULT '{}',
    skills          JSONB        NOT NULL DEFAULT '[]',
    status          VARCHAR(32)  NOT NULL DEFAULT 'pending', -- pending / parsed / failed
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_resumes_user_id ON resumes(user_id);
CREATE INDEX IF NOT EXISTS idx_resumes_status ON resumes(status);
CREATE INDEX IF NOT EXISTS idx_resumes_skills ON resumes USING GIN(skills);
