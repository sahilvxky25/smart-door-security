package flows

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/calls"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
)

const (
	vibrationDebounce = 10 * time.Second
	magneticDebounce  = 2 * time.Second
	leftOpenTimeout   = 18 * time.Second
)

type intrusionDoor interface {
	IsMotorAtLockedPosition() bool
}

type intrusionSecurityState interface {
	ActivateIntrusion(source string)
	IsAuthWindowActive(window time.Duration) bool
}

type intrusionEventLogger interface {
	LogEvent(eventType string, imageURL string) (*models.Event, error)
}

type intrusionFaceRecognizer interface {
	CaptureAndRecognize() (*services.FaceRecognitionResult, error)
}

type intrusionMediaStorage interface {
	UploadImage(ctx context.Context, objectName string, data []byte, contentType string) (string, error)
}

type intrusionNotifier interface {
	GetCallStatus(callID string) (calls.CallStatus, bool)
	HasActiveCallForEventType(eventType string) bool
	Notify(eventType, imageURL string)
	TriggerIncomingCall(eventType, imageURL string) string
}

type IntrusionFlow struct {
	doorService   intrusionDoor
	securityState intrusionSecurityState
	eventService  intrusionEventLogger
	faceService   intrusionFaceRecognizer
	storage       intrusionMediaStorage
	soundService  soundPlayer
	notify        intrusionNotifier

	vibrationMu    sync.Mutex
	vibrationFired time.Time
	lastCallID     string

	doorMu        sync.Mutex
	isDoorOpen    bool
	lastDoorEvent time.Time
	leftOpenTimer *time.Timer
}

func NewIntrusionFlow(
	doorService intrusionDoor,
	securityState intrusionSecurityState,
	eventService intrusionEventLogger,
	faceService intrusionFaceRecognizer,
	storage intrusionMediaStorage,
	soundService soundPlayer,
	notify intrusionNotifier,
) *IntrusionFlow {
	return &IntrusionFlow{
		doorService:   doorService,
		securityState: securityState,
		eventService:  eventService,
		faceService:   faceService,
		storage:       storage,
		soundService:  soundService,
		notify:        notify,
	}
}

func (f *IntrusionFlow) HandleVibrationDetected() {
	if f.securityState != nil && f.securityState.IsAuthWindowActive(services.AutoLockDelay) && !f.doorService.IsMotorAtLockedPosition() {
		log.Println("[IntrusionFlow] Vibration detected but suppressed (auth window active and motor not at 0)")
		return
	}

	f.vibrationMu.Lock()
	if f.lastCallID != "" {
		status, ok := f.notify.GetCallStatus(f.lastCallID)
		if ok {
			if status == calls.StatusRinging || status == calls.StatusAccepted {
				f.vibrationMu.Unlock()
				log.Printf("[IntrusionFlow] Suppressed: call %s still %s", f.lastCallID, status)
				return
			}
			if status == calls.StatusDeclined {
				declinedID := f.lastCallID
				f.lastCallID = ""
				log.Printf("[IntrusionFlow] Last call %s was declined; re-arming", declinedID)
			} else {
				f.lastCallID = ""
			}
		} else {
			f.lastCallID = ""
		}
	}

	if f.notify.HasActiveCallForEventType(models.EventForcedEntry) {
		f.vibrationMu.Unlock()
		log.Println("[IntrusionFlow] Suppressed: forced-entry call already live")
		return
	}

	if time.Since(f.vibrationFired) < vibrationDebounce {
		f.vibrationMu.Unlock()
		return
	}
	f.vibrationFired = time.Now()
	f.vibrationMu.Unlock()

	log.Println("[IntrusionFlow] Vibration detected -> triggering alert")
	f.securityState.ActivateIntrusion("vibration")
	f.soundService.PlaySOS()
	imageURL := f.captureEventImage()
	f.eventService.LogEvent(models.EventForcedEntry, imageURL)
	callID := f.notify.TriggerIncomingCall(models.EventForcedEntry, imageURL)

	f.vibrationMu.Lock()
	f.lastCallID = callID
	f.vibrationMu.Unlock()

	log.Printf("[IntrusionFlow] SOS alert and video call triggered at %s", time.Now().Format(time.RFC3339))
}

func (f *IntrusionFlow) HandleDoorOpened() {
	f.doorMu.Lock()
	if f.isDoorOpen {
		f.doorMu.Unlock()
		return
	}
	if time.Since(f.lastDoorEvent) < magneticDebounce {
		f.doorMu.Unlock()
		return
	}
	f.lastDoorEvent = time.Now()
	f.isDoorOpen = true
	f.doorMu.Unlock()

	authorizedOpen := f.securityState != nil && f.securityState.IsAuthWindowActive(services.AutoLockDelay)
	if !authorizedOpen {
		log.Println("[IntrusionFlow] Forced entry detected: door opened without recent authentication")
		f.securityState.ActivateIntrusion("magnetic")
		f.soundService.PlaySOS()
		imageURL := f.captureEventImage()
		f.eventService.LogEvent(models.EventForcedEntry, imageURL)
		if f.notify.HasActiveCallForEventType(models.EventForcedEntry) {
			log.Println("[IntrusionFlow] Forced-entry call already live - suppressing duplicate incoming call")
		} else {
			f.notify.TriggerIncomingCall(models.EventForcedEntry, imageURL)
		}
		return
	}

	log.Println("[IntrusionFlow] Door opened (authorized via recent auth window)")
	f.eventService.LogEvent(models.EventDoorOpened, "")

	f.doorMu.Lock()
	f.leftOpenTimer = time.AfterFunc(leftOpenTimeout, func() {
		f.handleAuthorizedDoorOpenTimeout()
	})
	f.doorMu.Unlock()
}

func (f *IntrusionFlow) HandleDoorClosed() {
	f.doorMu.Lock()
	if !f.isDoorOpen {
		f.doorMu.Unlock()
		return
	}
	if time.Since(f.lastDoorEvent) < magneticDebounce {
		f.doorMu.Unlock()
		return
	}
	f.lastDoorEvent = time.Now()
	f.isDoorOpen = false

	if f.leftOpenTimer != nil {
		f.leftOpenTimer.Stop()
		f.leftOpenTimer = nil
	}
	f.doorMu.Unlock()

	log.Println("[IntrusionFlow] Door closed")
	f.eventService.LogEvent(models.EventDoorClosed, "")
}

func (f *IntrusionFlow) handleAuthorizedDoorOpenTimeout() {
	f.doorMu.Lock()
	if !f.isDoorOpen {
		f.doorMu.Unlock()
		return
	}
	f.leftOpenTimer = nil
	f.doorMu.Unlock()

	log.Println("[IntrusionFlow] Door left open - playing SOS alert")
	f.soundService.PlaySOS()
	f.eventService.LogEvent(models.EventDoorLeftOpen, "")
	f.notify.Notify(models.EventDoorLeftOpen, "")
}

func (f *IntrusionFlow) captureEventImage() string {
	if f.faceService == nil || f.storage == nil {
		return ""
	}

	result, err := f.faceService.CaptureAndRecognize()
	if err != nil {
		log.Printf("[IntrusionFlow] Failed to capture forced-entry frame: %v", err)
		return ""
	}
	if result == nil || len(result.FrameJPG) == 0 {
		return ""
	}

	objectName := fmt.Sprintf("events/%d.jpg", time.Now().UnixMilli())
	imageURL, err := f.storage.UploadImage(context.Background(), objectName, result.FrameJPG, "image/jpeg")
	if err != nil {
		log.Printf("[IntrusionFlow] Failed to upload forced-entry frame: %v", err)
		return ""
	}
	return imageURL
}
