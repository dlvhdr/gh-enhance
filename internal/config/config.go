package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDirName  = "gh-enhance"
	ConfigFileName = "config.yml"

	defaultXDGConfigDirName = ".config"
)

type Config struct {
	Theme       string      `yaml:"theme"`
	Flat        *bool       `yaml:"flat"`
	Keybindings Keybindings `yaml:"keybindings"`
}

type Keybindings struct {
	Universal []Keybinding `yaml:"universal"`
}

type Keybinding struct {
	Builtin string `yaml:"builtin"`
	Key     string `yaml:"key"`
	Name    string `yaml:"name,omitempty"`
}

func Path() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, defaultXDGConfigDirName)
	}

	return filepath.Join(configDir, ConfigDirName, ConfigFileName), nil
}

func Load() (Config, error) {
	configPath, err := Path()
	if err != nil {
		return Config{}, err
	}

	return LoadFile(configPath)
}

func LoadFile(configPath string) (Config, error) {
	contents, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(contents, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed parsing config at %s: %w", configPath, err)
	}

	return cfg, nil
}
