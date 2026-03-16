package services

import (
	"log"
	"sync"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"gorm.io/gorm"
)

// DoorStateService handles the magnetic door sensor (reed switch).
// It tracks whether the door is physically open or closed and raises an alert
// if the door is left open beyond the configured timeout.
// MQTT topic: home/door/magnetic  payload: "OPEN" | "CLOSED"
type DoorStateService struct {
	db            *gorm.DB
	eventService  *EventService
	soundService  *SoundService
	notify        *NotificationService
	mu            sync.Mutex
	isOpen        bool
	leftOpenTimer *time.Timer
}

// leftOpenTimeout is how long the door may stay open before an alert fires.
const leftOpenTimeout = 30 * time.Second

func NewDoorStateService(db *gorm.DB, eventService *EventService, soundService *SoundService, notify *NotificationService) *DoorStateService {
	return &DoorStateService{
		db:           db,
		eventService: eventService,
		soundService: soundService,
		notify:       notify,
	}
}

// HandleMagneticSensor dispatches to HandleDoorOpen or HandleDoorClose based
// on the MQTT payload published by the ESP32.
func (d *DoorStateService) HandleMagneticSensor(payload string) {
	switch payload {
	case "OPEN":
		d.HandleDoorOpen()
	case "CLOSED":
		d.HandleDoorClose()
	default:
		log.Printf("[DoorStateService] Unknown magnetic sensor payload: %q", payload)
	}
}

// HandleDoorOpen is called when the reed switch detects the door opening.
func (d *DoorStateService) HandleDoorOpen() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.isOpen {
		return // already open, no duplicate event
	}
	d.isOpen = true

	log.Println("[DoorStateService] Door opened")
	d.eventService.LogEvent(models.EventDoorOpened, nil, "")

	// Start the left-open timer
	d.leftOpenTimer = time.AfterFunc(leftOpenTimeout, func() {
		log.Println("[DoorStateService] Door left open – playing SOS alert")
		d.soundService.PlaySOS()
		d.eventService.LogEvent(models.EventDoorLeftOpen, nil, "")
		d.notify.Notify(models.EventDoorLeftOpen, "")
	})
}

// HandleDoorClose is called when the reed switch detects the door closing.
func (d *DoorStateService) HandleDoorClose() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isOpen {
		return // already closed, no duplicate event
	}
	d.isOpen = false

	// Cancel the left-open alert if it hasn't fired yet
	if d.leftOpenTimer != nil {
		d.leftOpenTimer.Stop()
		d.leftOpenTimer = nil
	}

	log.Println("[DoorStateService] Door closed")
	d.eventService.LogEvent(models.EventDoorClosed, nil, "")
}
