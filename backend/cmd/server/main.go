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
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter: Bearer {your-jwt-token}
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

	// Initialize Cloudinary storage
	mediaStore, err := storage.NewMediaStorage(
		cfg.CLOUDINARY_CLOUD_NAME,
		cfg.CLOUDINARY_API_KEY,
		cfg.CLOUDINARY_API_SECRET,
	)
	if err != nil {
		log.Fatal("Cloudinary connection failed:", err)
	}

	// Initialize WebRTC signaling hub
	signalingHub := webrtc.NewHub()
	go signalingHub.Run()

	// Start in-process door WebRTC peer (replaces browser-based door.html)
	doorRecv, doorSendFn := signalingHub.RegisterLocalDoor()
	doorPeer := webrtc.NewDoorPeer(doorRecv, doorSendFn, nil)
	go doorPeer.Run()

	// Initialize services (notificationService first — all sensor services depend on it)
	eventService        := services.NewEventService(db)
	eventService.OnEventCreated = func(event *models.Event) {
		signalingHub.BroadcastEventUpdate(event.EventType)
	}
	eventService.GetActiveOwner = signalingHub.GetActiveOwnerID
	authService         := services.NewAuthService(db, cfg.JWT_SECRET)
	doorService         := services.NewDoorService(mqttClient, eventService)
	faceService         := services.NewFaceService(cfg.FACE_SERVICE_URL)
	soundService        := services.NewSoundService()
	notificationService := services.NewNotificationService(signalingHub)
	vibrationService    := services.NewVibrationService(db, eventService, soundService, notificationService, doorService)
	cameraService       := services.NewCameraService(faceService, doorService, eventService, mediaStore, mqttClient, db, notificationService, soundService)
	cameraService.SetCallActiveCheckFn(doorPeer.IsCallActive)
	proximityService    := services.NewProximityService(db, mqttClient, eventService, notificationService)
	ultrasonicService   := services.NewUltrasonicService(db, mqttClient, eventService, cameraService, notificationService)
	cameraService.SetUltrasonicService(ultrasonicService)
	magneticService     := services.NewMagneticService(db, eventService, doorService, soundService, notificationService)
	motorService        := services.NewMotorService(doorService, eventService, soundService, notificationService)

	// Start MQTT subscribers
	mqtt.StartSubscribers(mqttClient, cameraService, vibrationService, proximityService, ultrasonicService, magneticService, motorService)

	// Initialize API router
	router := api.NewRouter(
		db,
		authService,
		doorService,
		cameraService,
		vibrationService,
		notificationService,
		faceService,
		eventService,
		signalingHub,
		mediaStore,
		cfg.JWT_SECRET,
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