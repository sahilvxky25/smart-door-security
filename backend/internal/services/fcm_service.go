package services

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// FCMService handles communication with Firebase Cloud Messaging
type FCMService struct {
	client *messaging.Client
}

// NewFCMService initializes the Firebase SDK using a service account credentials file
func NewFCMService(ctx context.Context, credentialsFile string) (*FCMService, error) {
	var app *firebase.App
	var err error

	if credentialsFile != "" {
		opt := option.WithCredentialsFile(credentialsFile)
		app, err = firebase.NewApp(ctx, nil, opt)
	} else {
		// Try application default credentials if no file is provided
		app, err = firebase.NewApp(ctx, nil)
	}

	if err != nil {
		return nil, err
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}

	log.Println("[FCMService] Firebase Cloud Messaging initialized successfully")
	return &FCMService{client: client}, nil
}

// SendCallNotification pushes a high-priority data message to the device to trigger CallKit
func (f *FCMService) SendCallNotification(ctx context.Context, token, callID, eventType, imageURL string) error {
	message := &messaging.Message{
		Token: token,
		Data: map[string]string{
			"type":       "incoming_call",
			"call_id":    callID,
			"event_type": eventType,
			"image_url":  imageURL,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high", // Required to wake app in background
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10", // 10 = High priority (required for pushes that alert user)
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					ContentAvailable: true,
				},
			},
		},
	}

	response, err := f.client.Send(ctx, message)
	if err != nil {
		log.Printf("[FCMService] Failed to send call notification to token %s: %v", token, err)
		return err
	}

	log.Printf("[FCMService] Successfully sent call notification. MessageID: %s", response)
	return nil
}

// SendMissedCallNotification informs the app to dismiss the incoming call screen
func (f *FCMService) SendMissedCallNotification(ctx context.Context, token, callID string) error {
	message := &messaging.Message{
		Token: token,
		Data: map[string]string{
			"type":    "missed_call",
			"call_id": callID,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					ContentAvailable: true,
				},
			},
		},
	}

	response, err := f.client.Send(ctx, message)
	if err != nil {
		log.Printf("[FCMService] Failed to send missed call notification to token %s: %v", token, err)
		return err
	}

	log.Printf("[FCMService] Successfully sent missed call notification. MessageID: %s", response)
	return nil
}


