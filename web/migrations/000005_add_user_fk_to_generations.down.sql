-- Remove foreign key constraint
ALTER TABLE generation_requests
DROP CONSTRAINT IF EXISTS fk_generation_requests_user;
