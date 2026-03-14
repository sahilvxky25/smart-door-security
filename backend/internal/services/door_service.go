package services

import mqtt "github.com/eclipse/paho.mqtt.golang"

type DoorService struct {
	mqtt mqtt.Client
}

func NewDoorService(client mqtt.Client) *DoorService {
	return &DoorService{mqtt: client}
}

func (d *DoorService) UnlockDoor() {

	d.mqtt.Publish("home/door/servo", 0, false, "90")
}