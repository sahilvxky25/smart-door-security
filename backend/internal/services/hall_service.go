package services

import (
	"log"
	"sync"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"gorm.io/gorm"
)

// HallService handles Hall effect sensor events.
// The sensor detects when the door handle/latch mechanism moves (a magnet
// is attached to the handle shaft). An activation while the door is locked
// indicates an unauthorized handle-turn attempt.
// MQTT topic: home/door/hall  payload: "DETECTED"
type HallService struct {
	db           *gorm.DB
	eventService *EventService
	soundService *SoundService
	notify       *NotificationService
	mu           sync.Mutex
	lastFired    time.Time
}

const hallDebounce = 5 * time.Second

func NewHallService(db *gorm.DB, eventService *EventService, soundService *SoundService, notify *NotificationService) *HallService {
	return &HallService{
		db:           db,
		eventService: eventService,
		soundService: soundService,
		notify:       notify,
	}
}

// HandleHallDetected is called when the Hall effect sensor detects a magnetic
// field change (handle rotation). It logs a HANDLE_TAMPER event and plays an
// SOS alert on the laptop speaker.
func (h *HallService) HandleHallDetected() {
	h.mu.Lock()
	if time.Since(h.lastFired) < hallDebounce {
		h.mu.Unlock()
		log.Println("[HallService] Debounced – ignoring repeated trigger")
		return
	}
	h.lastFired = time.Now()
	h.mu.Unlock()

	log.Println("[HallService] Handle tamper detected via Hall effect sensor")

	h.soundService.PlaySOS()
	h.eventService.LogEvent(models.EventHandleTamper, nil, "")
	h.notify.Notify(models.EventHandleTamper, "")

	log.Printf("[HallService] SOS alert played and event logged at %s", time.Now().Format(time.RFC3339))
}
