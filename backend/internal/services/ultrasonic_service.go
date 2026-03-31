package services

import (
	"log"
	"strconv"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
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
	mu            sync.RWMutex
	lastDistance  float64
}

const (
	distanceAtDoorCm     = 20.0
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

	u.mu.Lock()
	u.lastDistance = distanceCm
	u.mu.Unlock()

	log.Printf("[UltrasonicService] Distance reading: %.1f cm", distanceCm)

	switch {
	case distanceCm < distanceAtDoorCm:
		// Visitor is right at the door – we just log it and store it.
		// The PIR detection will check this distance before triggering the camera.
		log.Println("[UltrasonicService] Visitor at door (distance logic)")
	}
}

// IsAtDoor returns true if the last measured distance is within the threshold.
func (u *UltrasonicService) IsAtDoor() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.lastDistance > 0 && u.lastDistance < distanceAtDoorCm
}
