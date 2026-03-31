ALTER TABLE generation_requests ADD COLUMN IF NOT EXISTS source VARCHAR(10) DEFAULT 'bot';
UPDATE generation_requests SET source = 'bot' WHERE source IS NULL;
