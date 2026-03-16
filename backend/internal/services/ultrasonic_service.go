package services

import (
	"log"
	"strconv"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"gorm.io/gorm"
)

// UltrasonicService handles HC-SR04 ultrasonic distance sensor events.
// The ESP32 publishes the measured distance (cm) as the MQTT payload.
// MQTT topic: home/door/ultrasonic  payload: e.g. "85.5"
//
// Distance tiers:
//   < 80 cm  → someone is at the door, triggers the full motion pipeline
//   80–200 cm → someone approaching, logs VISITOR_APPROACHING
//   > 200 cm → clear, no action
type UltrasonicService struct {
	db            *gorm.DB
	mqttClient    mqtt.Client
	eventService  *EventService
	cameraService *CameraService
	notify        *NotificationService
	mu            sync.Mutex
	lastFired     time.Time
}

const (
	ultrasonicDebounce = 10 * time.Second
	distanceAtDoorCm   = 80.0
	distanceApproachCm = 200.0
)

func NewUltrasonicService(
	db *gorm.DB,
	mqttClient mqtt.Client,
	eventService *EventService,
	cameraService *CameraService,
	notify *NotificationService,
) *UltrasonicService {
	return &UltrasonicService{
		db:            db,
		mqttClient:    mqttClient,
		eventService:  eventService,
		cameraService: cameraService,
		notify:        notify,
	}
}

// HandleDistance is called every time the ESP32 publishes a distance reading.
// rawPayload is the MQTT message payload (e.g. "85.5").
func (u *UltrasonicService) HandleDistance(rawPayload string) {
	distanceCm, err := strconv.ParseFloat(rawPayload, 64)
	if err != nil {
		log.Printf("[UltrasonicService] Invalid payload %q: %v", rawPayload, err)
		return
	}

	log.Printf("[UltrasonicService] Distance reading: %.1f cm", distanceCm)

	switch {
	case distanceCm < distanceAtDoorCm:
		// Visitor is right at the door → run face recognition pipeline
		u.mu.Lock()
		if time.Since(u.lastFired) < ultrasonicDebounce {
			u.mu.Unlock()
			log.Println("[UltrasonicService] Debounced – ignoring repeated close-range trigger")
			return
		}
		u.lastFired = time.Now()
		u.mu.Unlock()

		log.Println("[UltrasonicService] Visitor at door – triggering motion pipeline")
		go u.cameraService.HandleMotion()

	case distanceCm < distanceApproachCm:
		// Visitor approaching – log and notify
		u.mu.Lock()
		if time.Since(u.lastFired) < ultrasonicDebounce {
			u.mu.Unlock()
			return
		}
		u.lastFired = time.Now()
		u.mu.Unlock()

		log.Println("[UltrasonicService] Visitor approaching")
		u.eventService.LogEvent(models.EventVisitorApproaching, nil, "")
		u.notify.Notify(models.EventVisitorApproaching, "")
	}
}
