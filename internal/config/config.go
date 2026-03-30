package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const appName = "portainerctl"

type Config struct {
	URL                  string `json:"url,omitempty"`
	Username             string `json:"username,omitempty"`
	APIKey               string `json:"api_key,omitempty"`
	DefaultEnvironmentID int    `json:"default_environment_id,omitempty"`
	DefaultEnvironment   string `json:"default_environment,omitempty"`
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.DefaultEnvironment = strings.TrimSpace(cfg.DefaultEnvironment)
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}

	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.DefaultEnvironment = strings.TrimSpace(cfg.DefaultEnvironment)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func Clear() error {
	path, err := Path()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove config: %w", err)
	}
	return nil
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, appName, "config.json"), nil
}
