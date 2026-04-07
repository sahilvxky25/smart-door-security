package services

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/storage"
	"gorm.io/gorm"
)

type CameraService struct {
	faceService   *FaceService
	doorService   *DoorService
	eventService  *EventService
	storage       *storage.MediaStorage
	mqttClient    mqtt.Client
	db            *gorm.DB
	notifySvc     *NotificationService
	soundService  *SoundService
	ultrasonicSvc *UltrasonicService
	isCallActive  func() bool
}

func NewCameraService(
	faceService *FaceService,
	doorService *DoorService,
	eventService *EventService,
	store *storage.MediaStorage,
	mqttClient mqtt.Client,
	db *gorm.DB,
	notifySvc *NotificationService,
	soundService *SoundService,
) *CameraService {
	return &CameraService{
		faceService:  faceService,
		doorService:  doorService,
		eventService: eventService,
		storage:      store,
		mqttClient:   mqttClient,
		db:           db,
		notifySvc:    notifySvc,
		soundService: soundService,
	}
}

func (c *CameraService) SetUltrasonicService(svc *UltrasonicService) {
	c.ultrasonicSvc = svc
}

func (c *CameraService) SetCallActiveCheckFn(fn func() bool) {
	c.isCallActive = fn
}

// HandleMotion is called when the PIR sensor detects motion.
// It logs an "approaching" alert and then runs the full face recognition
// pipeline only if the ultrasonic sensor confirms a visitor at the door.
func (c *CameraService) HandleMotion() {
	if c.isCallActive != nil && c.isCallActive() {
		log.Println("[Pipeline] PIR triggered but video call is active – skipping face detection")
		return
	}

	if c.ultrasonicSvc != nil && !c.ultrasonicSvc.IsAtDoor() {
		log.Println("[Pipeline] PIR triggered but ultrasonic confirms no visitor at < 20cm – skipping camera")
		return
	}

	// 1. Log the approach event (PIR + Ultrasonic confirmed) with global debouncing
	event, _ := c.eventService.LogEventWithDebounce(models.EventVisitorApproaching, "", VisitorAlertDebounce)
	if event != nil {
		log.Println("[Pipeline] PIR + Ultrasonic match → Visitor approaching notification")
		c.notifySvc.Notify(models.EventVisitorApproaching, "")
	}

	log.Println("[Pipeline] PIR + Ultrasonic match → asking face service to capture & recognize")

	result, err := c.faceService.CaptureAndRecognize()
	if err != nil {
		log.Println("[Pipeline] Face service error:", err)
		return
	}

	// Upload captured frame to Cloudinary
	imageURL := ""
	if len(result.FrameJPG) > 0 {
		objectName := fmt.Sprintf("events/%d.jpg", time.Now().UnixMilli())
		url, err := c.storage.UploadImage(context.Background(), objectName, result.FrameJPG, "image/jpeg")
		if err != nil {
			log.Printf("[Pipeline] Failed to upload image to Cloudinary: %v", err)
		} else {
			imageURL = url
		}
	}

	// Decision engine
	switch {
	case result.Spoof:
		log.Println("[Pipeline] SPOOF DETECTED → creating event + triggering call")
		c.eventService.LogEvent(models.EventSpoofAttempt, imageURL)
		if c.notifySvc.HasActiveCallForEventType(models.EventSpoofAttempt) {
			log.Println("[Pipeline] Spoof call already live -> suppressing duplicate incoming call")
		} else {
			c.notifySvc.TriggerIncomingCall(models.EventSpoofAttempt, imageURL)
		}
		c.soundService.PlaySOS()

	case result.Match:
		log.Printf("[Pipeline] Authorized face %q → unlocking door", result.User)
		c.doorService.UnlockDoor()
		c.soundService.PlayWelcome()

		c.eventService.LogEvent(models.EventAuthorizedEntry, imageURL)

	default:
		log.Println("[Pipeline] Unknown visitor → storing image + triggering call")
		// hardware events belong to the public feed, so userID remains nil
		c.eventService.LogEvent(models.EventUnknownVisitor, imageURL)
		c.mqttClient.Publish("home/door/unknown_visitor", 0, false, imageURL)
		if c.notifySvc.HasActiveCallForEventType(models.EventUnknownVisitor) {
			log.Println("[Pipeline] Unknown visitor call already live -> suppressing duplicate incoming call")
		} else {
			c.notifySvc.TriggerIncomingCall(models.EventUnknownVisitor, imageURL)
		}
	}
}
