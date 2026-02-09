package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Environment string `json:"environment"`
	Server      struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	MongoDB struct {
		URI      string `json:"uri"`
		Database string `json:"database"`
	} `json:"mongodb"`
	Frontend struct {
		URL string `json:"url"`
	} `json:"frontend"`
	JWT struct {
		AccessSecret  string `json:"accessSecret"`
		RefreshSecret string `json:"refreshSecret"`
		AccessTTL     int    `json:"accessTtl"`  // in minutes
		RefreshTTL    int    `json:"refreshTtl"` // in days
	} `json:"jwt"`
	OAuth struct {
		GoogleClientID     string `json:"googleClientId"`
		GoogleClientSecret string `json:"googleClientSecret"`
		GoogleRedirectURL  string `json:"googleRedirectUrl"`
	} `json:"oauth"`
}

func Load(env string) (*Config, error) {
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		// Default to configs directory relative to working directory
		configDir = "configs"
	}

	filename := fmt.Sprintf("config.%s.json", env)
	configPath := filepath.Join(configDir, filename)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Replace environment variables in the config
	configStr := string(data)
	configStr = expandEnvVars(configStr)

	var cfg Config
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.Environment = env
	return &cfg, nil
}

// expandEnvVars replaces ${VAR_NAME} with environment variable values
func expandEnvVars(s string) string {
	return os.Expand(s, func(key string) string {
		return os.Getenv(key)
	})
}

func GetEnv() string {
	env := os.Getenv("CHESS_ENV")
	if env == "" {
		return "dev"
	}
	return env
}
