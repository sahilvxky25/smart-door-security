package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DB_URL string
	MQTT_BROKER string
	PORT string
}

func LoadConfig() *Config {
	LoadEnv()
	return &Config{
		DB_URL: os.Getenv("DB_URL"),
		MQTT_BROKER: os.Getenv("MQTT_BROKER"),
		PORT: os.Getenv("PORT"),
	}
}

func LoadEnv() {

	err := godotenv.Load()
	if err != nil {
		log.Println(".env file not found")
	}
}

func GetEnv(key string) string {
	return os.Getenv(key)
}