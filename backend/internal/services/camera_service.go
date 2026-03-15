package services

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/storage"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/webrtc"
	"gorm.io/gorm"
)

type CameraService struct {
	faceService  *FaceService
	doorService  *DoorService
	eventService *EventService
	storage      *storage.MediaStorage
	mqttClient   mqtt.Client
	db           *gorm.DB
	signalingHub *webrtc.Hub
}

func NewCameraService(
	faceService *FaceService,
	doorService *DoorService,
	eventService *EventService,
	store *storage.MediaStorage,
	mqttClient mqtt.Client,
	db *gorm.DB,
	signalingHub *webrtc.Hub,
) *CameraService {
	return &CameraService{
		faceService:  faceService,
		doorService:  doorService,
		eventService: eventService,
		storage:      store,
		mqttClient:   mqttClient,
		db:           db,
		signalingHub: signalingHub,
	}
}

// HandleMotion is called when the PIR sensor detects motion.
// It runs the full decision engine: capture → recognize → act.
func (c *CameraService) HandleMotion() {
	log.Println("[Pipeline] PIR triggered → asking face service to capture & recognize")

	result, err := c.faceService.CaptureAndRecognize()
	if err != nil {
		log.Println("[Pipeline] Face service error:", err)
		return
	}

	// Upload captured frame to MinIO
	imageURL := ""
	if len(result.FrameJPG) > 0 {
		objectName := fmt.Sprintf("events/%d.jpg", time.Now().UnixMilli())
		url, err := c.storage.UploadImage(context.Background(), objectName, result.FrameJPG, "image/jpeg")
		if err != nil {
			log.Printf("[Pipeline] Failed to upload image to MinIO: %v", err)
		} else {
			imageURL = url
		}
	}

	// Decision engine
	switch {
	case result.Spoof:
		log.Println("[Pipeline] SPOOF DETECTED → creating event + alert")
		c.eventService.LogEvent(models.EventSpoofAttempt, nil, imageURL)
		c.mqttClient.Publish("home/door/alert", 0, false, "SPOOF_DETECTED")

	case result.Match:
		log.Printf("[Pipeline] Authorized user %q → unlocking door", result.User)
		c.doorService.UnlockDoor()

		var userID *uint
		var user models.User
		if err := c.db.Where("name = ?", result.User).First(&user).Error; err == nil {
			userID = &user.ID
		} else {
			log.Printf("[Pipeline] Warning: recognized user %q not found in DB: %v", result.User, err)
		}
		c.eventService.LogEvent(models.EventAuthorizedEntry, userID, imageURL)

	default:
		log.Println("[Pipeline] Unknown visitor → storing image + notifying owner")
		c.eventService.LogEvent(models.EventUnknownVisitor, nil, imageURL)
		c.mqttClient.Publish("home/door/unknown_visitor", 0, false, imageURL)
		c.signalingHub.NotifyOwner(imageURL)
	}
}
