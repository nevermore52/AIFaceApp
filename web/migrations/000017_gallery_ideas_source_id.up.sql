ALTER TABLE gallery_ideas ADD COLUMN IF NOT EXISTS source_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_gallery_ideas_source_id ON gallery_ideas(source_id) WHERE source_id IS NOT NULL;
