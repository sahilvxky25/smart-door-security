package flows

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
)

type faceRecognizer interface {
	CaptureAndRecognize() (*services.FaceRecognitionResult, error)
}

type doorAuthorizer interface {
	UnlockDoorAuthorized()
}

type authSecurityState interface {
	IsIntrusionActive() bool
}

type eventLogger interface {
	LogEvent(eventType string, imageURL string) (*models.Event, error)
	LogEventWithDebounce(eventType string, imageURL string, window time.Duration) (*models.Event, error)
}

type mediaStorage interface {
	UploadImage(ctx context.Context, objectName string, data []byte, contentType string) (string, error)
}

type visitorNotifier interface {
	Notify(eventType, imageURL string)
	HasActiveCallForEventType(eventType string) bool
	TriggerIncomingCall(eventType, imageURL string) string
}

type soundPlayer interface {
	PlaySOS()
	PlayWelcome()
}

type ultrasonicReader interface {
	IsAtDoor() bool
}

type VisitorAuthFlow struct {
	faceService   faceRecognizer
	doorService   doorAuthorizer
	securityState authSecurityState
	eventService  eventLogger
	storage       mediaStorage
	mqttClient    mqtt.Client
	notifySvc     visitorNotifier
	soundService  soundPlayer
	ultrasonicSvc ultrasonicReader
	isCallActive  func() bool
}

func NewVisitorAuthFlow(
	faceService faceRecognizer,
	doorService doorAuthorizer,
	securityState authSecurityState,
	eventService eventLogger,
	store mediaStorage,
	mqttClient mqtt.Client,
	notifySvc visitorNotifier,
	soundService soundPlayer,
	ultrasonicSvc ultrasonicReader,
	isCallActive func() bool,
) *VisitorAuthFlow {
	return &VisitorAuthFlow{
		faceService:   faceService,
		doorService:   doorService,
		securityState: securityState,
		eventService:  eventService,
		storage:       store,
		mqttClient:    mqttClient,
		notifySvc:     notifySvc,
		soundService:  soundService,
		ultrasonicSvc: ultrasonicSvc,
		isCallActive:  isCallActive,
	}
}

func (f *VisitorAuthFlow) HandleMotionDetected() {
	if f.securityState != nil && f.securityState.IsIntrusionActive() {
		log.Println("[VisitorAuthFlow] PIR triggered while intrusion is active - skipping visitor authentication")
		return
	}

	if f.isCallActive != nil && f.isCallActive() {
		log.Println("[VisitorAuthFlow] PIR triggered but video call is active - skipping face detection")
		return
	}

	if f.ultrasonicSvc != nil && !f.ultrasonicSvc.IsAtDoor() {
		log.Println("[VisitorAuthFlow] PIR triggered but ultrasonic confirms no visitor at < 20cm - skipping camera")
		return
	}

	event, _ := f.eventService.LogEventWithDebounce(models.EventVisitorApproaching, "", services.VisitorAlertDebounce)
	if event != nil {
		log.Println("[VisitorAuthFlow] PIR + Ultrasonic match -> visitor approaching notification")
		f.notifySvc.Notify(models.EventVisitorApproaching, "")
	}

	log.Println("[VisitorAuthFlow] PIR + Ultrasonic match -> capturing and recognizing face")

	result, err := f.faceService.CaptureAndRecognize()
	if err != nil {
		log.Println("[VisitorAuthFlow] Face service error:", err)
		return
	}

	imageURL := ""
	if len(result.FrameJPG) > 0 {
		objectName := fmt.Sprintf("events/%d.jpg", time.Now().UnixMilli())
		url, err := f.storage.UploadImage(context.Background(), objectName, result.FrameJPG, "image/jpeg")
		if err != nil {
			log.Printf("[VisitorAuthFlow] Failed to upload image: %v", err)
		} else {
			imageURL = url
		}
	}

	switch {
	case result.Spoof:
		log.Println("[VisitorAuthFlow] Spoof detected -> creating event and triggering call")
		f.eventService.LogEvent(models.EventSpoofAttempt, imageURL)
		if f.notifySvc.HasActiveCallForEventType(models.EventSpoofAttempt) {
			log.Println("[VisitorAuthFlow] Spoof call already live -> suppressing duplicate incoming call")
		} else {
			f.notifySvc.TriggerIncomingCall(models.EventSpoofAttempt, imageURL)
		}
		f.soundService.PlaySOS()

	case result.Match:
		log.Printf("[VisitorAuthFlow] Authorized face %q -> unlocking door", result.User)
		f.doorService.UnlockDoorAuthorized()
		f.soundService.PlayWelcome()
		f.eventService.LogEvent(models.EventAuthorizedEntry, imageURL)

	default:
		log.Println("[VisitorAuthFlow] Unknown visitor -> storing image and triggering call")
		f.eventService.LogEvent(models.EventUnknownVisitor, imageURL)
		f.mqttClient.Publish("home/door/unknown_visitor", 0, false, imageURL)
		if f.notifySvc.HasActiveCallForEventType(models.EventUnknownVisitor) {
			log.Println("[VisitorAuthFlow] Unknown visitor call already live -> suppressing duplicate incoming call")
		} else {
			f.notifySvc.TriggerIncomingCall(models.EventUnknownVisitor, imageURL)
		}
	}
}
