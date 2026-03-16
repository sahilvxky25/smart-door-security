package services

import (
	"log"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"gorm.io/gorm"
)

type IntrusionService struct {
	db           *gorm.DB
	eventService *EventService
	soundService *SoundService
	notify       *NotificationService
}

func NewIntrusionService(db *gorm.DB, eventService *EventService, soundService *SoundService, notify *NotificationService) *IntrusionService {
	return &IntrusionService{
		db:           db,
		eventService: eventService,
		soundService: soundService,
		notify:       notify,
	}
}

func (i *IntrusionService) HandleIntrusion() {
	log.Println("[IntrusionService] Vibration detected → triggering alert")

	i.soundService.PlaySOS()
	i.eventService.LogEvent(models.EventForcedEntry, nil, "")
	i.notify.Notify(models.EventForcedEntry, "")

	log.Printf("[IntrusionService] SOS alert played and event logged at %s", time.Now().Format(time.RFC3339))
}
