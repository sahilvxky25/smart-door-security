package services

import (
	"log"
	"math"
	"strconv"
	"sync"
	"time"
	// "github.com/gottatouchsomegrass/smart-door-backend/internal/models"
)

// MotorService monitors the servo motor angle reported by the ESP32.
// If the angle deviates from what the backend last commanded (via DoorService)
// by more than a tolerance, it is treated as physical tamper / intrusion.
//
// MQTT topic: home/door/motor   payload: angle in degrees (e.g. "90")
type MotorService struct {
	doorService  *DoorService
	eventService *EventService
	soundService *SoundService
	notify       *NotificationService
	mu           sync.Mutex
	lastFired    time.Time
}

const (
	motorAngleTolerance = 5            // degrees of acceptable deviation
	// motorDebounce       = 10 * time.Second // avoid repeated alerts
)

func NewMotorService(
	doorService *DoorService,
	eventService *EventService,
	soundService *SoundService,
	notify *NotificationService,
) *MotorService {
	return &MotorService{
		doorService:  doorService,
		eventService: eventService,
		soundService: soundService,
		notify:       notify,
	}
}

// HandleMotorReading is called each time the ESP32 publishes the current
// servo angle to home/door/motor. It compares against the expected angle
// from DoorService and raises an intrusion alert on mismatch.
func (m *MotorService) HandleMotorReading(payload string) {
	angle, err := strconv.Atoi(payload)
	if err != nil {
		log.Printf("[MotorService] Invalid motor payload %q: %v", payload, err)
		return
	}

	expected := m.doorService.ExpectedAngle()
	diff := math.Abs(float64(angle - expected))

	log.Printf("[MotorService] Motor angle=%d° expected=%d° diff=%.0f°", angle, expected, diff)

	if diff <= float64(motorAngleTolerance) {
		return // within tolerance, all good
	}

	// Angle deviates significantly from what the backend commanded
	// LOGGING ONLY: We no longer trigger a security alarm based on the motor angle.
	log.Printf("[MotorService] Angle deviation detected: expected %d° but got %d° (Alert disabled)", expected, angle)
}
