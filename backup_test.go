package main

import (
	"bytes"
	"log/slog"
	"net/http"
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
	slog.SetDefault(slog.New(slog.DiscardHandler))

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
		name            string
		limit           int
		backups         []fakepanel.Backup
		failNewBackups  bool
		neverComplete   bool
		pollErrors      int
		pollErrorStatus int
		wantEvents      []string
		wantBackups     []string
		wantErr         string
	}{
		{
			name:        "free slot: create first, then delete the oldest",
			limit:       3,
			backups:     []fakepanel.Backup{existing("old-1", 1), existing("old-2", 2)},
			wantEvents:  []string{"create s/new-1", "delete s/old-1"},
			wantBackups: []string{"old-2", "new-1"},
		},
		{
			name:  "full server: free a slot once, then keep it free",
			limit: 3,
			// the panel does not sort backups, so neither does the fake
			backups:     []fakepanel.Backup{existing("old-3", 3), existing("old-1", 1), existing("old-2", 2)},
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
			wantErr:     "free backup slot",
		},
		{
			name:        "locked backups filling the limit: nothing is deleted",
			limit:       2,
			backups:     []fakepanel.Backup{locked(existing("old-1", 1)), locked(existing("old-2", 2)), existing("old-3", 3)},
			wantBackups: []string{"old-1", "old-2", "old-3"},
			wantErr:     "free backup slot",
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
			wantErr:        "failed on the panel and was deleted",
		},
		{
			name:          "backup that never completes times out and keeps old ones",
			limit:         3,
			backups:       []fakepanel.Backup{existing("old-1", 1)},
			neverComplete: true,
			wantEvents:    []string{"create s/new-1"},
			wantBackups:   []string{"old-1", "new-1"},
			wantErr:       "not completed within",
		},
		{
			name:            "temporary poll errors are retried",
			limit:           3,
			backups:         []fakepanel.Backup{existing("old-1", 1), existing("old-2", 2)},
			pollErrors:      2,
			pollErrorStatus: http.StatusBadGateway,
			wantEvents:      []string{"create s/new-1", "delete s/old-1"},
			wantBackups:     []string{"old-2", "new-1"},
		},
		{
			name:            "backup that disappears stops waiting and keeps old ones",
			limit:           3,
			backups:         []fakepanel.Backup{existing("old-1", 1)},
			pollErrors:      1,
			pollErrorStatus: http.StatusNotFound,
			wantEvents:      []string{"create s/new-1"},
			wantBackups:     []string{"old-1", "new-1"},
			wantErr:         "no longer exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panel := &fakepanel.Panel{
				Limits:          map[string]int{"s": tt.limit},
				FailNewBackups:  tt.failNewBackups,
				NeverComplete:   tt.neverComplete,
				PollErrors:      tt.pollErrors,
				PollErrorStatus: tt.pollErrorStatus,
			}
			for _, backup := range tt.backups {
				panel.AddBackup("s", backup)
			}
			clientAPI := &clientapi.Client{URL: panel.Start(t)}

			err := backupServer(slog.Default(), clientAPI, newServer("s", tt.limit))

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
		wantSucceeded  int32
		wantFailed     int32
	}{
		{name: "all succeed", wantSucceeded: 4},
		// more failures than workers used to leave no worker to receive the next server
		{name: "all fail", failNewBackups: true, wantFailed: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panel := &fakepanel.Panel{
				Limits:         map[string]int{"a": 2, "b": 2, "c": 2, "d": 2, "none": 0, "suspended": 2, "installing": 2, "recent": 2},
				FailNewBackups: tt.failNewBackups,
			}
			panel.AddBackup("recent", fakepanel.Backup{UUID: "recent-1", CreatedAt: time.Now().Add(-time.Hour), Successful: true, Completed: true})
			clientAPI := &clientapi.Client{URL: panel.Start(t)}

			suspended := newServer("suspended", 2)
			suspended.Attributes.Suspended = true
			installing := newServer("installing", 2)
			installing.Attributes.Status = "installing"

			servers := []structs.Server{
				newServer("a", 2), newServer("none", 0), newServer("b", 2), suspended,
				newServer("c", 2), installing, newServer("d", 2), newServer("recent", 2),
			}

			done := make(chan *batchResult)
			go func() { done <- createBackupBatch(clientAPI, servers, 2, 12*time.Hour) }()

			select {
			case result := <-done:
				if got := result.succeeded.Load(); got != tt.wantSucceeded {
					t.Errorf("succeeded = %d, want %d", got, tt.wantSucceeded)
				}
				if got := result.failed.Load(); got != tt.wantFailed {
					t.Errorf("failed = %d, want %d", got, tt.wantFailed)
				}
				if got := result.skipped.Load(); got != 4 {
					t.Errorf("skipped = %d, want 4", got)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("createBackupBatch did not return")
			}

			for _, event := range panel.Events() {
				for _, skipped := range []string{"none", "suspended", "installing", "recent"} {
					if strings.Contains(event, " "+skipped+"/") {
						t.Errorf("skipped server was touched: %v", event)
					}
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

func TestBackupServerLogs(t *testing.T) {
	panel := &fakepanel.Panel{Limits: map[string]int{"s": 2}}
	panel.AddBackup("s", existing("old-1", 1))
	clientAPI := &clientapi.Client{URL: panel.Start(t)}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	err := backupServer(logger.With("server", "s"), clientAPI, newServer("s", 2))
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	for _, want := range []string{
		`level=INFO msg="backup started" server=s backup=new-1`,
		`level=DEBUG msg="backup still running" server=s backup=new-1`,
		`level=INFO msg="backup completed" server=s backup=new-1 bytes=0`,
		`level=INFO msg="deleted old backup" server=s backup=old-1 reason=rotation`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("log output is missing %q:\n%s", want, output)
		}
	}
}

func TestNewestBackupAge(t *testing.T) {
	now := time.Now()
	backup := func(uuid string, age time.Duration, successful, completed, isLocked bool) fakepanel.Backup {
		return fakepanel.Backup{UUID: uuid, CreatedAt: now.Add(-age), Successful: successful, Completed: completed, Locked: isLocked}
	}

	tests := []struct {
		name      string
		backups   []fakepanel.Backup
		wantAge   time.Duration
		wantFound bool
	}{
		{name: "no backups"},
		{
			name:      "newest of several, in any order",
			backups:   []fakepanel.Backup{backup("old", 30*time.Hour, true, true, false), backup("new", 2*time.Hour, true, true, false), backup("mid", 10*time.Hour, true, true, false)},
			wantAge:   2 * time.Hour,
			wantFound: true,
		},
		{
			name:      "locked backups count",
			backups:   []fakepanel.Backup{backup("old", 30*time.Hour, true, true, false), backup("locked", time.Hour, true, true, true)},
			wantAge:   time.Hour,
			wantFound: true,
		},
		{
			name:      "failed and in progress backups do not count",
			backups:   []fakepanel.Backup{backup("old", 30*time.Hour, true, true, false), backup("failed", time.Hour, false, true, false), backup("running", 0, false, false, false)},
			wantAge:   30 * time.Hour,
			wantFound: true,
		},
		{
			name:    "only failed backups",
			backups: []fakepanel.Backup{backup("failed", time.Hour, false, true, false)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panel := &fakepanel.Panel{}
			for _, b := range tt.backups {
				panel.AddBackup("s", b)
			}
			clientAPI := &clientapi.Client{URL: panel.Start(t)}

			age, found, err := newestBackupAge(clientAPI, "s")
			if err != nil {
				t.Fatal(err)
			}
			if found != tt.wantFound {
				t.Fatalf("found = %v, want %v", found, tt.wantFound)
			}
			// age is measured against the real clock, so allow for the time the test takes
			if age < tt.wantAge || age > tt.wantAge+time.Minute {
				t.Errorf("age = %v, want about %v", age, tt.wantAge)
			}
		})
	}
}
