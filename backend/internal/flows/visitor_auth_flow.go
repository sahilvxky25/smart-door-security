package flows

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
)

type faceRecognizer interface {
	CaptureAndRecognizeForUser(userID uint) (*services.FaceRecognitionResult, error)
	CaptureAndRecognizeCandidates(candidates []services.FaceCandidate) (*services.FaceRecognitionResult, error)
}

type doorAuthorizer interface {
	UnlockDoorAuthorized()
}

type authSecurityState interface {
	IsIntrusionActive() bool
}

type eventLogger interface {
	LogEvent(eventType string, imageURL string) (*models.Event, error)
	LogEventWithFamilyMember(eventType string, imageURL string, familyMember string) (*models.Event, error)
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

type faceCandidateStore interface {
	ListCandidates(userID uint) ([]services.FaceCandidate, error)
}

type VisitorAuthFlow struct {
	authFlowMu    sync.Mutex
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
	getOwnerID    func() *uint
	faceStore     faceCandidateStore
}

func normalizeMatchName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func validateCandidateMatch(candidates []services.FaceCandidate, result *services.FaceRecognitionResult) (bool, string) {
	if len(candidates) == 0 {
		return false, ""
	}

	memberID, err := strconv.ParseUint(strings.TrimSpace(result.MemberID), 10, 64)
	if err == nil && memberID != 0 {
		for _, c := range candidates {
			if uint64(c.MemberID) != memberID {
				continue
			}
			if result.User == "" || normalizeMatchName(c.Name) == normalizeMatchName(result.User) {
				return true, c.Name
			}
			return false, c.Name
		}
		return false, ""
	}

	matchedName := normalizeMatchName(result.User)
	if matchedName == "" {
		return false, ""
	}
	for _, c := range candidates {
		if normalizeMatchName(c.Name) == matchedName {
			return true, c.Name
		}
	}
	return false, ""
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
	getOwnerID func() *uint,
	faceStore faceCandidateStore,
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
		getOwnerID:    getOwnerID,
		faceStore:     faceStore,
	}
}

func (f *VisitorAuthFlow) HandleMotionDetected() {
	if !f.authFlowMu.TryLock() {
		log.Println("[VisitorAuthFlow] PIR triggered but visitor authentication is already active - skipping duplicate scan")
		return
	}
	defer f.authFlowMu.Unlock()

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

	if f.getOwnerID == nil {
		log.Println("[VisitorAuthFlow] PIR triggered but no owner resolver is configured - skipping face detection")
		return
	}
	ownerID := f.getOwnerID()
	if ownerID == nil || *ownerID == 0 {
		log.Println("[VisitorAuthFlow] PIR triggered but no active/configured owner is available - skipping face detection")
		return
	}
	var candidates []services.FaceCandidate
	if f.faceStore != nil {
		var err error
		candidates, err = f.faceStore.ListCandidates(*ownerID)
		if err != nil {
			log.Printf("[VisitorAuthFlow] Failed to load face candidates for owner %d: %v", *ownerID, err)
			return
		}
	}

	event, _ := f.eventService.LogEventWithDebounce(models.EventVisitorApproaching, "", services.VisitorAlertDebounce)
	if event != nil {
		log.Println("[VisitorAuthFlow] PIR + Ultrasonic match -> visitor approaching notification")
		f.notifySvc.Notify(models.EventVisitorApproaching, "")
	}

	log.Println("[VisitorAuthFlow] PIR + Ultrasonic match -> capturing and recognizing face")

	var result *services.FaceRecognitionResult
	var err error
	if f.faceStore != nil {
		result, err = f.faceService.CaptureAndRecognizeCandidates(candidates)
	} else {
		result, err = f.faceService.CaptureAndRecognizeForUser(*ownerID)
	}
	if err != nil {
		log.Println("[VisitorAuthFlow] Face service error:", err)
		return
	}
	if result.Match && len(candidates) > 0 {
		ok, canonicalName := validateCandidateMatch(candidates, result)
		if !ok {
			log.Printf(
				"[VisitorAuthFlow] Face match rejected: memberID=%q user=%q not found in current owner candidates",
				result.MemberID,
				result.User,
			)
			result.Match = false
			result.User = ""
		} else {
			result.User = canonicalName
		}
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
		f.eventService.LogEventWithFamilyMember(models.EventAuthorizedEntry, imageURL, result.User)

	default:
		log.Println("[VisitorAuthFlow] Unknown visitor -> storing image and triggering call")
		f.eventService.LogEvent(models.EventUnknownVisitor, imageURL)
		if f.mqttClient != nil {
			f.mqttClient.Publish("home/door/unknown_visitor", 0, false, imageURL)
		}
		if f.notifySvc.HasActiveCallForEventType(models.EventUnknownVisitor) {
			log.Println("[VisitorAuthFlow] Unknown visitor call already live -> suppressing duplicate incoming call")
		} else {
			f.notifySvc.TriggerIncomingCall(models.EventUnknownVisitor, imageURL)
		}
	}
}
