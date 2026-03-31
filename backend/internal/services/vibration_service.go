package services

import (
	"log"
	"sync"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"gorm.io/gorm"
)

type VibrationService struct {
	db           *gorm.DB
	eventService *EventService
	soundService *SoundService
	notify       *NotificationService
	doorService  *DoorService
	mu           sync.Mutex
	lastFired    time.Time
}

const vibrationDebounce = 5 * time.Second

func NewVibrationService(db *gorm.DB, eventService *EventService, soundService *SoundService, notify *NotificationService, doorService *DoorService) *VibrationService {
	return &VibrationService{
		db:           db,
		eventService: eventService,
		soundService: soundService,
		notify:       notify,
		doorService:  doorService,
	}
}

func (v *VibrationService) HandleVibration() {
	// 1. Check if we are within the 15s auth window
	if v.doorService.IsAuthWindowActive(15 * time.Second) {
		log.Println("[VibrationService] Vibration detected but suppressed (recently authenticated window active)")
		return
	}

	// 2. Debounce to prevent multiple SOS events for the same interaction
	v.mu.Lock()
	if time.Since(v.lastFired) < vibrationDebounce {
		v.mu.Unlock()
		return
	}
	v.lastFired = time.Now()
	v.mu.Unlock()

	log.Println("[VibrationService] Vibration detected → triggering alert")

	v.soundService.PlaySOS()
	v.eventService.LogEvent(models.EventForcedEntry, "")
	v.notify.Notify(models.EventForcedEntry, "")

	log.Printf("[VibrationService] SOS alert played and event logged at %s", time.Now().Format(time.RFC3339))
}
