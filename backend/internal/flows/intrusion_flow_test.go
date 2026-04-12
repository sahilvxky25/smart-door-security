package flows

import (
	"context"
	"testing"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/calls"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
)

type testIntrusionDoor struct {
	locked bool
}

func (d *testIntrusionDoor) IsMotorAtLockedPosition() bool {
	return d.locked
}

type testIntrusionSecurityState struct {
	authWindow      bool
	activateSources []string
}

func (s *testIntrusionSecurityState) ActivateIntrusion(source string) {
	s.activateSources = append(s.activateSources, source)
}

func (s *testIntrusionSecurityState) IsAuthWindowActive(window time.Duration) bool {
	return s.authWindow
}

type loggedEvent struct {
	eventType string
	imageURL  string
}

type testIntrusionEventLogger struct {
	events []loggedEvent
}

func (l *testIntrusionEventLogger) LogEvent(eventType string, imageURL string) (*models.Event, error) {
	l.events = append(l.events, loggedEvent{eventType: eventType, imageURL: imageURL})
	return &models.Event{EventType: eventType, ImageURL: imageURL}, nil
}

type testIntrusionFaceRecognizer struct {
	calls  int
	result *services.FaceRecognitionResult
	err    error
}

func (f *testIntrusionFaceRecognizer) CaptureAndRecognize() (*services.FaceRecognitionResult, error) {
	f.calls++
	return f.result, f.err
}

type testIntrusionMediaStorage struct {
	uploads int
	url     string
}

func (s *testIntrusionMediaStorage) UploadImage(ctx context.Context, objectName string, data []byte, contentType string) (string, error) {
	s.uploads++
	return s.url, nil
}

type triggeredCall struct {
	eventType string
	imageURL  string
}

type testIntrusionNotifier struct {
	activeCall bool
	calls      []triggeredCall
}

func (n *testIntrusionNotifier) GetCallStatus(callID string) (calls.CallStatus, bool) {
	return "", false
}

func (n *testIntrusionNotifier) HasActiveCallForEventType(eventType string) bool {
	return n.activeCall
}

func (n *testIntrusionNotifier) Notify(eventType, imageURL string) {}

func (n *testIntrusionNotifier) TriggerIncomingCall(eventType, imageURL string) string {
	n.calls = append(n.calls, triggeredCall{eventType: eventType, imageURL: imageURL})
	return "test-call-id"
}

func TestHandleVibrationDetectedCapturesImageForForcedEntry(t *testing.T) {
	door := &testIntrusionDoor{}
	security := &testIntrusionSecurityState{}
	events := &testIntrusionEventLogger{}
	face := &testIntrusionFaceRecognizer{
		result: &services.FaceRecognitionResult{FrameJPG: []byte("jpg-bytes")},
	}
	storage := &testIntrusionMediaStorage{url: "https://example.com/forced-entry.jpg"}
	sound := &testSoundPlayer{}
	notifier := &testIntrusionNotifier{}

	flow := NewIntrusionFlow(door, security, events, face, storage, sound, notifier)

	flow.HandleVibrationDetected()

	if face.calls != 1 {
		t.Fatalf("expected one frame capture, got %d", face.calls)
	}
	if storage.uploads != 1 {
		t.Fatalf("expected one image upload, got %d", storage.uploads)
	}
	if len(events.events) != 1 {
		t.Fatalf("expected one logged event, got %d", len(events.events))
	}
	if events.events[0].eventType != models.EventForcedEntry {
		t.Fatalf("expected logged event %q, got %q", models.EventForcedEntry, events.events[0].eventType)
	}
	if events.events[0].imageURL != storage.url {
		t.Fatalf("expected logged event image %q, got %q", storage.url, events.events[0].imageURL)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected one triggered call, got %d", len(notifier.calls))
	}
	if notifier.calls[0].imageURL != storage.url {
		t.Fatalf("expected call image %q, got %q", storage.url, notifier.calls[0].imageURL)
	}
	if sound.sosCalls != 1 {
		t.Fatalf("expected SOS to play once, got %d", sound.sosCalls)
	}
}
