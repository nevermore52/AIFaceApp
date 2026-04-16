DROP INDEX IF EXISTS idx_gallery_ideas_source_id;
ALTER TABLE gallery_ideas DROP COLUMN IF EXISTS source_id;
