package models

import "time"

const (
	EventAuthorizedEntry = "AUTHORIZED_ENTRY"
	EventUnknownVisitor  = "UNKNOWN_VISITOR"
	EventForcedEntry     = "FORCED_ENTRY"
	EventManualUnlock    = "MANUAL_UNLOCK"
	EventSpoofAttempt    = "SPOOF_ATTEMPT"
)

type Event struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	UserID    *uint     `json:"user_id"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ImageURL  string    `json:"image_url"`
}
