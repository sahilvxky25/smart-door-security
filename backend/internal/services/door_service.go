package services

import (
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const autoLockDelay = 5 * time.Second

type DoorService struct {
	mqtt mqtt.Client
}

func NewDoorService(client mqtt.Client) *DoorService {
	return &DoorService{mqtt: client}
}

func (d *DoorService) UnlockDoor() {
	log.Println("[DoorService] Publishing UNLOCK (servo → 90°)")
	d.mqtt.Publish("home/door/servo", 0, false, "UNLOCK")

	// Auto-lock after delay
	go func() {
		log.Printf("[DoorService] Auto-lock scheduled in %v", autoLockDelay)
		time.Sleep(autoLockDelay)
		d.LockDoor()
	}()
}

func (d *DoorService) LockDoor() {
	log.Println("[DoorService] Publishing LOCK (servo → 0°)")
	d.mqtt.Publish("home/door/servo", 0, false, "LOCK")
}
