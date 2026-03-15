package mqtt

import (
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func NewClient(broker string) mqtt.Client {

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID("smart-door-backend")

	client := mqtt.NewClient(opts)

	token := client.Connect()
	token.Wait()

	if token.Error() != nil {
		log.Fatal(token.Error())
	}

	log.Println("MQTT connected")

	return client
}