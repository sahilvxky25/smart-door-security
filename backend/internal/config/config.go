package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DB_URL           string
	MQTT_BROKER      string
	PORT             string
	FACE_SERVICE_URL string
	MINIO_ENDPOINT   string
	MINIO_ACCESS_KEY string
	MINIO_SECRET_KEY string
	MINIO_BUCKET     string
}

func LoadConfig() *Config {
	LoadEnv()

	faceURL := os.Getenv("FACE_SERVICE_URL")
	if faceURL == "" {
		faceURL = "http://localhost:5000"
	}

	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = "localhost:9000"
	}

	minioBucket := os.Getenv("MINIO_BUCKET")
	if minioBucket == "" {
		minioBucket = "door-images"
	}

	return &Config{
		DB_URL:           os.Getenv("DB_URL"),
		MQTT_BROKER:      os.Getenv("MQTT_BROKER"),
		PORT:             os.Getenv("PORT"),
		FACE_SERVICE_URL: faceURL,
		MINIO_ENDPOINT:   minioEndpoint,
		MINIO_ACCESS_KEY: os.Getenv("MINIO_ACCESS_KEY"),
		MINIO_SECRET_KEY: os.Getenv("MINIO_SECRET_KEY"),
		MINIO_BUCKET:     minioBucket,
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