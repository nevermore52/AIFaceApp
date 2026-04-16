CREATE TABLE promo_codes (
    id SERIAL PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    image_tokens INT NOT NULL DEFAULT 0,
    video_tokens INT NOT NULL DEFAULT 0,
    text_tokens  INT NOT NULL DEFAULT 0,
    music_tokens INT NOT NULL DEFAULT 0,
    max_activations INT,               -- NULL = unlimited
    activations_count INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,            -- NULL = no expiry
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE promo_activations (
    id SERIAL PRIMARY KEY,
    promo_code_id INT NOT NULL REFERENCES promo_codes(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (promo_code_id, user_id)
);
