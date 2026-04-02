package services

import (
	"log"
	"sync"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"gorm.io/gorm"
)

// MagneticService handles magnetic sensor events.
// The sensor detects when the door is opened (a magnet is attached to the door).
// MQTT topic: home/door/magnetic  payload: "DETECTED"

type MagneticService struct {
	db            *gorm.DB
	eventService  *EventService
	doorService   *DoorService
	soundService  *SoundService
	notify        *NotificationService
	mu            sync.Mutex
	isOpen        bool
	leftOpenTimer *time.Timer
	lastFired     time.Time
}

const (
	magneticDebounce = 2 * time.Second
	leftOpenTimeout  = 13 * time.Second
)

func NewMagneticService(db *gorm.DB, eventService *EventService, doorService *DoorService, soundService *SoundService, notify *NotificationService) *MagneticService {
	return &MagneticService{
		db:           db,
		eventService: eventService,
		doorService:  doorService,
		soundService: soundService,
		notify:       notify,
	}
}

// HandleDoorOpen is called when the magnetic sensor detects the door opening.
func (m *MagneticService) HandleDoorOpen() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isOpen {
		return // already open
	}

	if time.Since(m.lastFired) < magneticDebounce {
		return
	}
	m.lastFired = time.Now()
	m.isOpen = true

	// INTRUSION DETECTION: If the door is physically opened, trigger an alarm
	// UNLESS we are within the 15s auth window.
	if !m.doorService.IsAuthWindowActive(15 * time.Second) {
		log.Println("[MagneticService] ⚠ FORCED ENTRY DETECTED: Door opened without recent authentication!")
		m.soundService.PlaySOS()
		m.eventService.LogEvent(models.EventForcedEntry, "")
		m.notify.TriggerIncomingCall(models.EventForcedEntry, "")
	} else {
		log.Println("[MagneticService] Door opened (authorized via recent auth window)")
		m.eventService.LogEvent(models.EventDoorOpened, "")
	}

	// Always start the left-open timer whenever the door opens
	m.leftOpenTimer = time.AfterFunc(leftOpenTimeout, func() {
		log.Println("[MagneticService] Door left open – playing SOS alert")
		m.soundService.PlaySOS()
		m.eventService.LogEvent(models.EventDoorLeftOpen, "")
		m.notify.Notify(models.EventDoorLeftOpen, "")
	})
}


// HandleDoorClose is called when the magnetic sensor detects the door closing.
func (m *MagneticService) HandleDoorClose() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isOpen {
		return // already closed
	}

	if time.Since(m.lastFired) < magneticDebounce {
		return
	}
	m.lastFired = time.Now()
	m.isOpen = false

	// Cancel the left-open alert if it hasn't fired yet
	if m.leftOpenTimer != nil {
		m.leftOpenTimer.Stop()
		m.leftOpenTimer = nil
	}

	log.Println("[MagneticService] Door closed")
	m.eventService.LogEvent(models.EventDoorClosed, "")
}