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
	AdminConnection     *ConnectionDetails            `json:"admin_connection"`
	DatabaseConnections map[string]*ConnectionDetails `json:"database_connections"`
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

// SaveDatabaseConnection saves a regular database connection (not admin)
func SaveDatabaseConnection(connectionName string, details ConnectionDetails, password string) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Load existing config or create new one
	var cfg Config
	if file, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(file, &cfg)
	}

	// Initialize database connections map if nil
	if cfg.DatabaseConnections == nil {
		cfg.DatabaseConnections = make(map[string]*ConnectionDetails)
	}

	// Save the connection details (except password)
	cfg.DatabaseConnections[connectionName] = &details

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}

	file, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, file, 0600)
}

// LoadDatabaseConnection loads a regular database connection by name
func LoadDatabaseConnection(connectionName string) (*ConnectionDetails, error) {
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

	if cfg.DatabaseConnections == nil {
		return nil, os.ErrNotExist
	}

	details, exists := cfg.DatabaseConnections[connectionName]
	if !exists {
		return nil, os.ErrNotExist
	}

	return details, nil
}

// ListDatabaseConnections returns all saved database connection names
func ListDatabaseConnections() ([]string, error) {
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

	if cfg.DatabaseConnections == nil {
		return []string{}, nil
	}

	var names []string
	for name := range cfg.DatabaseConnections {
		names = append(names, name)
	}

	return names, nil
}
