package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/marte26/pterodactylBackup/pterodactylapi/clientapi"
	"github.com/marte26/pterodactylBackup/pterodactylapi/structs"
)

// variables so tests can shorten them
var (
	backupTimeout = 2 * time.Hour
	pollInterval  = 5 * time.Second
)

// batchResult counts the outcome of a backup run, workers update it concurrently
type batchResult struct {
	succeeded, failed, skipped atomic.Int32
}

// createBackupBatch backs up servers with batchSize workers and returns how many succeeded, failed or were skipped
func createBackupBatch(clientAPI *clientapi.Client, servers []structs.Server, batchSize int, minInterval time.Duration) *batchResult {
	serverChan := make(chan structs.Server)
	var wg sync.WaitGroup
	var result batchResult

	for i := range batchSize {
		wg.Add(1)

		go func() {
			defer wg.Done()
			backupWorker(i, clientAPI, serverChan, minInterval, &result)
		}()
	}

	for _, server := range servers {
		if reason := skipReason(server); reason != "" {
			slog.Info("skipping server", "server", server.Attributes.Name, "server_id", server.Attributes.Identifier, "reason", reason)
			result.skipped.Add(1)
			continue
		}

		serverChan <- server
	}
	close(serverChan)

	wg.Wait()

	return &result
}

// skipReason returns why server cannot be backed up, or "" if it can
func skipReason(server structs.Server) string {
	switch {
	case server.Attributes.FeatureLimits.Backups <= 0:
		return "backups not allowed"
	case server.Attributes.Suspended:
		return "suspended"
	// the panel refuses backups while a server is installing, restoring a backup and so on
	case server.Attributes.Status != "":
		return "status is " + server.Attributes.Status
	}

	return ""
}

func backupWorker(id int, clientAPI *clientapi.Client, serverChan <-chan structs.Server, minInterval time.Duration, result *batchResult) {
	logger := slog.With("worker", id)
	logger.Debug("worker started")

	for server := range serverChan {
		serverLogger := logger.With("server", server.Attributes.Name, "server_id", server.Attributes.Identifier)

		if minInterval > 0 {
			age, found, err := newestBackupAge(clientAPI, server.Attributes.Identifier)
			if err != nil {
				serverLogger.Error("backup failed", "error", fmt.Errorf("check newest backup: %w", err))
				result.failed.Add(1)
				continue
			}

			if found && age < minInterval {
				serverLogger.Info("skipping server", "reason", "recent backup", "age", age.Round(time.Minute))
				result.skipped.Add(1)
				continue
			}
		}

		err := backupServer(serverLogger, clientAPI, server)
		if err != nil {
			serverLogger.Error("backup failed", "error", err)
			result.failed.Add(1)
			continue
		}

		result.succeeded.Add(1)
	}

	logger.Debug("worker stopped")
}

// newestBackupAge returns how old the newest successful backup of server is, found is false if it has none
func newestBackupAge(clientAPI *clientapi.Client, serverIdentifier string) (age time.Duration, found bool, err error) {
	backups, err := clientAPI.GetBackups(serverIdentifier)
	if err != nil {
		return 0, false, err
	}

	// failed and in progress backups are not usable, locked ones are
	var newest time.Time
	for _, backup := range backups {
		if backup.Attributes.IsSuccessful && backup.Attributes.CreatedAt.After(newest) {
			newest = backup.Attributes.CreatedAt
		}
	}

	if newest.IsZero() {
		return 0, false, nil
	}

	return time.Since(newest), true, nil
}

func backupServer(logger *slog.Logger, clientAPI *clientapi.Client, server structs.Server) error {
	identifier := server.Attributes.Identifier
	limit := server.Attributes.FeatureLimits.Backups

	// the panel refuses new backups once a server is at its limit, so keep one slot free
	deleted, err := clientAPI.PruneBackups(identifier, limit-1, "")
	logDeleted(logger, deleted, "free a slot for the new backup")
	if err != nil {
		return fmt.Errorf("free backup slot: %w", err)
	}

	start := time.Now()

	backup, err := clientAPI.CreateBackup(identifier)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	uuid := backup.Attributes.UUID
	backupLogger := logger.With("backup", uuid)
	backupLogger.Info("backup started")

	backupDetails, err := waitForBackup(backupLogger, clientAPI, identifier, uuid)
	if err != nil {
		return fmt.Errorf("wait for backup %v: %w", uuid, err)
	}

	if !backupDetails.Attributes.IsSuccessful {
		err := clientAPI.DeleteBackup(identifier, uuid)
		if err != nil {
			return fmt.Errorf("delete failed backup %v: %w", uuid, err)
		}
		backupLogger.Info("deleted failed backup")

		return fmt.Errorf("backup %v failed on the panel and was deleted", uuid)
	}
	backupLogger.Info("backup completed", "bytes", backupDetails.Attributes.Bytes, "duration", time.Since(start).Round(time.Second))

	// with a limit of 1 there is no slot to keep free, so keep the new backup
	deleted, err = clientAPI.PruneBackups(identifier, max(limit-1, 1), uuid)
	logDeleted(logger, deleted, "rotation")

	switch {
	case errors.Is(err, clientapi.ErrNoRoom):
		// not a failure, the next run frees a slot before creating
		logger.Warn("locked or in progress backups take the free slot", "error", err)
	case err != nil:
		return fmt.Errorf("delete old backups: %w", err)
	}

	return nil
}

func waitForBackup(logger *slog.Logger, clientAPI *clientapi.Client, serverIdentifier string, backupUUID string) (structs.Backup, error) {
	timeout := time.After(backupTimeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastErr error

	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return structs.Backup{}, fmt.Errorf("not completed within %v, last poll error: %w", backupTimeout, lastErr)
			}

			return structs.Backup{}, fmt.Errorf("not completed within %v", backupTimeout)
		case <-ticker.C:
			backupDetails, err := clientAPI.GetBackup(serverIdentifier, backupUUID)

			// the backup is gone, so there is nothing left to wait for
			var httpErr *clientapi.HTTPError
			if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
				return backupDetails, fmt.Errorf("backup no longer exists: %w", err)
			}

			// keep polling through temporary errors until the timeout
			if err != nil {
				logger.Warn("cannot get backup status, retrying", "error", err)
				lastErr = err
				continue
			}

			if !backupDetails.Attributes.CompletedAt.IsZero() {
				return backupDetails, nil
			}
			logger.Debug("backup still running")
		}
	}
}

func logDeleted(logger *slog.Logger, uuids []string, reason string) {
	for _, uuid := range uuids {
		logger.Info("deleted old backup", "backup", uuid, "reason", reason)
	}
}
