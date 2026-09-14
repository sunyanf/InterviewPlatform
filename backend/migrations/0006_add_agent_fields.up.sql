-- Agent 模块：回答分析结果、会话元数据（开场白等）
ALTER TABLE interview_answers ADD COLUMN IF NOT EXISTS analysis JSONB NOT NULL DEFAULT '{}';
ALTER TABLE interview_sessions ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';
