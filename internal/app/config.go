package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const appConfigDirName = ".adm"

type Config struct {
	MirrorAlwaysOnTop map[string]bool   `json:"mirror_always_on_top,omitempty"`
	ToolPaths         map[string]string `json:"tool_paths"`
	Aliases           map[string]string `json:"aliases"`
	LastDeviceKey     string            `json:"last_device_key,omitempty"`
	LastAPKSource     string            `json:"last_apk_source,omitempty"`
	LastPackageName   string            `json:"last_package_name,omitempty"`
}

func defaultConfig() *Config {
	return &Config{
		ToolPaths: map[string]string{},
		Aliases:   map[string]string{},
	}
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, appConfigDirName), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func loadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig(), nil
	}
	if err != nil {
		return nil, err
	}

	cfg := defaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.ToolPaths == nil {
		cfg.ToolPaths = map[string]string{}
	}
	if cfg.Aliases == nil {
		cfg.Aliases = map[string]string{}
	}
	return cfg, nil
}

func saveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
