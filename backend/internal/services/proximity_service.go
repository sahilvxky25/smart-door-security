package services

import (
	"log"
	// "time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gorm.io/gorm"
)

// ProximityService handles IR proximity sensor events.
// The proximity sensor detects close presence (~30 cm) at the door.
// MQTT topic: home/door/proximity  payload: "DETECTED"
type ProximityService struct {
	db           *gorm.DB
	mqttClient   mqtt.Client
	eventService *EventService
	notify       *NotificationService
}

func NewProximityService(db *gorm.DB, mqttClient mqtt.Client, eventService *EventService, notify *NotificationService) *ProximityService {
	return &ProximityService{
		db:           db,
		mqttClient:   mqttClient,
		eventService: eventService,
		notify:       notify,
	}
}

// HandleProximityDetected is called when the proximity sensor fires.
// It logs a VISITOR_APPROACHING event and notifies the ESP32 to activate
// the door-area indicator LED via home/door/proximity_alert.
func (p *ProximityService) HandleProximityDetected() {
	log.Println("[ProximityService] Visitor detected close to door")

	// Notify ESP32 so it can activate a status indicator (e.g. LED)
	// We keep this for local feedback, but remove the backend event/notification.
	p.mqttClient.Publish("home/door/proximity_alert", 0, false, "VISITOR_NEAR")
}
