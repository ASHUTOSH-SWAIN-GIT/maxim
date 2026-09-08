package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

type ConnectionDetails struct {
	Host    string `json:"host"`
	Port    string `json:"port"`
	User    string `json:"user"`
	DBName  string `json:"dbname"`
	SSLMode string `json:"sslmode,omitempty"`
}

type Config struct {
	AdminConnection     *ConnectionDetails            `json:"admin_connection"`
	DatabaseConnections map[string]*ConnectionDetails `json:"database_connections"`
}

func getConfigPath() (string, error) {
	if configHome := os.Getenv("MAXIM_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "config.json"), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	maximDir := filepath.Join(configDir, "maxim")
	return filepath.Join(maximDir, "config.json"), nil
}

func loadConfig() (Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return Config{}, err
	}

	file, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(file, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func saveConfig(cfg Config) error {
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

	temp, err := os.CreateTemp(filepath.Dir(configPath), "config-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(file); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, configPath)
}

func SaveAdminConnection(details ConnectionDetails, _ string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.AdminConnection = &details
	return saveConfig(cfg)
}

func LoadAdminConnection() (*ConnectionDetails, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	if cfg.AdminConnection == nil {
		return nil, os.ErrNotExist
	}

	return cfg.AdminConnection, nil
}

// SaveDatabaseConnection saves a regular database connection (not admin)
func SaveDatabaseConnection(connectionName string, details ConnectionDetails, _ string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Initialize database connections map if nil
	if cfg.DatabaseConnections == nil {
		cfg.DatabaseConnections = make(map[string]*ConnectionDetails)
	}

	// Save the connection details (except password)
	cfg.DatabaseConnections[connectionName] = &details

	return saveConfig(cfg)
}

// LoadDatabaseConnection loads a regular database connection by name
func LoadDatabaseConnection(connectionName string) (*ConnectionDetails, error) {
	cfg, err := loadConfig()
	if err != nil {
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
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	if cfg.DatabaseConnections == nil {
		return []string{}, nil
	}

	var names []string
	for name := range cfg.DatabaseConnections {
		names = append(names, name)
	}
	sort.Strings(names)

	return names, nil
}

// DeleteDatabaseConnection removes a saved connection profile.
func DeleteDatabaseConnection(connectionName string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if _, exists := cfg.DatabaseConnections[connectionName]; !exists {
		return os.ErrNotExist
	}
	delete(cfg.DatabaseConnections, connectionName)
	return saveConfig(cfg)
}

// RenameDatabaseConnection changes a saved profile's display name.
func RenameDatabaseConnection(oldName, newName string) error {
	if newName == "" {
		return errors.New("connection name cannot be empty")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	details, exists := cfg.DatabaseConnections[oldName]
	if !exists {
		return os.ErrNotExist
	}
	if _, exists := cfg.DatabaseConnections[newName]; exists && oldName != newName {
		return errors.New("a connection with that name already exists")
	}
	delete(cfg.DatabaseConnections, oldName)
	cfg.DatabaseConnections[newName] = details
	return saveConfig(cfg)
}
