package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    Config
		wantErr string
	}{
		{
			name:    "all keys",
			content: "api_key: key\nbase_url: https://panel.example.com\nlog_level: debug\nmin_backup_interval_minutes: 60\n",
			want:    Config{APIKey: "key", BaseURL: "https://panel.example.com", LogLevel: "debug", MinBackupIntervalMinutes: 60},
		},
		{
			name:    "min backup interval defaults to 12 hours",
			content: "api_key: key\n",
			want:    Config{APIKey: "key", MinBackupIntervalMinutes: 720},
		},
		{
			name:    "min backup interval of 0 disables the check",
			content: "min_backup_interval_minutes: 0\n",
			want:    Config{MinBackupIntervalMinutes: 0},
		},
		{
			name:    "negative min backup interval",
			content: "min_backup_interval_minutes: -5\n",
			wantErr: "cannot be negative",
		},
		{
			name:    "unknown key",
			content: "api_key: key\nbaseurl: https://panel.example.com\n",
			wantErr: "field baseurl not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			err := os.WriteFile(filepath.Join(dir, "app.yaml"), []byte(tt.content), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			got, err := loadConfig(dir)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got error %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := loadConfig(t.TempDir())
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("got error %v, want fs.ErrNotExist", err)
	}
}
