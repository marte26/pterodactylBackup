package clientapi

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/marte26/pterodactylBackup/internal/fakepanel"
)

func backup(uuid string, day int) fakepanel.Backup {
	return fakepanel.Backup{
		UUID:       uuid,
		CreatedAt:  time.Date(2025, 1, day, 0, 0, 0, 0, time.UTC),
		Successful: true,
		Completed:  true,
	}
}

func TestGetBackupsPaginates(t *testing.T) {
	panel := &fakepanel.Panel{PageSize: 2}
	for day, uuid := range []string{"b1", "b2", "b3", "b4", "b5"} {
		panel.AddBackup("s", backup(uuid, day+1))
	}
	client := Client{URL: panel.Start(t)}

	backups, err := client.GetBackups("s")
	if err != nil {
		t.Fatal(err)
	}

	var uuids []string
	for _, b := range backups {
		uuids = append(uuids, b.Attributes.UUID)
	}
	if want := []string{"b1", "b2", "b3", "b4", "b5"}; !slices.Equal(uuids, want) {
		t.Errorf("backups = %v, want %v", uuids, want)
	}
}

func TestPruneBackups(t *testing.T) {
	lockedBackup := backup("locked", 1)
	lockedBackup.Locked = true

	tests := []struct {
		name        string
		backups     []fakepanel.Backup
		keep        int
		exclude     string
		wantBackups []string
	}{
		{
			name:        "under the limit deletes nothing",
			backups:     []fakepanel.Backup{backup("b1", 1), backup("b2", 2)},
			keep:        2,
			wantBackups: []string{"b1", "b2"},
		},
		{
			name:        "deletes oldest first",
			backups:     []fakepanel.Backup{backup("b3", 3), backup("b1", 1), backup("b2", 2)},
			keep:        1,
			wantBackups: []string{"b3"},
		},
		{
			name:        "keep 0 deletes every unlocked backup",
			backups:     []fakepanel.Backup{lockedBackup, backup("b2", 2), backup("b3", 3)},
			keep:        0,
			wantBackups: []string{"locked"},
		},
		{
			name:        "excluded backup is kept even when oldest",
			backups:     []fakepanel.Backup{backup("b1", 1), backup("b2", 2), backup("b3", 3)},
			keep:        2,
			exclude:     "b1",
			wantBackups: []string{"b1", "b3"},
		},
		{
			name:        "stops when only locked and excluded backups remain",
			backups:     []fakepanel.Backup{lockedBackup, backup("b2", 2)},
			keep:        0,
			exclude:     "b2",
			wantBackups: []string{"locked", "b2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panel := &fakepanel.Panel{}
			for _, b := range tt.backups {
				panel.AddBackup("s", b)
			}
			client := Client{URL: panel.Start(t)}

			err := client.PruneBackups("s", tt.keep, tt.exclude)
			if err != nil {
				t.Fatal(err)
			}
			if backups := panel.Backups("s"); !slices.Equal(backups, tt.wantBackups) {
				t.Errorf("backups = %v, want %v", backups, tt.wantBackups)
			}
		})
	}
}

func TestCreateBackupHTTPError(t *testing.T) {
	panel := &fakepanel.Panel{Limits: map[string]int{"s": 1}}
	panel.AddBackup("s", backup("b1", 1))
	client := Client{URL: panel.Start(t)}

	_, err := client.CreateBackup("s")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "reached its limit") {
		t.Errorf("error = %v, want status 400 and the panel's message", err)
	}
}

func TestGetBackupsHTTPError(t *testing.T) {
	panel := &fakepanel.Panel{APIKey: "right"}
	client := Client{URL: panel.Start(t), APIKey: "wrong"}

	_, err := client.GetBackups("s")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %v, want it to mention status 401", err)
	}
}
