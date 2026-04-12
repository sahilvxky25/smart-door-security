package flows

import (
	"context"
	"testing"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
)

type testFaceRecognizer struct {
	calls  int
	result *services.FaceRecognitionResult
	err    error
}

func (f *testFaceRecognizer) CaptureAndRecognize() (*services.FaceRecognitionResult, error) {
	f.calls++
	return f.result, f.err
}

type testDoorAuthorizer struct {
	unlockCalls int
}

func (d *testDoorAuthorizer) UnlockDoorAuthorized() {
	d.unlockCalls++
}

type testSecurityState struct {
	intrusionActive bool
}

func (s *testSecurityState) IsIntrusionActive() bool {
	return s.intrusionActive
}

type testEventLogger struct{}

func (l *testEventLogger) LogEvent(eventType string, imageURL string) (*models.Event, error) {
	return &models.Event{EventType: eventType, ImageURL: imageURL}, nil
}

func (l *testEventLogger) LogEventWithDebounce(eventType string, imageURL string, window time.Duration) (*models.Event, error) {
	return &models.Event{EventType: eventType, ImageURL: imageURL}, nil
}

type testMediaStorage struct{}

func (s *testMediaStorage) UploadImage(ctx context.Context, objectName string, data []byte, contentType string) (string, error) {
	return "https://example.com/test.jpg", nil
}

type testVisitorNotifier struct{}

func (n *testVisitorNotifier) Notify(eventType, imageURL string) {}

func (n *testVisitorNotifier) HasActiveCallForEventType(eventType string) bool {
	return false
}

func (n *testVisitorNotifier) TriggerIncomingCall(eventType, imageURL string) string {
	return "test-call-id"
}

type testSoundPlayer struct {
	welcomeCalls int
	sosCalls     int
}

func (s *testSoundPlayer) PlaySOS() {
	s.sosCalls++
}

func (s *testSoundPlayer) PlayWelcome() {
	s.welcomeCalls++
}

type testUltrasonicReader struct {
	atDoor bool
}

func (u *testUltrasonicReader) IsAtDoor() bool {
	return u.atDoor
}

func TestVisitorAuthFlowResumesAfterIntrusionCleared(t *testing.T) {
	face := &testFaceRecognizer{
		result: &services.FaceRecognitionResult{
			Match: true,
			User:  "owner",
		},
	}
	door := &testDoorAuthorizer{}
	security := &testSecurityState{intrusionActive: true}
	events := &testEventLogger{}
	storage := &testMediaStorage{}
	notifier := &testVisitorNotifier{}
	sound := &testSoundPlayer{}
	ultrasonic := &testUltrasonicReader{atDoor: true}

	flow := NewVisitorAuthFlow(
		face,
		door,
		security,
		events,
		storage,
		nil,
		notifier,
		sound,
		ultrasonic,
		func() bool { return false },
	)

	flow.HandleMotionDetected()

	if face.calls != 0 {
		t.Fatalf("expected face recognition to be skipped while intrusion is active, got %d calls", face.calls)
	}
	if door.unlockCalls != 0 {
		t.Fatalf("expected no unlock while intrusion is active, got %d", door.unlockCalls)
	}

	security.intrusionActive = false

	flow.HandleMotionDetected()

	if face.calls != 1 {
		t.Fatalf("expected face recognition to resume after intrusion was cleared, got %d calls", face.calls)
	}
	if door.unlockCalls != 1 {
		t.Fatalf("expected unlock after intrusion was cleared, got %d", door.unlockCalls)
	}
	if sound.welcomeCalls != 1 {
		t.Fatalf("expected welcome sound after authorized entry, got %d", sound.welcomeCalls)
	}
}
