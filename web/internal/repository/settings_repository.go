package repository

import (
	"database/sql"
	"fmt"
)

type SettingsRepository struct {
	db *sql.DB
}

func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func (r *SettingsRepository) GetSetting(key string) (string, error) {
	var value string
	err := r.db.QueryRow(`SELECT value FROM system_settings WHERE key = $1`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("setting %s not found", key)
	}
	return value, err
}

func (r *SettingsRepository) SetSetting(key, value string) error {
	_, err := r.db.Exec(`
		INSERT INTO system_settings (key, value, updated_at) 
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) 
		DO UPDATE SET value = $2, updated_at = NOW()
	`, key, value)
	return err
}

func (r *SettingsRepository) IsMaintenanceMode() (bool, error) {
	value, err := r.GetSetting("maintenance_mode")
	if err != nil {
		return false, err
	}
	return value == "true", nil
}

func (r *SettingsRepository) GetMaintenanceMessage() (string, error) {
	return r.GetSetting("maintenance_message")
}

func (r *SettingsRepository) SetMaintenanceMode(enabled bool, message string) error {
	modeValue := "false"
	if enabled {
		modeValue = "true"
	}

	if err := r.SetSetting("maintenance_mode", modeValue); err != nil {
		return err
	}

	if message != "" {
		if err := r.SetSetting("maintenance_message", message); err != nil {
			return err
		}
	}

	return nil
}
