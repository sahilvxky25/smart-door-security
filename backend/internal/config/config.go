package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DB_URL               string
	MQTT_BROKER          string
	PORT                 string
	JWT_SECRET           string
	FACE_SERVICE_URL     string
	CLOUDINARY_CLOUD_NAME string
	CLOUDINARY_API_KEY    string
	CLOUDINARY_API_SECRET string
	BACKEND_URL          string
}

func LoadConfig() *Config {
	LoadEnv()

	faceURL := os.Getenv("FACE_SERVICE_URL")
	if faceURL == "" {
		faceURL = "http://localhost:5000"
	}

	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}

	return &Config{
		DB_URL:               os.Getenv("DB_URL"),
		MQTT_BROKER:          os.Getenv("MQTT_BROKER"),
		PORT:                 os.Getenv("PORT"),
		JWT_SECRET:           os.Getenv("JWT_SECRET"),
		FACE_SERVICE_URL:     faceURL,
		CLOUDINARY_CLOUD_NAME: os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CLOUDINARY_API_KEY:    os.Getenv("CLOUDINARY_API_KEY"),
		CLOUDINARY_API_SECRET: os.Getenv("CLOUDINARY_API_SECRET"),
		BACKEND_URL:          backendURL,
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