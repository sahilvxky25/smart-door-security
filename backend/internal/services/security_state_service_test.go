package services

import (
	"testing"
	"time"
)

func TestSecurityStateServiceAuthWindow(t *testing.T) {
	state := NewSecurityStateService()

	if state.IsAuthWindowActive(100 * time.Millisecond) {
		t.Fatal("expected auth window to be inactive before authorization")
	}

	state.RecordAuthorization()

	if !state.IsAuthWindowActive(100 * time.Millisecond) {
		t.Fatal("expected auth window to be active after authorization")
	}

	time.Sleep(120 * time.Millisecond)

	if state.IsAuthWindowActive(100 * time.Millisecond) {
		t.Fatal("expected auth window to expire")
	}
}

func TestSecurityStateServiceIntrusionLifecycle(t *testing.T) {
	state := NewSecurityStateService()

	if state.IsIntrusionActive() {
		t.Fatal("expected intrusion to start inactive")
	}

	state.ActivateIntrusion("vibration")

	if !state.IsIntrusionActive() {
		t.Fatal("expected intrusion to become active")
	}

	if !state.ClearIntrusion("manual owner unlock") {
		t.Fatal("expected clear intrusion to report a state change")
	}

	if state.IsIntrusionActive() {
		t.Fatal("expected intrusion to clear")
	}
}
