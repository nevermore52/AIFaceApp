package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

func InitDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func RunMigrations(db *sql.DB) error {
	userTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		telegram_id BIGINT UNIQUE NOT NULL,
		username VARCHAR(255),
		first_name VARCHAR(255),
		last_name VARCHAR(255),
		language_code VARCHAR(10),
		is_premium BOOLEAN DEFAULT FALSE,
		is_admin BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		is_blocked BOOLEAN DEFAULT FALSE
	);`

	generationRequestTableSQL := `
	CREATE TABLE IF NOT EXISTS generation_requests (
		id SERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
		type VARCHAR(50) NOT NULL,
		status VARCHAR(50) DEFAULT 'pending',
		input_image TEXT,
		output_image TEXT,
		prompt TEXT,
		error_msg TEXT,
		tokens_used INTEGER DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMP WITH TIME ZONE NULL
	);`

	categorySettingsSQL := `
	CREATE TABLE IF NOT EXISTS category_settings (
		category VARCHAR(50) PRIMARY KEY,
		enabled BOOLEAN DEFAULT TRUE
	);`

	appSettingsSQL := `
	CREATE TABLE IF NOT EXISTS app_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	INSERT INTO app_settings (key, value) VALUES ('payments_enabled', 'true')
	ON CONFLICT (key) DO NOTHING;`

	userQuotasSQL := `
	CREATE TABLE IF NOT EXISTS user_quotas (
		id SERIAL PRIMARY KEY,
		telegram_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
		text_daily INTEGER DEFAULT 0,
		text_extra INTEGER DEFAULT 0,
		image_weekly INTEGER DEFAULT 0,
		image_extra INTEGER DEFAULT 0,
		music_weekly INTEGER DEFAULT 0,
		music_extra INTEGER DEFAULT 0,
		video_weekly INTEGER DEFAULT 0,
		video_extra INTEGER DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (telegram_id)
	);`

	referralMigrationSQL := `
	ALTER TABLE users ADD COLUMN IF NOT EXISTS referrer_id BIGINT REFERENCES users(telegram_id);
	ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code VARCHAR(20) UNIQUE;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS referrals_count INTEGER DEFAULT 0;`

	dropTokensSQL := `
	ALTER TABLE users DROP COLUMN IF EXISTS tokens;
	DROP TABLE IF EXISTS token_transactions;`

	upgradeQuotasSQL := `
	ALTER TABLE user_quotas ADD COLUMN IF NOT EXISTS text_extra INTEGER DEFAULT 0;
	ALTER TABLE user_quotas ADD COLUMN IF NOT EXISTS image_extra INTEGER DEFAULT 0;
	ALTER TABLE user_quotas ADD COLUMN IF NOT EXISTS music_extra INTEGER DEFAULT 0;
	ALTER TABLE user_quotas ADD COLUMN IF NOT EXISTS video_extra INTEGER DEFAULT 0;
	-- Переименовываем месячные лимиты в недельные
	DO $$
	BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'user_quotas' AND column_name = 'image_monthly') THEN
			ALTER TABLE user_quotas RENAME COLUMN image_monthly TO image_weekly;
		END IF;
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'user_quotas' AND column_name = 'music_monthly') THEN
			ALTER TABLE user_quotas RENAME COLUMN music_monthly TO music_weekly;
		END IF;
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'user_quotas' AND column_name = 'video_monthly') THEN
			ALTER TABLE user_quotas RENAME COLUMN video_monthly TO video_weekly;
		END IF;
	END $$;`

	subscriptionSQL := `
	ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_type VARCHAR(20) DEFAULT '';
	ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_started_at TIMESTAMP WITH TIME ZONE;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_end TIMESTAMP WITH TIME ZONE;`

	indexesSQL := `
	CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);
	CREATE INDEX IF NOT EXISTS idx_users_is_admin ON users(is_admin);
	CREATE INDEX IF NOT EXISTS idx_users_referral_code ON users(referral_code);
	CREATE INDEX IF NOT EXISTS idx_generation_requests_user_id ON generation_requests(user_id);
	CREATE INDEX IF NOT EXISTS idx_generation_requests_status ON generation_requests(status);
	CREATE INDEX IF NOT EXISTS idx_user_quotas_telegram_id ON user_quotas(telegram_id);`

	tables := []string{
		userTableSQL,
		generationRequestTableSQL,
		categorySettingsSQL,
		appSettingsSQL,
		userQuotasSQL,
		referralMigrationSQL,
		dropTokensSQL,
		upgradeQuotasSQL,
		subscriptionSQL,
		indexesSQL,
	}

	for _, sql := range tables {
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}

	return nil
}
