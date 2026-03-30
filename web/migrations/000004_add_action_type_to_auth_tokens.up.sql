ALTER TABLE web_auth_tokens ADD COLUMN action_type VARCHAR(20) DEFAULT 'auth';
ALTER TABLE web_auth_tokens ADD COLUMN user_id BIGINT;
