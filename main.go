package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/marte26/pterodactylBackup/pterodactylapi/pterodactyladminapi"
	"github.com/marte26/pterodactylBackup/pterodactylapi/pterodactylclientapi"

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
	config, err := loadEnv(".")
	if err != nil {
		log.Fatal("cannot load config: ", err)
	}

	adminAPI := pterodactyladminapi.Client{
		URL:    config.BaseURL,
		APIKey: config.APIKey,
	}
	clientAPI := pterodactylclientapi.Client{
		URL:    config.BaseURL,
		APIKey: config.APIKey,
	}

	servers, err := adminAPI.GetServers()
	if err != nil {
		log.Fatal("cannot get servers:", err)
	}

	files, err := clientAPI.GetFiles(servers[0].Attributes.Identifier, "/world")

	printJson(files)
}

func printJson(s any) {
	jsonIndent, _ := json.MarshalIndent(s, "", "    ")

	fmt.Println(string(jsonIndent))
}
