CREATE TABLE IF NOT EXISTS gallery_ideas (
    id SERIAL PRIMARY KEY,
    model VARCHAR(255) NOT NULL,
    output TEXT NOT NULL,
    prompt TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_gallery_ideas_created_at ON gallery_ideas(created_at DESC);
