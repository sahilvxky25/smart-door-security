package models

import "time"

const (
	EventAuthorizedEntry    = "AUTHORIZED_ENTRY"
	EventUnknownVisitor     = "UNKNOWN_VISITOR"
	EventForcedEntry        = "FORCED_ENTRY"
	EventManualUnlock       = "MANUAL_UNLOCK"
	EventManualLock         = "MANUAL_LOCK"
	EventSpoofAttempt       = "SPOOF_ATTEMPT"
	EventDoorOpened         = "DOOR_OPENED"
	EventDoorClosed         = "DOOR_CLOSED"
	EventDoorLeftOpen       = "DOOR_LEFT_OPEN"
	EventVisitorApproaching = "VISITOR_APPROACHING"
	EventIntrusionCleared   = "INTRUSION_CLEARED"
	EventHandleTamper       = "HANDLE_TAMPER"
	EventMotorTamper        = "MOTOR_TAMPER"
)

type Event struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	FamilyMember string `json:"family_member,omitempty"`
	ImageURL  string    `json:"image_url"`
}
