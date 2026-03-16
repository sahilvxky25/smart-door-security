package services

import (
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
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
	mu           sync.Mutex
	lastFired    time.Time
}

// debounce window – ignore repeated triggers within this duration
const proximityDebounce = 10 * time.Second

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
	p.mu.Lock()
	if time.Since(p.lastFired) < proximityDebounce {
		p.mu.Unlock()
		log.Println("[ProximityService] Debounced – ignoring repeated trigger")
		return
	}
	p.lastFired = time.Now()
	p.mu.Unlock()

	log.Println("[ProximityService] Visitor detected close to door")

	// Notify ESP32 so it can activate a status indicator (e.g. LED)
	p.mqttClient.Publish("home/door/proximity_alert", 0, false, "VISITOR_NEAR")

	// Log the event and push notification to app
	p.eventService.LogEvent(models.EventVisitorApproaching, nil, "")
	p.notify.Notify(models.EventVisitorApproaching, "")

	log.Printf("[ProximityService] Event logged at %s", time.Now().Format(time.RFC3339))
}
