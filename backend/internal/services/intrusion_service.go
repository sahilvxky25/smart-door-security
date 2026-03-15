package services

import (
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"gorm.io/gorm"
)

type IntrusionService struct {
	db           *gorm.DB
	mqttClient   mqtt.Client
	eventService *EventService
}

func NewIntrusionService(db *gorm.DB, mqttClient mqtt.Client, eventService *EventService) *IntrusionService {
	return &IntrusionService{
		db:           db,
		mqttClient:   mqttClient,
		eventService: eventService,
	}
}

func (i *IntrusionService) HandleIntrusion() {
	log.Println("[IntrusionService] Vibration detected → triggering alert")

	// Publish alert to MQTT so the ESP/buzzer can react
	i.mqttClient.Publish("home/door/alert", 0, false, "INTRUSION_DETECTED")

	// Log event
	i.eventService.LogEvent(models.EventForcedEntry, nil, "")

	log.Printf("[IntrusionService] Alert published and event logged at %s", time.Now().Format(time.RFC3339))
}
