package services

import (
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
)

const AutoLockDelay = 15 * time.Second

// DoorService controls the door servo via MQTT and tracks the expected
// servo angle so that MotorService can detect unauthorised movement.
type DoorService struct {
	mqtt          mqtt.Client
	events        *EventService
	securityState *SecurityStateService
	mu            sync.RWMutex
	expectedAngle int  // 0 = locked, 55 = unlocked
	currentAngle  int  // latest motor angle reported by ESP32
	hasMotorAngle bool // true once at least one motor reading is received
	autoLockTimer *time.Timer
	autoLockSeq   uint64
	lastAuthTime  time.Time // timestamp of last legitimate unlock
}

func NewDoorService(client mqtt.Client, events *EventService, securityState *SecurityStateService) *DoorService {
	return &DoorService{
		mqtt:          client,
		events:        events,
		securityState: securityState,
		expectedAngle: 0, // starts locked
	}
}

func (d *DoorService) UnlockDoor() {
	d.unlockDoor(true)
}

func (d *DoorService) UnlockDoorAuthorized() {
	d.unlockDoor(true)
}

func (d *DoorService) unlockDoor(recordAuth bool) {
	d.mu.Lock()
	d.expectedAngle = 55
	d.lastAuthTime = time.Now()
	d.autoLockSeq++
	currentSeq := d.autoLockSeq
	if d.autoLockTimer != nil {
		d.autoLockTimer.Stop()
	}
	d.autoLockTimer = time.AfterFunc(AutoLockDelay, func() {
		d.runAutoLock(currentSeq)
	})
	d.mu.Unlock()

	if recordAuth && d.securityState != nil {
		d.securityState.RecordAuthorization()
	}

	log.Println("[DoorService] Publishing UNLOCK (servo -> 55 deg)")
	d.mqtt.Publish("home/door/servo", 0, false, "UNLOCK")
	log.Printf("[DoorService] Auto-lock scheduled in %v", AutoLockDelay)
}

func (d *DoorService) LockDoor() {
	d.mu.Lock()
	d.autoLockSeq++
	d.expectedAngle = 0
	if d.autoLockTimer != nil {
		d.autoLockTimer.Stop()
		d.autoLockTimer = nil
	}
	d.mu.Unlock()

	log.Println("[DoorService] Publishing LOCK (servo -> 0 deg)")
	d.mqtt.Publish("home/door/servo", 0, false, "LOCK")
}

func (d *DoorService) runAutoLock(seq uint64) {
	d.mu.Lock()
	if seq != d.autoLockSeq || d.expectedAngle == 0 {
		d.mu.Unlock()
		return
	}
	d.autoLockTimer = nil
	d.mu.Unlock()

	d.LockDoor()
	if d.events != nil {
		d.events.LogEvent(models.EventManualLock, "")
	}
}

// ExpectedAngle returns the angle the backend last commanded.
func (d *DoorService) ExpectedAngle() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.expectedAngle
}

// IsAuthWindowActive returns true if an authorization occurred within the last 'window'.
func (d *DoorService) IsAuthWindowActive(window time.Duration) bool {
	if d.securityState != nil {
		return d.securityState.IsAuthWindowActive(window)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return time.Since(d.lastAuthTime) < window
}

// UpdateCurrentMotorAngle stores the latest angle reported by ESP32.
func (d *DoorService) UpdateCurrentMotorAngle(angle int) {
	d.mu.Lock()
	d.currentAngle = angle
	d.hasMotorAngle = true
	d.mu.Unlock()
}

// IsMotorAtLockedPosition returns true only when the latest reading is exactly lock angle (0).
func (d *DoorService) IsMotorAtLockedPosition() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.hasMotorAngle && d.currentAngle == 0
}
