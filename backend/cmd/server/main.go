package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/api"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/calls"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/config"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/database"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/flows"
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

	// Initialize WebRTC signaling hub (requires CallManager injected)
	callManager := calls.NewCallManager(nil, 30*time.Second) // Hub injected below
	signalingHub := webrtc.NewHub(callManager)
	callManager.SetHub(signalingHub) // Add a SetHub method to break chicken-egg dependency
	go signalingHub.Run()

	// Start in-process door WebRTC peer (replaces browser-based door.html)
	doorRecv, doorSendFn := signalingHub.RegisterLocalDoor()
	doorPeer := webrtc.NewDoorPeer(doorRecv, doorSendFn, nil)
	go doorPeer.Run()

	// (Services initialization continued...)

	eventService := services.NewEventService(db)
	eventService.OnEventCreated = func(event *models.Event) {
		signalingHub.BroadcastEventUpdate(event.EventType)
	}
	eventService.GetActiveOwner = signalingHub.GetActiveOwnerID
	securityState := services.NewSecurityStateService()
	authService := services.NewAuthService(db, cfg.JWT_SECRET)
	doorService := services.NewDoorService(mqttClient, eventService, securityState)
	faceService := services.NewFaceService(cfg.FACE_SERVICE_URL)
	soundService := services.NewSoundService()
	notificationService := services.NewNotificationService(signalingHub, callManager, db)
	proximityService := services.NewProximityService(db, mqttClient, eventService, notificationService)
	ultrasonicService := services.NewUltrasonicService(db, mqttClient, eventService, notificationService)
	motorService := services.NewMotorService(doorService, eventService, soundService, notificationService)
	visitorAuthFlow := flows.NewVisitorAuthFlow(faceService, doorService, securityState, eventService, mediaStore, mqttClient, notificationService, soundService, ultrasonicService, doorPeer.IsCallActive)
	intrusionFlow := flows.NewIntrusionFlow(doorService, securityState, eventService, faceService, mediaStore, soundService, notificationService)
	callManager.SetOnDeclinedCall(func(callID string, callType string) {
		if callType != models.EventForcedEntry {
			return
		}
		log.Printf("[Main] Forced-entry call %s declined by owner -> locking door and clearing intrusion", callID)
		doorService.LockDoor()
		if securityState.ClearIntrusion("forced-entry call declined") {
			eventService.LogEvent(models.EventIntrusionCleared, "")
		}
	})

	// Start MQTT subscribers
	mqtt.StartSubscribers(mqttClient, visitorAuthFlow, intrusionFlow, proximityService, ultrasonicService, motorService)

	// Initialize API router
	router := api.NewRouter(
		db,
		authService,
		doorService,
		securityState,
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
