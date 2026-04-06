CREATE TABLE IF NOT EXISTS user_uploads (
    id        BIGSERIAL PRIMARY KEY,
    user_id   BIGINT NOT NULL,
    url       TEXT NOT NULL,
    filename  TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_uploads_user_id ON user_uploads(user_id);
CREATE INDEX IF NOT EXISTS idx_user_uploads_created_at ON user_uploads(created_at);
