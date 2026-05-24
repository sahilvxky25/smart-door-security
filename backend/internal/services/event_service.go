package services

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"gorm.io/gorm"
)

const VisitorAlertDebounce = 1 * time.Minute

type EventService struct {
	db             *gorm.DB
	OnEventCreated func(event *models.Event)
	GetActiveOwner func() *uint

	// Debouncing logic for informational events (e.g. VISITOR_APPROACHING)
	lastFired sync.Map // map[string]time.Time
}

func NewEventService(db *gorm.DB) *EventService {
	return &EventService{db: db}
}

func (e *EventService) LogEvent(eventType string, imageURL string) (*models.Event, error) {
	return e.LogEventWithFamilyMember(eventType, imageURL, "")
}

func (e *EventService) LogEventWithFamilyMember(eventType string, imageURL string, familyMember string) (*models.Event, error) {
	userID, err := e.resolveEventUserID(eventType)
	if err != nil {
		return nil, err
	}

	event := models.Event{
		Timestamp: time.Now(),
		EventType: eventType,
		UserID:    userID,
		FamilyMember: familyMember,
		ImageURL:  imageURL,
	}

	if err := e.db.Create(&event).Error; err != nil {
		log.Printf("[EventService] Failed to create event: %v", err)
		return nil, err
	}

	log.Printf("[EventService] Created event: type=%s userID=%v familyMember=%q imageURL=%s", eventType, userID, familyMember, imageURL)

	if e.OnEventCreated != nil {
		e.OnEventCreated(&event)
	}

	return &event, nil
}

// LogEventWithDebounce works like LogEvent but skips if the same event type
// was logged within the specified window.
func (e *EventService) LogEventWithDebounce(eventType string, imageURL string, window time.Duration) (*models.Event, error) {
	if last, ok := e.lastFired.Load(eventType); ok {
		if time.Since(last.(time.Time)) < window {
			log.Printf("[EventService] Debounced %s – skipping repeated log", eventType)
			return nil, nil
		}
	}

	event, err := e.LogEvent(eventType, imageURL)
	if err == nil && event != nil {
		e.lastFired.Store(eventType, time.Now())
	}
	return event, err
}

func (e *EventService) resolveEventUserID(eventType string) (uint, error) {
	if e.GetActiveOwner != nil {
		if userID := e.GetActiveOwner(); userID != nil {
			return *userID, nil
		}
	}

	var user models.User
	if err := e.db.Order("id ASC").First(&user).Error; err != nil {
		return 0, fmt.Errorf("cannot log event %s: unable to resolve owner: %w", eventType, err)
	}

	log.Printf("[EventService] No active owner session for %s, falling back to userID=%d", eventType, user.ID)
	return user.ID, nil
}

func (e *EventService) ListEvents(userID uint, limit, offset int) ([]models.Event, error) {
	var events []models.Event
	query := e.db.Preload("User").Where("user_id = ?", userID).Order("timestamp DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (e *EventService) GetEvent(id uint) (*models.Event, error) {
	var event models.Event
	if err := e.db.Preload("User").First(&event, id).Error; err != nil {
		return nil, err
	}
	return &event, nil
}
