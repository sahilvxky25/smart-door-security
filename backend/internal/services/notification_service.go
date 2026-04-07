package services

import (
	"context"
	"log"
	"time"

	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/calls"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/webrtc"
	"google.golang.org/api/option"
	"gorm.io/gorm"
)

// NotificationService broadcasts real-time security alerts to connected owner
// clients over the WebSocket signaling hub.
type NotificationService struct {
	hub         *webrtc.Hub
	callManager *calls.CallManager
	fcmClient   *messaging.Client
	db          *gorm.DB
}

func NewNotificationService(hub *webrtc.Hub, cm *calls.CallManager, db *gorm.DB) *NotificationService {
	creds := os.Getenv("FIREBASE_ADMIN_CREDENTIALS")
	if creds == "" {
		creds = "service-account.json"
	}
	opt := option.WithCredentialsFile(creds)
	app, err := firebase.NewApp(context.Background(), nil, opt)

	var fcmClient *messaging.Client
	if err == nil {
		fcmClient, err = app.Messaging(context.Background())
		if err != nil {
			log.Printf("[NotificationService] Failed to init FCM Client: %v", err)
		}
	} else {
		log.Printf("[NotificationService] Failed to init Firebase App: %v", err)
	}

	ns := &NotificationService{
		hub:         hub,
		callManager: cm,
		fcmClient:   fcmClient,
		db:          db,
	}

	cm.SetOnMissedCall(ns.onMissedCall)
	return ns
}

func (n *NotificationService) onMissedCall(callID string) {
	if n.fcmClient == nil {
		return
	}
	var users []models.User
	n.db.Where("fcm_token IS NOT NULL AND fcm_token != ''").Find(&users)

	for _, user := range users {
		msg := &messaging.Message{
			Token: user.FCMToken,
			Data: map[string]string{
				"type":    "missed_call",
				"call_id": callID,
			},
			Android: &messaging.AndroidConfig{Priority: "high"},
			APNS: &messaging.APNSConfig{
				Headers: map[string]string{"apns-priority": "10"},
				Payload: &messaging.APNSPayload{Aps: &messaging.Aps{ContentAvailable: true}},
			},
		}
		_, err := n.fcmClient.Send(context.Background(), msg)
		if err != nil {
			log.Printf("[NotificationService] FCM MissedCall error for user %d: %v", user.ID, err)
		}
	}
}

// Notify sends a structured alert to all owner WebSocket clients.
// eventType must be one of the models.Event* constants.
// imageURL may be empty when no image is associated with the event.
func (n *NotificationService) Notify(eventType, imageURL string) {
	title, body := alertText(eventType)
	log.Printf("[NotificationService] %s – notifying owner at %s", eventType, time.Now().Format(time.RFC3339))
	n.hub.BroadcastAlert(eventType, title, body, imageURL)
}

// TriggerIncomingCall sends an incoming_call WebSocket message to all owner
// clients. This triggers the high-priority "Awesome Notification" call UI.
func (n *NotificationService) TriggerIncomingCall(eventType, imageURL string) string {
	log.Printf("[NotificationService] Security event – triggering incoming call at %s", time.Now().Format(time.RFC3339))

	// Create a new call session which starts the ringing timeout machine
	callID := n.callManager.CreateCallSession(eventType, imageURL)

	// Broadcast to connected WebSockets
	n.hub.BroadcastIncomingCall(callID, imageURL)

	// Send FCM push notifications to all users with registered tokens
	if n.fcmClient != nil {
		var users []models.User
		n.db.Where("fcm_token IS NOT NULL AND fcm_token != ''").Find(&users)

		title, body := alertText(eventType)

		for _, user := range users {
			msg := &messaging.Message{
				Token: user.FCMToken,
				Data: map[string]string{
					"type":       "incoming_call",
					"call_id":    callID,
					"event_type": eventType,
					"image_url":  imageURL,
					"title":      title,
					"body":       body,
				},
				// Requires high priority to wake up backgrounded apps for CallKit
				Android: &messaging.AndroidConfig{Priority: "high"},
				APNS: &messaging.APNSConfig{
					Headers: map[string]string{"apns-priority": "10"},
					Payload: &messaging.APNSPayload{Aps: &messaging.Aps{ContentAvailable: true}},
				},
			}
			_, err := n.fcmClient.Send(context.Background(), msg)
			if err != nil {
				log.Printf("[NotificationService] FCM error for user %d: %v", user.ID, err)
			}
		}
	}

	return callID
}

func (n *NotificationService) GetCallStatus(callID string) (calls.CallStatus, bool) {
	call, ok := n.callManager.GetCall(callID)
	if !ok {
		return "", false
	}
	return call.Status, true
}

func (n *NotificationService) HasActiveCallForEventType(eventType string) bool {
	return n.callManager.HasLiveCallOfType(eventType)
}

// alertText returns a human-readable title and body for each event type.
func alertText(eventType string) (title, body string) {
	switch eventType {
	case models.EventForcedEntry:
		return "Forced Entry Detected!", "Someone is attempting to break into your home."
	case models.EventSpoofAttempt:
		return "Spoof Attempt Detected", "A spoofed face was detected at the door."
	case models.EventHandleTamper:
		return "Handle Tamper Alert", "The door handle is being tampered with."
	case models.EventDoorLeftOpen:
		return "Door Left Open", "Your door has been open for too long."
	case models.EventVisitorApproaching:
		return "Visitor Approaching", "Someone is approaching your door."
	case models.EventUnknownVisitor:
		return "Unknown Visitor", "An unrecognized person is at your door."
	case models.EventMotorTamper:
		return "Motor Tamper Detected!", "The door lock servo was moved without authorisation."
	default:
		return "Security Alert", "A security event occurred at your door."
	}
}
