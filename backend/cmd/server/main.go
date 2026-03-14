package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"smart-door-backend/internal/api"
	"smart-door-backend/internal/config"
	"smart-door-backend/internal/database"
	"smart-door-backend/internal/mqtt"
	"smart-door-backend/internal/services"
	"syscall"
	"time"
)

func main() {

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database
	db, err := database.NewPostgres(cfg)
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	// Initialize MQTT client
	mqttClient := mqtt.NewClient(cfg)

	// Initialize services
	authService := services.NewAuthService(db)
	doorService := services.NewDoorService(mqttClient)
	faceService := services.NewFaceService()
	intrusionService := services.NewIntrusionService(db, mqttClient)
	cameraService := services.NewCameraService(faceService, doorService)
	notificationService := services.NewNotificationService()

	// Start MQTT subscribers
	mqtt.StartSubscribers(mqttClient, cameraService, intrusionService)

	// Initialize API router
	router := api.NewRouter(
		authService,
		doorService,
		cameraService,
		notificationService,
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