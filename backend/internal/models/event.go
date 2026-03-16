package models

import "time"

const (
	EventAuthorizedEntry    = "AUTHORIZED_ENTRY"
	EventUnknownVisitor     = "UNKNOWN_VISITOR"
	EventForcedEntry        = "FORCED_ENTRY"
	EventManualUnlock       = "MANUAL_UNLOCK"
	EventSpoofAttempt       = "SPOOF_ATTEMPT"
	EventDoorOpened         = "DOOR_OPENED"
	EventDoorClosed         = "DOOR_CLOSED"
	EventDoorLeftOpen       = "DOOR_LEFT_OPEN"
	EventVisitorApproaching = "VISITOR_APPROACHING"
	EventHandleTamper       = "HANDLE_TAMPER"
)

type Event struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	UserID    *uint     `json:"user_id"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ImageURL  string    `json:"image_url"`
}
