package services

import (
	"log"
	"sync"
	"time"
)

// SecurityStateService owns cross-cutting security state so intrusion and
// visitor-auth policy is enforced from one place instead of per-sensor logic.
type SecurityStateService struct {
	mu              sync.RWMutex
	lastAuthTime    time.Time
	intrusionActive bool
	intrusionSource string
	intrusionSince  time.Time
}

func NewSecurityStateService() *SecurityStateService {
	return &SecurityStateService{}
}

func (s *SecurityStateService) RecordAuthorization() {
	s.mu.Lock()
	s.lastAuthTime = time.Now()
	s.mu.Unlock()
}

func (s *SecurityStateService) IsAuthWindowActive(window time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.lastAuthTime) < window
}

func (s *SecurityStateService) ActivateIntrusion(source string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.intrusionActive {
		return
	}

	s.intrusionActive = true
	s.intrusionSource = source
	s.intrusionSince = time.Now()
	log.Printf("[SecurityState] Intrusion activated by %s", source)
}

func (s *SecurityStateService) ClearIntrusion(reason string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.intrusionActive {
		return false
	}

	log.Printf("[SecurityState] Intrusion cleared (%s). Previous source=%s since=%s",
		reason,
		s.intrusionSource,
		s.intrusionSince.Format(time.RFC3339),
	)

	s.intrusionActive = false
	s.intrusionSource = ""
	s.intrusionSince = time.Time{}
	return true
}

func (s *SecurityStateService) IsIntrusionActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.intrusionActive
}
