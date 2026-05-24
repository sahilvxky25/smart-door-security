package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DB_URL                     string
	MQTT_BROKER                string
	PORT                       string
	JWT_SECRET                 string
	FACE_SERVICE_URL           string
	CLOUDINARY_CLOUD_NAME      string
	CLOUDINARY_API_KEY         string
	CLOUDINARY_API_SECRET      string
	BACKEND_URL                string
	FIREBASE_ADMIN_CREDENTIALS string
	DOOR_OWNER_USER_ID         uint
}

func LoadConfig() *Config {
	LoadEnv()

	faceURL := GetEnv("FACE_SERVICE_URL")
	if faceURL == "" {
		faceURL = "http://localhost:5000"
	}

	backendURL := GetEnv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}

	mqttBroker := GetEnv("MQTT_BROKER")
	if mqttBroker == "" {
		mqttBroker = "tcp://localhost:1883"
	}

	var doorOwnerUserID uint
	if raw := GetEnv("DOOR_OWNER_USER_ID"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			log.Printf("Invalid DOOR_OWNER_USER_ID=%q, ignoring", raw)
		} else {
			doorOwnerUserID = uint(parsed)
		}
	}

	return &Config{
		DB_URL:                     GetEnv("DB_URL"),
		MQTT_BROKER:                mqttBroker,
		PORT:                       GetEnv("PORT"),
		JWT_SECRET:                 GetEnv("JWT_SECRET"),
		FACE_SERVICE_URL:           faceURL,
		CLOUDINARY_CLOUD_NAME:      GetEnv("CLOUDINARY_CLOUD_NAME"),
		CLOUDINARY_API_KEY:         GetEnv("CLOUDINARY_API_KEY"),
		CLOUDINARY_API_SECRET:      GetEnv("CLOUDINARY_API_SECRET"),
		BACKEND_URL:                backendURL,
		FIREBASE_ADMIN_CREDENTIALS: GetEnv("FIREBASE_ADMIN_CREDENTIALS"),
		DOOR_OWNER_USER_ID:         doorOwnerUserID,
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
