package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/marte26/pterodactylBackup/pterodactylapi/adminapi"
	"github.com/marte26/pterodactylBackup/pterodactylapi/clientapi"
	"github.com/marte26/pterodactylBackup/pterodactylapi/structs"

	"github.com/spf13/viper"
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
	exPath, err := os.Executable()
	if err != nil {
		log.Fatalf("cannot get executable path: %v", err)
	}

	config, err := loadEnv(filepath.Dir(exPath))
	if err != nil {
		log.Fatal("cannot load config: ", err)
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
		log.Fatalf("cannot get servers: %v", err)
	}

	for _, server := range servers {
		createBackup(clientAPI, server)
	}
}

func createBackup(clientAPI clientapi.Client, server structs.Server) {
	if server.Attributes.FeatureLimits.Backups == 0 {
		log.Printf("backups for server %v not allowed, skipping", server.Attributes.Name)
		return
	}

	backups, err := clientAPI.GetBackups(server.Attributes.Identifier)
	if err != nil {
		log.Printf("cannot get backups for server %v: %v", server.Attributes.Name, err)
		return
	}

	if len(backups) >= server.Attributes.FeatureLimits.Backups {
		log.Printf("backup limit of %v reached, deleting oldest backup", server.Attributes.FeatureLimits.Backups)
		err := clientAPI.DeleteBackup(server.Attributes.Identifier, backups[0].Attributes.UUID)
		if err != nil {
			log.Printf("cannot delete backup %v for server %v: %v", backups[0].Attributes.UUID, server.Attributes.Name, err)
			return
		}
	}

	response, err := clientAPI.CreateBackup(server.Attributes.Identifier)
	if err != nil {
		log.Printf("cannot create backup for server %v: %v", server.Attributes.Name, err)
		return
	}

	log.Printf("creating backup %v for server %v", response.Attributes.UUID, server.Attributes.Name)
}

// debug function
func printJSON(s any) {
	jsonIndent, _ := json.MarshalIndent(s, "", "    ")

	fmt.Println(string(jsonIndent))
}
