package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ConnectionDetails struct {
	Host   string `json:"host"`
	Port   string `json:"port"`
	User   string `json:"user"`
	DBName string `json:"dbname"`
}

type Config struct {
	AdminConnection *ConnectionDetails `json:"admin_connection"`
}

func getConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	maximDir := filepath.Join(configDir, "maxim")
	return filepath.Join(maximDir, "config.json"), nil
}

func SaveAdminConnection(details ConnectionDetails, password string) error {
	// Save all connection details except password
	cfg := Config{
		AdminConnection: &details,
	}

	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}

	file, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, file, 0600)
}

func LoadAdminConnection() (*ConnectionDetails, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}

	if cfg.AdminConnection == nil {
		return nil, os.ErrNotExist
	}

	return cfg.AdminConnection, nil
}
