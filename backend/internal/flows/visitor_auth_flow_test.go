package flows

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
)

type testFaceRecognizer struct {
	mu     sync.Mutex
	calls  int
	result *services.FaceRecognitionResult
	err    error
}

func (f *testFaceRecognizer) CaptureAndRecognizeForUser(userID uint) (*services.FaceRecognitionResult, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.result, f.err
}

func (f *testFaceRecognizer) CaptureAndRecognizeCandidates(candidates []services.FaceCandidate) (*services.FaceRecognitionResult, error) {
	return f.CaptureAndRecognizeForUser(0)
}

func (f *testFaceRecognizer) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type blockingFaceRecognizer struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
	result  *services.FaceRecognitionResult
}

func (f *blockingFaceRecognizer) CaptureAndRecognizeForUser(userID uint) (*services.FaceRecognitionResult, error) {
	f.mu.Lock()
	f.calls++
	if f.calls == 1 {
		close(f.entered)
	}
	f.mu.Unlock()

	<-f.release
	return f.result, nil
}

func (f *blockingFaceRecognizer) CaptureAndRecognizeCandidates(candidates []services.FaceCandidate) (*services.FaceRecognitionResult, error) {
	return f.CaptureAndRecognizeForUser(0)
}

func (f *blockingFaceRecognizer) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
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

func (l *testEventLogger) LogEventWithFamilyMember(eventType string, imageURL string, familyMember string) (*models.Event, error) {
	return &models.Event{EventType: eventType, ImageURL: imageURL, FamilyMember: familyMember}, nil
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
		func() *uint {
			id := uint(1)
			return &id
		},
		nil,
	)

	flow.HandleMotionDetected()

	if face.Calls() != 0 {
		t.Fatalf("expected face recognition to be skipped while intrusion is active, got %d calls", face.Calls())
	}
	if door.unlockCalls != 0 {
		t.Fatalf("expected no unlock while intrusion is active, got %d", door.unlockCalls)
	}

	security.intrusionActive = false

	flow.HandleMotionDetected()

	if face.Calls() != 1 {
		t.Fatalf("expected face recognition to resume after intrusion was cleared, got %d calls", face.Calls())
	}
	if door.unlockCalls != 1 {
		t.Fatalf("expected unlock after intrusion was cleared, got %d", door.unlockCalls)
	}
	if sound.welcomeCalls != 1 {
		t.Fatalf("expected welcome sound after authorized entry, got %d", sound.welcomeCalls)
	}
}

func TestVisitorAuthFlowSkipsDuplicateScanWhileAuthActive(t *testing.T) {
	face := &blockingFaceRecognizer{
		entered: make(chan struct{}),
		release: make(chan struct{}),
		result: &services.FaceRecognitionResult{
			Match: true,
			User:  "owner",
		},
	}
	door := &testDoorAuthorizer{}
	security := &testSecurityState{}
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
		func() *uint {
			id := uint(1)
			return &id
		},
		nil,
	)

	done := make(chan struct{})
	go func() {
		flow.HandleMotionDetected()
		close(done)
	}()

	select {
	case <-face.entered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first face recognition call")
	}

	flow.HandleMotionDetected()

	if face.Calls() != 1 {
		t.Fatalf("expected duplicate PIR to be skipped while auth flow is active, got %d face calls", face.Calls())
	}
	if door.unlockCalls != 0 {
		t.Fatalf("expected no unlock before first auth response completes, got %d", door.unlockCalls)
	}

	close(face.release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first auth flow to complete")
	}

	if door.unlockCalls != 1 {
		t.Fatalf("expected exactly one unlock after auth response completes, got %d", door.unlockCalls)
	}
}
