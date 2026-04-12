package calls

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// HubInterface defines the required behavior from the WebRTC Hub to avoid cyclic dependencies.
type HubInterface interface {
	BroadcastMissedCall(callID string)
}

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
	mu           sync.RWMutex
	calls        map[string]*CallSession
	hub          HubInterface // For broadcasting state changes (e.g. missed call)
	timeout      time.Duration
	onMissedCall func(callID string)
	onDeclined   func(callID string, callType string)
}

func NewCallManager(hub HubInterface, timeout time.Duration) *CallManager {
	return &CallManager{
		calls:   make(map[string]*CallSession),
		hub:     hub,
		timeout: timeout,
	}
}

// SetHub sets the WebRTC hub for broadcasting after initialization to break cyclical dependencies.
func (cm *CallManager) SetHub(hub HubInterface) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.hub = hub
}

// SetOnMissedCall sets a callback executed when a ringing call times out
func (cm *CallManager) SetOnMissedCall(fn func(callID string)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onMissedCall = fn
}

// SetOnDeclinedCall sets a callback executed when a ringing call is declined.
func (cm *CallManager) SetOnDeclinedCall(fn func(callID string, callType string)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onDeclined = fn
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

		log.Printf("[CallManager] Call %s timed out -> Missed", callID)

		// Capture callback safely before unlocking
		var callback func(string)
		if cm.onMissedCall != nil {
			callback = cm.onMissedCall
		}
		cm.mu.Unlock() // Release lock before external broadcasts/callbacks

		// Broadcast missed call so the Flutter app can cancel the ringing UI
		cm.hub.BroadcastMissedCall(callID)

		if callback != nil {
			callback(callID)
		}
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
	session, exists := cm.calls[callID]
	if !exists {
		cm.mu.Unlock()
		return false
	}

	if session.Status == StatusRinging {
		session.Status = StatusDeclined
		log.Printf("[CallManager] Call %s transitioning to Declined", callID)
		callType := session.Type
		callback := cm.onDeclined
		cm.mu.Unlock()
		if callback != nil {
			callback(callID, callType)
		}
		return true
	}

	cm.mu.Unlock()
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

// HasLiveCallOfType reports whether a call of the given type is still ringing or accepted.
func (cm *CallManager) HasLiveCallOfType(callType string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, session := range cm.calls {
		if session.Type != callType {
			continue
		}
		if session.Status == StatusRinging || session.Status == StatusAccepted {
			return true
		}
	}

	return false
}
