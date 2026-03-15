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

	// PIR Motion Sensor → triggers camera capture + face recognition
	client.Subscribe("home/door/pir", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTT] ← home/door/pir payload=%q", string(msg.Payload()))
		go cameraService.HandleMotion()
	})

	// Vibration Sensor → triggers intrusion alert
	client.Subscribe("home/door/vibration", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTT] ← home/door/vibration payload=%q", string(msg.Payload()))
		go intrusionService.HandleIntrusion()
	})

	log.Println("[MQTT] Subscribers started on: home/door/pir, home/door/vibration")
}
