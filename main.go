package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/marte26/pterodactylBackup/pterodactylapi/adminapi"
	"github.com/marte26/pterodactylBackup/pterodactylapi/clientapi"
)

const workers = 2

var commitHash = "unknown"

func main() {
	var logLevel slog.LevelVar
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &logLevel})))

	slog.Info("starting pterodactylBackup", "commit", commitHash)

	exPath, err := os.Executable()
	if err != nil {
		fatal("cannot determine executable path", "error", err)
	}

	config, err := loadConfig(filepath.Dir(exPath))
	if err != nil {
		fatal("cannot load config", "error", err)
	}

	if config.LogLevel != "" {
		err := logLevel.UnmarshalText([]byte(config.LogLevel))
		if err != nil {
			fatal("invalid log_level", "value", config.LogLevel, "error", err)
		}
	}

	adminAPI := adminapi.Client{
		URL:    config.BaseURL,
		APIKey: config.APIKey,
	}
	clientAPI := clientapi.Client{
		URL:    config.BaseURL,
		APIKey: config.APIKey,
	}

	servers, err := adminAPI.GetServers()
	if err != nil {
		fatal("cannot list servers", "error", err)
	}
	minInterval := time.Duration(config.MinBackupIntervalMinutes) * time.Minute
	slog.Info("starting backups", "servers", len(servers), "workers", workers, "min_interval", minInterval)

	result := createBackupBatch(&clientAPI, servers, workers, minInterval)

	summary := []any{
		"succeeded", result.succeeded.Load(),
		"failed", result.failed.Load(),
		"skipped", result.skipped.Load(),
	}
	if result.failed.Load() > 0 {
		fatal("backup run finished with failures", summary...)
	}
	slog.Info("backup run finished", summary...)
}

// fatal logs msg as an error and exits with status 1
func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
