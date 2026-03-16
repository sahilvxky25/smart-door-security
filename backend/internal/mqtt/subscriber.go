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
	proximityService *services.ProximityService,
	ultrasonicService *services.UltrasonicService,
	hallService *services.HallService,
	doorStateService *services.DoorStateService,
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

	// Proximity Sensor (IR) → close-range presence at door
	client.Subscribe("home/door/proximity", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTT] ← home/door/proximity payload=%q", string(msg.Payload()))
		go proximityService.HandleProximityDetected()
	})

	// Ultrasonic Sensor → distance reading in cm (e.g. "85.5")
	client.Subscribe("home/door/ultrasonic", 1, func(client mqtt.Client, msg mqtt.Message) {
		payload := string(msg.Payload())
		log.Printf("[MQTT] ← home/door/ultrasonic payload=%q", payload)
		go ultrasonicService.HandleDistance(payload)
	})

	// Hall Effect Sensor → handle/latch movement detected
	client.Subscribe("home/door/hall", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTT] ← home/door/hall payload=%q", string(msg.Payload()))
		go hallService.HandleHallDetected()
	})

	// Magnetic Door Sensor (reed switch) → door open/closed state
	client.Subscribe("home/door/magnetic", 1, func(client mqtt.Client, msg mqtt.Message) {
		payload := string(msg.Payload())
		log.Printf("[MQTT] ← home/door/magnetic payload=%q", payload)
		go doorStateService.HandleMagneticSensor(payload)
	})

	log.Println("[MQTT] Subscribers started on: home/door/pir, home/door/vibration, home/door/proximity, home/door/ultrasonic, home/door/hall, home/door/magnetic")
}
