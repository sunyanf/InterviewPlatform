CREATE TABLE IF NOT EXISTS settings (
    key        VARCHAR(64)  PRIMARY KEY,
    value      TEXT         NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO settings (key, value) VALUES
    ('llm.provider',            ''),
    ('llm.api_key',             ''),
    ('llm.base_url',            ''),
    ('llm.model',               ''),
    ('llm.price_input_per_1k',  '0'),
    ('llm.price_output_per_1k', '0'),
    ('rate_limit.enabled',      'true'),
    ('rate_limit.auth_per_min', '10'),
    ('rate_limit.api_per_min',  '120'),
    ('task.enabled',            'true'),
    ('task.workers',            '2'),
    ('task.poll_interval',      '2s'),
    ('task.lease_timeout',      '5m'),
    ('task.shutdown_timeout',   '20s')
ON CONFLICT (key) DO NOTHING;
