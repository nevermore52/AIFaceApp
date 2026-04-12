ALTER TABLE gallery_ideas ADD COLUMN IF NOT EXISTS priority INT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_gallery_ideas_priority
    ON gallery_ideas (priority)
    WHERE priority IS NOT NULL;
