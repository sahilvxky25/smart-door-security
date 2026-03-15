package services

import (
	"log"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"gorm.io/gorm"
)

type EventService struct {
	db *gorm.DB
}

func NewEventService(db *gorm.DB) *EventService {
	return &EventService{db: db}
}

func (e *EventService) LogEvent(eventType string, userID *uint, imageURL string) (*models.Event, error) {
	event := models.Event{
		Timestamp: time.Now(),
		EventType: eventType,
		UserID:    userID,
		ImageURL:  imageURL,
	}

	if err := e.db.Create(&event).Error; err != nil {
		log.Printf("[EventService] Failed to create event: %v", err)
		return nil, err
	}

	log.Printf("[EventService] Created event: type=%s userID=%v imageURL=%s", eventType, userID, imageURL)
	return &event, nil
}

func (e *EventService) ListEvents(limit, offset int) ([]models.Event, error) {
	var events []models.Event
	query := e.db.Preload("User").Order("timestamp DESC")
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
