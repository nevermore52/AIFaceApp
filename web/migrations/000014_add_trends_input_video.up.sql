ALTER TABLE trends ADD COLUMN IF NOT EXISTS input_video TEXT;

COMMENT ON COLUMN trends.input_video IS 'Input/reference video URL for motion control models';
