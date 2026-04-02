package services

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/webrtc"
)

// CallStatus represents the current state of a call in the state machine.
type CallStatus string

const (
	StatusRinging  CallStatus = "ringing"
	StatusAccepted CallStatus = "accepted"
	StatusDeclined CallStatus = "declined"
	StatusMissed   CallStatus = "missed"
	StatusEnded    CallStatus = "ended"
)

// CallSession represents an active or recently finished call.
type CallSession struct {
	ID        string
	Type      string // e.g., "unknown_visitor", "spoof_attempt"
	Status    CallStatus
	ImageURL  string
	CreatedAt time.Time
}

// CallManager tracks active call sessions and enforces timeouts.
type CallManager struct {
	mu       sync.RWMutex
	calls    map[string]*CallSession
	hub      *webrtc.Hub // For broadcasting state changes (e.g. missed call)
	timeout  time.Duration
}

func NewCallManager(hub *webrtc.Hub, timeout time.Duration) *CallManager {
	return &CallManager{
		calls:   make(map[string]*CallSession),
		hub:     hub,
		timeout: timeout,
	}
}

// CreateCallSession starts a new call session, sets it to ringing, and returns the CallID.
func (cm *CallManager) CreateCallSession(callType, imageURL string) string {
	callID := uuid.New().String()
	session := &CallSession{
		ID:        callID,
		Type:      callType,
		Status:    StatusRinging,
		ImageURL:  imageURL,
		CreatedAt: time.Now(),
	}

	cm.mu.Lock()
	cm.calls[callID] = session
	cm.mu.Unlock()

	log.Printf("[CallManager] Call %s created (Ringing) for type: %s", callID, callType)

	// Start the timeout goroutine
	go cm.watchTimeout(callID)

	return callID
}

// watchTimeout waits for the timeout duration. If the call is still ringing, it transitions it to missed.
func (cm *CallManager) watchTimeout(callID string) {
	time.Sleep(cm.timeout)

	cm.mu.Lock()
	session, exists := cm.calls[callID]
	if !exists {
		cm.mu.Unlock()
		return
	}

	if session.Status == StatusRinging {
		session.Status = StatusMissed
		cm.mu.Unlock()

		log.Printf("[CallManager] Call %s timeout out -> Missed", callID)
		
		// Broadcast missed call so the Flutter app can cancel the ringing UI
		cm.hub.BroadcastMissedCall(callID)
	} else {
		cm.mu.Unlock()
	}
}

// AcceptCall transitions a ringing call to accepted. Returns true if successful.
func (cm *CallManager) AcceptCall(callID string) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	session, exists := cm.calls[callID]
	if !exists {
		return false
	}

	if session.Status == StatusRinging {
		session.Status = StatusAccepted
		log.Printf("[CallManager] Call %s transitioning to Accepted", callID)
		return true
	}

	return false
}

// DeclineCall transitions a ringing call to declined. Returns true if successful.
func (cm *CallManager) DeclineCall(callID string) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	session, exists := cm.calls[callID]
	if !exists {
		return false
	}

	if session.Status == StatusRinging {
		session.Status = StatusDeclined
		log.Printf("[CallManager] Call %s transitioning to Declined", callID)
		return true
	}

	return false
}

// EndCall terminates an accepted call. Returns true if successful.
func (cm *CallManager) EndCall(callID string) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	session, exists := cm.calls[callID]
	if !exists {
		return false
	}

	if session.Status == StatusAccepted {
		session.Status = StatusEnded
		log.Printf("[CallManager] Call %s transitioning to Ended", callID)
		// Usually we'd clean it up here or keep it for history. We'll simply mark it ended.
		return true
	}
	
	return false
}

// GetCall returns the current status of a call session.
func (cm *CallManager) GetCall(callID string) (*CallSession, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	session, exists := cm.calls[callID]
	return session, exists
}
