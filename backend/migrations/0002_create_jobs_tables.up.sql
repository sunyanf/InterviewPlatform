-- 岗位分类表
CREATE TABLE IF NOT EXISTS job_categories (
    id          VARCHAR(64) PRIMARY KEY,
    name        VARCHAR(128) NOT NULL UNIQUE,
    description TEXT,
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 岗位表
CREATE TABLE IF NOT EXISTS jobs (
    id           VARCHAR(64) PRIMARY KEY,
    category_id  VARCHAR(64) NOT NULL REFERENCES job_categories(id),
    title        VARCHAR(255) NOT NULL,
    description  TEXT,
    requirements JSONB NOT NULL DEFAULT '[]',
    skills       JSONB NOT NULL DEFAULT '[]',
    source       VARCHAR(64),
    source_url   VARCHAR(512),
    status       VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jobs_category_id ON jobs(category_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_skills ON jobs USING GIN(skills);
