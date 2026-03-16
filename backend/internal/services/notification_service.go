package services

import (
	"log"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/webrtc"
)

// NotificationService broadcasts real-time security alerts to connected owner
// clients over the WebSocket signaling hub.
type NotificationService struct {
	hub *webrtc.Hub
}

func NewNotificationService(hub *webrtc.Hub) *NotificationService {
	return &NotificationService{hub: hub}
}

// Notify sends a structured alert to all owner WebSocket clients.
// eventType must be one of the models.Event* constants.
// imageURL may be empty when no image is associated with the event.
func (n *NotificationService) Notify(eventType, imageURL string) {
	title, body := alertText(eventType)
	log.Printf("[NotificationService] %s – notifying owner at %s", eventType, time.Now().Format(time.RFC3339))
	n.hub.BroadcastAlert(eventType, title, body, imageURL)
}

// NotifyUnknownVisitor sends a legacy unknown_visitor WebSocket message to all
// owner clients. This uses a dedicated message format (type="unknown_visitor")
// that triggers the video call UI in the Flutter app in addition to showing a
// local notification.
func (n *NotificationService) NotifyUnknownVisitor(imageURL string) {
	log.Printf("[NotificationService] Unknown visitor – notifying owner at %s", time.Now().Format(time.RFC3339))
	n.hub.NotifyOwner(imageURL)
}

// alertText returns a human-readable title and body for each event type.
func alertText(eventType string) (title, body string) {
	switch eventType {
	case models.EventForcedEntry:
		return "Forced Entry Detected!", "Someone is attempting to break into your home."
	case models.EventSpoofAttempt:
		return "Spoof Attempt Blocked", "A spoofed face was detected at the door."
	case models.EventHandleTamper:
		return "Handle Tamper Alert", "The door handle is being tampered with."
	case models.EventDoorLeftOpen:
		return "Door Left Open", "Your door has been open for too long."
	case models.EventVisitorApproaching:
		return "Visitor Approaching", "Someone is approaching your door."
	case models.EventUnknownVisitor:
		return "Unknown Visitor", "An unrecognized person is at your door."
	default:
		return "Security Alert", "A security event occurred at your door."
	}
}
