package services

import (
	"log"
	"sync"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/calls"
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
	lastCallID   string
}

const vibrationDebounce = 10 * time.Second

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
	// 1. Suppress only while auth window is active AND motor has not yet reached lock angle (0).
	if v.doorService.IsAuthWindowActive(autoLockDelay) && !v.doorService.IsMotorAtLockedPosition() {
		log.Println("[VibrationService] Vibration detected but suppressed (auth window active and motor not at 0)")
		return
	}

	v.mu.Lock()
	// 2. Gate by current call state for vibration-triggered calls.
	if v.lastCallID != "" {
		status, ok := v.notify.GetCallStatus(v.lastCallID)
		if ok {
			if status == calls.StatusRinging || status == calls.StatusAccepted {
				v.mu.Unlock()
				log.Printf("[VibrationService] Suppressed: call %s still %s", v.lastCallID, status)
				return
			}
			if status == calls.StatusDeclined {
				declinedID := v.lastCallID
				// Declined calls should re-arm future vibration events.
				v.lastCallID = ""
				log.Printf("[VibrationService] Last call %s was declined; re-arming", declinedID)
			} else {
				// Missed/ended sessions re-arm the flow for future vibration events.
				v.lastCallID = ""
			}
		} else {
			// Non-existent sessions re-arm the flow for future vibration events.
			v.lastCallID = ""
		}
	}

	if v.notify.HasActiveCallForEventType(models.EventForcedEntry) {
		v.mu.Unlock()
		log.Println("[VibrationService] Suppressed: forced-entry call already live")
		return
	}

	// 3. Debounce to prevent multiple SOS events for the same interaction.
	if time.Since(v.lastFired) < vibrationDebounce {
		v.mu.Unlock()
		return
	}
	v.lastFired = time.Now()
	v.mu.Unlock()

	log.Println("[VibrationService] Vibration detected -> triggering alert")

	v.soundService.PlaySOS()
	v.eventService.LogEvent(models.EventForcedEntry, "")
	callID := v.notify.TriggerIncomingCall(models.EventForcedEntry, "")
	v.mu.Lock()
	v.lastCallID = callID
	v.mu.Unlock()

	log.Printf("[VibrationService] SOS alert and video call triggered at %s", time.Now().Format(time.RFC3339))
}
