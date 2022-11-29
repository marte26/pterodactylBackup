package main

import (
	"fmt"
	"log"

	"github.com/marte26/pterodactylBackup/pterodactylApi/pterodactylAdminApi"

	"github.com/spf13/viper"
)

type Config struct {
	ApiKey  string `mapstructure:"API_KEY"`
	BaseUrl string `mapstructure:"BASE_URL"`
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

	adminApi := pterodactylAdminApi.Client{
		URL:    config.BaseUrl,
		ApiKey: config.ApiKey,
	}

	servers, err := adminApi.GetServers()
	if err != nil {
		log.Fatal("cannot get servers:", err)
	}

	fmt.Println(servers)
}
