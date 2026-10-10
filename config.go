package main

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const defaultMinBackupIntervalMinutes = 12 * 60

type Config struct {
	APIKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"`
	LogLevel string `yaml:"log_level"`
	// servers whose newest successful backup is younger than this are skipped, 0 always backs up
	MinBackupIntervalMinutes int `yaml:"min_backup_interval_minutes"`
}

// loadConfig reads app.yaml from dir, unknown keys are an error so typos don't go unnoticed
func loadConfig(dir string) (config Config, err error) {
	file, err := os.Open(filepath.Join(dir, "app.yaml"))
	if err != nil {
		return
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	// keys missing from the file keep these defaults
	config.MinBackupIntervalMinutes = defaultMinBackupIntervalMinutes

	err = decoder.Decode(&config)
	if err != nil {
		return
	}

	if config.MinBackupIntervalMinutes < 0 {
		err = fmt.Errorf("min_backup_interval_minutes cannot be negative, got %d", config.MinBackupIntervalMinutes)
	}

	return
}
