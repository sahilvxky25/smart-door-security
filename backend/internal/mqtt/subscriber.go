package mqtt

import (
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
)

func StartSubscribers(
	client mqtt.Client,
	cameraService *services.CameraService,
	intrusionService *services.IntrusionService,
) {

	// PIR Motion Sensor
	client.Subscribe("home/door/pir", 0, func(client mqtt.Client, msg mqtt.Message) {

		log.Println("PIR motion detected:", string(msg.Payload()))

		cameraService.HandleMotion()
	})

	// Vibration Sensor (Intrusion)
	client.Subscribe("home/door/vibration", 0, func(client mqtt.Client, msg mqtt.Message) {

		log.Println("Door vibration detected:", string(msg.Payload()))

		intrusionService.HandleIntrusion()
	})

	log.Println("MQTT subscribers started")
}