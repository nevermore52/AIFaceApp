-- Add foreign key constraint to generation_requests
-- Simply add the constraint - data cleanup should be done manually if needed
ALTER TABLE generation_requests
ADD CONSTRAINT fk_generation_requests_user
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
