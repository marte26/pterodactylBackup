package main

import (
	"io"
	"log"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/marte26/pterodactylBackup/internal/fakepanel"
	"github.com/marte26/pterodactylBackup/pterodactylapi/clientapi"
	"github.com/marte26/pterodactylBackup/pterodactylapi/structs"
)

func TestMain(m *testing.M) {
	pollInterval = time.Millisecond
	backupTimeout = 100 * time.Millisecond
	log.SetOutput(io.Discard)

	os.Exit(m.Run())
}

func newServer(identifier string, backupLimit int) structs.Server {
	var server structs.Server
	server.Attributes.Identifier = identifier
	server.Attributes.Name = identifier
	server.Attributes.FeatureLimits.Backups = backupLimit

	return server
}

// existing returns a completed successful backup created on the given day
func existing(uuid string, day int) fakepanel.Backup {
	return fakepanel.Backup{
		UUID:       uuid,
		CreatedAt:  time.Date(2025, 1, day, 0, 0, 0, 0, time.UTC),
		Successful: true,
		Completed:  true,
	}
}

func locked(backup fakepanel.Backup) fakepanel.Backup {
	backup.Locked = true
	return backup
}

func TestBackupServer(t *testing.T) {
	tests := []struct {
		name           string
		limit          int
		backups        []fakepanel.Backup
		failNewBackups bool
		neverComplete  bool
		wantEvents     []string
		wantBackups    []string
		wantErr        string
	}{
		{
			name:        "free slot: create first, then delete the oldest",
			limit:       3,
			backups:     []fakepanel.Backup{existing("old-1", 1), existing("old-2", 2)},
			wantEvents:  []string{"create s/new-1", "delete s/old-1"},
			wantBackups: []string{"old-2", "new-1"},
		},
		{
			name:        "full server: free a slot once, then keep it free",
			limit:       3,
			backups:     []fakepanel.Backup{existing("old-1", 1), existing("old-2", 2), existing("old-3", 3)},
			wantEvents:  []string{"delete s/old-1", "create s/new-1", "delete s/old-2"},
			wantBackups: []string{"old-3", "new-1"},
		},
		{
			name:        "limit of 1 replaces the only backup",
			limit:       1,
			backups:     []fakepanel.Backup{existing("old-1", 1)},
			wantEvents:  []string{"delete s/old-1", "create s/new-1"},
			wantBackups: []string{"new-1"},
		},
		{
			name:        "empty server",
			limit:       2,
			wantEvents:  []string{"create s/new-1"},
			wantBackups: []string{"new-1"},
		},
		{
			name:        "locked backups are never deleted",
			limit:       3,
			backups:     []fakepanel.Backup{locked(existing("old-1", 1)), existing("old-2", 2), existing("old-3", 3)},
			wantEvents:  []string{"delete s/old-2", "create s/new-1", "delete s/old-3"},
			wantBackups: []string{"old-1", "new-1"},
		},
		{
			name:        "locked backup taking the free slot is not an error",
			limit:       2,
			backups:     []fakepanel.Backup{locked(existing("old-1", 1)), existing("old-2", 2)},
			wantEvents:  []string{"delete s/old-2", "create s/new-1"},
			wantBackups: []string{"old-1", "new-1"},
		},
		{
			name:        "server full of locked backups fails without deleting",
			limit:       1,
			backups:     []fakepanel.Backup{locked(existing("old-1", 1))},
			wantBackups: []string{"old-1"},
			wantErr:     "reached its limit",
		},
		{
			name:  "in progress backups count but are not deleted",
			limit: 2,
			backups: []fakepanel.Backup{
				existing("old-1", 1),
				{UUID: "running", CreatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)},
			},
			wantEvents:  []string{"delete s/old-1", "create s/new-1"},
			wantBackups: []string{"running", "new-1"},
		},
		{
			name:  "failed backups do not count",
			limit: 2,
			backups: []fakepanel.Backup{
				{UUID: "failed", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Completed: true},
				existing("old-2", 2),
			},
			wantEvents:  []string{"create s/new-1", "delete s/old-2"},
			wantBackups: []string{"failed", "new-1"},
		},
		{
			name:           "failed new backup is deleted and old ones are kept",
			limit:          3,
			backups:        []fakepanel.Backup{existing("old-1", 1), existing("old-2", 2)},
			failNewBackups: true,
			wantEvents:     []string{"create s/new-1", "delete s/new-1"},
			wantBackups:    []string{"old-1", "old-2"},
			wantErr:        "failed and was deleted",
		},
		{
			name:          "backup that never completes times out and keeps old ones",
			limit:         3,
			backups:       []fakepanel.Backup{existing("old-1", 1)},
			neverComplete: true,
			wantEvents:    []string{"create s/new-1"},
			wantBackups:   []string{"old-1", "new-1"},
			wantErr:       "did not complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panel := &fakepanel.Panel{
				Limits:         map[string]int{"s": tt.limit},
				FailNewBackups: tt.failNewBackups,
				NeverComplete:  tt.neverComplete,
			}
			for _, backup := range tt.backups {
				panel.AddBackup("s", backup)
			}
			clientAPI := &clientapi.Client{URL: panel.Start(t)}

			err := backupServer(0, clientAPI, newServer("s", tt.limit))

			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
			}
			if events := panel.Events(); !slices.Equal(events, tt.wantEvents) {
				t.Errorf("events = %v, want %v", events, tt.wantEvents)
			}
			if backups := panel.Backups("s"); !slices.Equal(backups, tt.wantBackups) {
				t.Errorf("backups = %v, want %v", backups, tt.wantBackups)
			}
		})
	}
}

func TestCreateBackupBatch(t *testing.T) {
	tests := []struct {
		name           string
		failNewBackups bool
		wantFailed     int
	}{
		{name: "all succeed", wantFailed: 0},
		// more failures than workers used to leave no worker to receive the next server
		{name: "all fail", failNewBackups: true, wantFailed: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panel := &fakepanel.Panel{
				Limits:         map[string]int{"a": 2, "b": 2, "c": 2, "d": 2, "none": 0},
				FailNewBackups: tt.failNewBackups,
			}
			clientAPI := &clientapi.Client{URL: panel.Start(t)}
			servers := []structs.Server{
				newServer("a", 2), newServer("none", 0), newServer("b", 2), newServer("c", 2), newServer("d", 2),
			}

			done := make(chan int)
			go func() { done <- createBackupBatch(clientAPI, servers, 2) }()

			select {
			case failed := <-done:
				if failed != tt.wantFailed {
					t.Errorf("failed = %d, want %d", failed, tt.wantFailed)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("createBackupBatch did not return")
			}

			for _, event := range panel.Events() {
				if strings.Contains(event, " none/") {
					t.Errorf("server without backups allowed was touched: %v", event)
				}
			}
			if !tt.failNewBackups {
				for _, server := range []string{"a", "b", "c", "d"} {
					if len(panel.Backups(server)) != 1 {
						t.Errorf("server %v has backups %v, want exactly one", server, panel.Backups(server))
					}
				}
			}
		})
	}
}
