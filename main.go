package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/marte26/pterodactylBackup/pterodactylapi/adminapi"
	"github.com/marte26/pterodactylBackup/pterodactylapi/clientapi"
	"github.com/marte26/pterodactylBackup/pterodactylapi/structs"

	"github.com/spf13/viper"
)

var commitHash = "unknown"

// variables so tests can shorten them
var (
	backupTimeout = 2 * time.Hour
	pollInterval  = 5 * time.Second
)

type Config struct {
	APIKey  string `mapstructure:"API_KEY"`
	BaseURL string `mapstructure:"BASE_URL"`
}

func loadEnv(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()

	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)

	return
}

func main() {
	log.Printf("starting pterodactylBackup commit hash: %v", commitHash)

	exPath, err := os.Executable()
	if err != nil {
		log.Fatalf("cannot get executable path: %v\n", err)
	}

	config, err := loadEnv(filepath.Dir(exPath))
	if err != nil {
		log.Fatalf("cannot load config: %v\n", err)
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
		log.Fatalf("cannot get servers: %v\n", err)
	}

	failed := createBackupBatch(&clientAPI, servers, 2)
	if failed > 0 {
		log.Fatalf("%v backups failed\n", failed)
	}
}

func backupWorker(id int, clientAPI *clientapi.Client, serverChan <-chan structs.Server, failed *atomic.Int32) {
	log.Printf("worker %v: started\n", id)
	for server := range serverChan {
		err := backupServer(id, clientAPI, server)
		if err != nil {
			log.Printf("worker %v: backup for server %v failed: %v\n", id, server.Attributes.Name, err)
			failed.Add(1)
		}
	}
	log.Printf("worker %v: stopped\n", id)
}

func backupServer(workerID int, clientAPI *clientapi.Client, server structs.Server) error {
	identifier := server.Attributes.Identifier
	limit := server.Attributes.FeatureLimits.Backups

	// the panel refuses new backups once a server is at its limit, so keep one slot free
	err := clientAPI.PruneBackups(identifier, limit-1, "")
	if err != nil {
		return fmt.Errorf("cannot free a backup slot: %w", err)
	}

	backup, err := clientAPI.CreateBackup(identifier)
	if err != nil {
		return fmt.Errorf("cannot create backup: %w", err)
	}
	log.Printf("worker %v: backup %v for server %v started\n", workerID, backup.Attributes.UUID, server.Attributes.Name)

	backupDetails, err := waitForBackup(clientAPI, identifier, backup.Attributes.UUID)
	if err != nil {
		return err
	}

	if !backupDetails.Attributes.IsSuccessful {
		err := clientAPI.DeleteBackup(identifier, backup.Attributes.UUID)
		if err != nil {
			return fmt.Errorf("backup %v failed and cannot be deleted: %w", backup.Attributes.UUID, err)
		}

		return fmt.Errorf("backup %v failed and was deleted", backup.Attributes.UUID)
	}
	log.Printf("worker %v: backup %v for server %v completed\n", workerID, backup.Attributes.UUID, server.Attributes.Name)

	// with a limit of 1 there is no slot to keep free, so keep the new backup. If locked
	// or in progress backups take the free slot, the next run frees one before creating.
	err = clientAPI.PruneBackups(identifier, max(limit-1, 1), backup.Attributes.UUID)
	if err != nil && !errors.Is(err, clientapi.ErrNoRoom) {
		return fmt.Errorf("cannot delete old backups: %w", err)
	}

	return nil
}

func waitForBackup(clientAPI *clientapi.Client, serverIdentifier string, backupUUID string) (structs.Backup, error) {
	timeout := time.After(backupTimeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastErr error

	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return structs.Backup{}, fmt.Errorf("backup %v did not complete within %v, last error: %w", backupUUID, backupTimeout, lastErr)
			}

			return structs.Backup{}, fmt.Errorf("backup %v did not complete within %v", backupUUID, backupTimeout)
		case <-ticker.C:
			backupDetails, err := clientAPI.GetBackup(serverIdentifier, backupUUID)

			// the backup is gone, so there is nothing left to wait for
			var httpErr *clientapi.HTTPError
			if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
				return backupDetails, fmt.Errorf("backup %v no longer exists: %w", backupUUID, err)
			}

			// keep polling through temporary errors until the timeout
			if err != nil {
				log.Printf("cannot get backup %v, retrying: %v\n", backupUUID, err)
				lastErr = err
				continue
			}

			if !backupDetails.Attributes.CompletedAt.IsZero() {
				return backupDetails, nil
			}
		}
	}
}

// createBackupBatch backs up servers with batchSize workers and returns the number of failed backups
func createBackupBatch(clientAPI *clientapi.Client, servers []structs.Server, batchSize int) int {
	serverChan := make(chan structs.Server)
	var wg sync.WaitGroup
	var failed atomic.Int32

	for i := 0; i < batchSize; i++ {
		wg.Add(1)

		go func(seq int) {
			defer wg.Done()
			backupWorker(seq, clientAPI, serverChan, &failed)
		}(i)
	}

	log.Println("sending servers to workers")
	for _, server := range servers {
		if reason := skipReason(server); reason != "" {
			log.Printf("skipping server %v: %v", server.Attributes.Name, reason)
			continue
		}

		serverChan <- server
	}
	close(serverChan)

	log.Println("finished sending servers to workers, waiting for workers to finish")
	wg.Wait()
	log.Println("finished waiting for workers")

	return int(failed.Load())
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
