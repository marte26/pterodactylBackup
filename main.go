package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/marte26/pterodactylBackup/pterodactylapi/adminapi"
	"github.com/marte26/pterodactylBackup/pterodactylapi/clientapi"
	"github.com/marte26/pterodactylBackup/pterodactylapi/structs"

	"github.com/spf13/viper"
)

var commitHash = "unknown"

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

	createBackupBatch(&clientAPI, servers, 2)
}

func backupWorker(id int, clientAPI *clientapi.Client, serverChan <-chan structs.Server) {
	log.Printf("worker %v: started\n", id)
	for server := range serverChan {
		backup, err := clientAPI.CreateBackup(server.Attributes.Identifier, true)
		if err != nil {
			log.Printf("worker %v: cannot create backup for server %v: %v\n", id, server.Attributes.Name, err)
		}
		log.Printf("worker %v: backup %v for server %v started\n", id, backup.Attributes.UUID, server.Attributes.Name)

		for range time.Tick(time.Second * 5) {
			backupDetails, err := clientAPI.GetBackup(server.Attributes.Identifier, backup.Attributes.UUID)
			if err != nil {
				log.Printf("worker %v: cannot get backup %v for server %v\n", id, backup.Attributes.UUID, server.Attributes.Name)
				return
			}

			if backupDetails.Attributes.Checksum != "" {
				log.Printf("worker %v: backup %v for server %v completed\n", id, backup.Attributes.UUID, server.Attributes.Name)
				break
			}
		}
	}
	log.Printf("worker %v: stopped\n", id)
}

func createBackupBatch(clientAPI *clientapi.Client, servers []structs.Server, batchSize int) {
	serverChan := make(chan structs.Server)
	var wg sync.WaitGroup

	for i := 0; i < batchSize; i++ {
		wg.Add(1)

		go func(seq int) {
			defer wg.Done()
			backupWorker(seq, clientAPI, serverChan)
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
}

// debug function
func printJSON(s any) {
	jsonIndent, _ := json.MarshalIndent(s, "", "    ")

	fmt.Println(string(jsonIndent))
}
