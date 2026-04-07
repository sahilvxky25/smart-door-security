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
	leftOpenTimeout  = 18 * time.Second
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
		return
	}

	if time.Since(m.lastFired) < magneticDebounce {
		return
	}
	m.lastFired = time.Now()
	m.isOpen = true

	authorizedOpen := m.doorService.IsAuthWindowActive(autoLockDelay)
	if !authorizedOpen {
		log.Println("[MagneticService] FORCED ENTRY DETECTED: door opened without recent authentication")
		m.soundService.PlaySOS()
		m.eventService.LogEvent(models.EventForcedEntry, "")
		if m.notify.HasActiveCallForEventType(models.EventForcedEntry) {
			log.Println("[MagneticService] Forced-entry call already live - suppressing duplicate incoming call")
		} else {
			m.notify.TriggerIncomingCall(models.EventForcedEntry, "")
		}
		return
	}

	log.Println("[MagneticService] Door opened (authorized via recent auth window)")
	m.eventService.LogEvent(models.EventDoorOpened, "")

	m.leftOpenTimer = time.AfterFunc(leftOpenTimeout, func() {
		m.handleAuthorizedDoorOpenTimeout()
	})
}

func (m *MagneticService) handleAuthorizedDoorOpenTimeout() {
	m.mu.Lock()
	if !m.isOpen {
		m.mu.Unlock()
		return
	}
	m.leftOpenTimer = nil
	m.mu.Unlock()

	log.Println("[MagneticService] Door left open - playing SOS alert")
	m.soundService.PlaySOS()
	m.eventService.LogEvent(models.EventDoorLeftOpen, "")
	m.notify.Notify(models.EventDoorLeftOpen, "")
}

// HandleDoorClose is called when the magnetic sensor detects the door closing.
func (m *MagneticService) HandleDoorClose() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isOpen {
		return
	}

	if time.Since(m.lastFired) < magneticDebounce {
		return
	}
	m.lastFired = time.Now()
	m.isOpen = false

	if m.leftOpenTimer != nil {
		m.leftOpenTimer.Stop()
		m.leftOpenTimer = nil
	}

	log.Println("[MagneticService] Door closed")
	m.eventService.LogEvent(models.EventDoorClosed, "")
}
