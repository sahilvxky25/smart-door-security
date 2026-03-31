package services

import (
	"log"
	"sync"
	"time"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const autoLockDelay = 15 * time.Second

// DoorService controls the door servo via MQTT and tracks the expected
// servo angle so that MotorService can detect unauthorised movement.
type DoorService struct {
	mqtt          mqtt.Client
	events        *EventService
	mu            sync.RWMutex
	expectedAngle int       // 0 = locked, 90 = unlocked
	lastAuthTime  time.Time // timestamp of last legitimate unlock
}

func NewDoorService(client mqtt.Client, events *EventService) *DoorService {
	return &DoorService{
		mqtt:          client,
		events:        events,
		expectedAngle: 0, // starts locked
	}
}

func (d *DoorService) UnlockDoor() {
	d.mu.Lock()
	d.expectedAngle = 90
	d.lastAuthTime = time.Now()
	d.mu.Unlock()

	log.Println("[DoorService] Publishing UNLOCK (servo → 90°)")
	d.mqtt.Publish("home/door/servo", 0, false, "UNLOCK")

	// Auto-lock after delay
	go func() {
		log.Printf("[DoorService] Auto-lock scheduled in %v", autoLockDelay)
		time.Sleep(autoLockDelay)
		d.LockDoor()
		if d.events != nil {
			d.events.LogEvent(models.EventManualLock, "")
		}
	}()
}

func (d *DoorService) LockDoor() {
	d.mu.Lock()
	d.expectedAngle = 0
	d.mu.Unlock()

	log.Println("[DoorService] Publishing LOCK (servo → 0°)")
	d.mqtt.Publish("home/door/servo", 0, false, "LOCK")
}

// ExpectedAngle returns the angle the backend last commanded.
func (d *DoorService) ExpectedAngle() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.expectedAngle
}

// IsAuthWindowActive returns true if an authorization occurred within the last 'window'.
func (d *DoorService) IsAuthWindowActive(window time.Duration) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return time.Since(d.lastAuthTime) < window
}
