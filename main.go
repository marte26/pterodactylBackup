package main

import (
	"fmt"
	"log"
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

const backupTimeout = 2 * time.Hour

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

	// with a limit of 1 there is no slot to keep free, so keep the new backup
	err = clientAPI.PruneBackups(identifier, max(limit-1, 1), backup.Attributes.UUID)
	if err != nil {
		return fmt.Errorf("cannot delete old backups: %w", err)
	}

	return nil
}

func waitForBackup(clientAPI *clientapi.Client, serverIdentifier string, backupUUID string) (structs.Backup, error) {
	timeout := time.After(backupTimeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return structs.Backup{}, fmt.Errorf("backup %v did not complete within %v", backupUUID, backupTimeout)
		case <-ticker.C:
			backupDetails, err := clientAPI.GetBackup(serverIdentifier, backupUUID)
			if err != nil {
				return backupDetails, fmt.Errorf("cannot get backup %v: %w", backupUUID, err)
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
		if server.Attributes.FeatureLimits.Backups > 0 {
			serverChan <- server
		} else {
			log.Printf("backups for server %v not allowed, skipping", server.Attributes.Name)
		}
	}
	close(serverChan)

	log.Println("finished sending servers to workers, waiting for workers to finish")
	wg.Wait()
	log.Println("finished waiting for workers")

	return int(failed.Load())
}
