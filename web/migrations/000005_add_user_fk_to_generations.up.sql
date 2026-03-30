-- Add foreign key constraint to ensure user_id references users.id
-- First, we need to update any existing generation_requests that have invalid user_ids
-- This assumes that old generations used telegram_id directly as user_id

-- Update generation_requests to use users.id instead of telegram_id
UPDATE generation_requests gr
SET user_id = u.id
FROM users u
WHERE gr.user_id = u.telegram_id AND gr.user_id != u.id;

-- Now add the foreign key constraint
ALTER TABLE generation_requests
ADD CONSTRAINT fk_generation_requests_user
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
