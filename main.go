package main

import (
	"fmt"
	"github.com/marte26/pterodactyl-backup/pterodactyl_api/pterodactyl_api_admin"
	"github.com/spf13/viper"
	"log"
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

	adminApi := pterodactyl_api_admin.Client{
		URL:    config.BaseUrl + "/api/application",
		ApiKey: config.ApiKey,
	}

	servers, err := adminApi.GetServers()
	if err != nil {
		log.Fatal("cannot get servers:", err)
	}

	fmt.Println(servers)
}
