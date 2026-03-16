package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/api"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/config"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/database"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/mqtt"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/storage"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/webrtc"
	// "github.com/gottatouchsomegrass/smart-door-backend/internal/controllers"
)

// @title Smart Door Security API
// @version 1.0
// @description Backend API for the Smart Door Security system. Manages door access, face recognition, intrusion detection, and MQTT-based IoT communication.
// @host localhost:8080
// @BasePath /
func main() {

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database
	db, err := database.NewPostgres(cfg.DB_URL)
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}
	db.AutoMigrate(&models.User{}, &models.Event{}, &models.FamilyMember{})

	// Initialize MQTT client
	mqttClient := mqtt.NewClient(cfg.MQTT_BROKER)

	// Initialize MinIO storage
	mediaStore, err := storage.NewMediaStorage(
		cfg.MINIO_ENDPOINT,
		cfg.MINIO_ACCESS_KEY,
		cfg.MINIO_SECRET_KEY,
		cfg.MINIO_BUCKET,
		cfg.BACKEND_URL,
	)
	if err != nil {
		log.Fatal("MinIO connection failed:", err)
	}

	// Initialize WebRTC signaling hub
	signalingHub := webrtc.NewHub()
	go signalingHub.Run()

	// Initialize services (notificationService first — all sensor services depend on it)
	eventService        := services.NewEventService(db)
	authService         := services.NewAuthService(db, cfg.JWT_SECRET)
	doorService         := services.NewDoorService(mqttClient)
	faceService         := services.NewFaceService(cfg.FACE_SERVICE_URL)
	soundService        := services.NewSoundService()
	notificationService := services.NewNotificationService(signalingHub)
	intrusionService    := services.NewIntrusionService(db, eventService, soundService, notificationService)
	cameraService       := services.NewCameraService(faceService, doorService, eventService, mediaStore, mqttClient, db, notificationService, soundService)
	proximityService    := services.NewProximityService(db, mqttClient, eventService, notificationService)
	ultrasonicService   := services.NewUltrasonicService(db, mqttClient, eventService, cameraService, notificationService)
	hallService         := services.NewHallService(db, eventService, soundService, notificationService)
	doorStateService    := services.NewDoorStateService(db, eventService, soundService, notificationService)

	// Start MQTT subscribers
	mqtt.StartSubscribers(mqttClient, cameraService, intrusionService, proximityService, ultrasonicService, hallService, doorStateService)

	// Initialize API router
	router := api.NewRouter(
		db,
		authService,
		doorService,
		cameraService,
		intrusionService,
		notificationService,
		faceService,
		eventService,
		signalingHub,
		mediaStore,
	)

	// Start HTTP server
	server := api.StartServer(router, cfg)

	// Graceful shutdown handling
	waitForShutdown(server)
}

func waitForShutdown(server *api.Server) {

	stop := make(chan os.Signal, 1)

	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Println("Server shutdown failed:", err)
	}

	log.Println("Server stopped")
}