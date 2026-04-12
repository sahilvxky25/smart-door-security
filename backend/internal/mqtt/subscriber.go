package mqtt

import (
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/flows"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
)

func StartSubscribers(
	client mqtt.Client,
	visitorAuthFlow *flows.VisitorAuthFlow,
	intrusionFlow *flows.IntrusionFlow,
	proximityService *services.ProximityService,
	ultrasonicService *services.UltrasonicService,
	motorService *services.MotorService,
) {
	client.Subscribe("home/door/pir", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTT] <- home/door/pir payload=%q", string(msg.Payload()))
		go visitorAuthFlow.HandleMotionDetected()
	})

	client.Subscribe("home/door/vibration", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTT] <- home/door/vibration payload=%q", string(msg.Payload()))
		go intrusionFlow.HandleVibrationDetected()
	})

	client.Subscribe("home/door/proximity", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTT] <- home/door/proximity payload=%q", string(msg.Payload()))
		go proximityService.HandleProximityDetected()
	})

	client.Subscribe("home/door/ultrasonic", 1, func(client mqtt.Client, msg mqtt.Message) {
		payload := string(msg.Payload())
		log.Printf("[MQTT] <- home/door/ultrasonic payload=%q", payload)
		go ultrasonicService.HandleDistance(payload)
	})

	client.Subscribe("home/door/magnetic/open", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Println("[MQTT] <- home/door/magnetic/open (no payload)")
		go intrusionFlow.HandleDoorOpened()
	})

	client.Subscribe("home/door/magnetic/closed", 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Println("[MQTT] <- home/door/magnetic/closed (no payload)")
		go intrusionFlow.HandleDoorClosed()
	})

	client.Subscribe("home/door/motor", 1, func(client mqtt.Client, msg mqtt.Message) {
		payload := string(msg.Payload())
		log.Printf("[MQTT] <- home/door/motor payload=%q", payload)
		go motorService.HandleMotorReading(payload)
	})

	log.Println("[MQTT] Subscribers started on: home/door/pir, home/door/vibration, home/door/proximity, home/door/ultrasonic, home/door/magnetic/open, home/door/magnetic/closed, home/door/motor")
}
