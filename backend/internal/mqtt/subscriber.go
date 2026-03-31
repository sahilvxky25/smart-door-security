package mqtt

import (
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
)

func StartSubscribers(
	client mqtt.Client,
	cameraService *services.CameraService,
	vibrationService *services.VibrationService,
	proximityService *services.ProximityService,
	ultrasonicService *services.UltrasonicService,
	magneticService *services.MagneticService,
	motorService *services.MotorService,
) {

	// PIR Motion Sensor → triggers camera capture + face recognition
	client.Subscribe("home/door/pir", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTT] ← home/door/pir payload=%q", string(msg.Payload()))
		go cameraService.HandleMotion()
	})

	client.Subscribe("home/door/vibration", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTT] ← home/door/vibration payload=%q", string(msg.Payload()))
		go vibrationService.HandleVibration()
	})

	// Ultrasonic Proximity Sensor → close-range presence at door
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

	// Magnetic Door Sensor (reed switch) → door open/closed state
	// Handling multiple topics for payload-less operation
	client.Subscribe("home/door/magnetic/open", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Println("[MQTT] ← home/door/magnetic/open (no payload)")
		go magneticService.HandleDoorOpen()
	})

	client.Subscribe("home/door/magnetic/closed", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Println("[MQTT] ← home/door/magnetic/closed (no payload)")
		go magneticService.HandleDoorClose()
	})

	// Motor Angle Sensor → detects unauthorized servo movement (tamper)
	client.Subscribe("home/door/motor", 1, func(client mqtt.Client, msg mqtt.Message) {
		payload := string(msg.Payload())
		log.Printf("[MQTT] ← home/door/motor payload=%q", payload)
		go motorService.HandleMotorReading(payload)
	})

	log.Println("[MQTT] Subscribers started on: home/door/pir, home/door/vibration, home/door/proximity, home/door/ultrasonic, home/door/magnetic/open, home/door/magnetic/closed, home/door/motor")
}
