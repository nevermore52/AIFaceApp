package services

import (
	"database/sql"
	"fmt"
)

type SettingsService struct {
	db *sql.DB
}

func NewSettingsService(db *sql.DB) *SettingsService {
	return &SettingsService{db: db}
}

func (s *SettingsService) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM system_settings WHERE key = $1`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("setting %s not found", key)
	}
	return value, err
}

func (s *SettingsService) IsMaintenanceMode() (bool, error) {
	value, err := s.GetSetting("maintenance_mode")
	if err != nil {
		// Если настройка не найдена, считаем что техработ нет
		return false, nil
	}
	return value == "true", nil
}

func (s *SettingsService) GetMaintenanceMessage() (string, error) {
	message, err := s.GetSetting("maintenance_message")
	if err != nil {
		return "Сервис временно недоступен. Ведутся технические работы. Пожалуйста, попробуйте позже.", nil
	}
	return message, nil
}
