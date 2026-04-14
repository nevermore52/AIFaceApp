CREATE TABLE IF NOT EXISTS trends (
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL DEFAULT '',
    output      TEXT         NOT NULL,
    prompt      TEXT         NOT NULL DEFAULT '',
    model       VARCHAR(255) NOT NULL DEFAULT '',
    is_popular  BOOLEAN      NOT NULL DEFAULT FALSE,
    priority    INT          NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_trends_priority
    ON trends (priority) WHERE priority IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_trends_created_at ON trends (created_at DESC);
