-- Refresh Token 表：只存 SHA-256 哈希，原始 token 仅在登录响应中出现一次。
-- 旋转（rotation）：每次刷新吊销旧 token、签发新 token；
-- 已吊销 token 被再次使用视为令牌被盗，吊销该用户全部活跃 token。
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id           VARCHAR(64)  PRIMARY KEY,
    user_id      VARCHAR(64)  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   CHAR(64)     NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ  NOT NULL,
    revoked_at   TIMESTAMPTZ,
    user_agent   VARCHAR(256) NOT NULL DEFAULT '',
    client_ip    VARCHAR(64)  NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

-- 列出/批量吊销某用户的 token
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens (user_id);
-- 定期清理过期 token
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires ON refresh_tokens (expires_at);
