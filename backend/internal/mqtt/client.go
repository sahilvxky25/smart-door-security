package mqtt

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func NewClient(broker string) mqtt.Client {

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID("smart-door-backend")

	client := mqtt.NewClient(opts)

	token := client.Connect()
	token.Wait()

	return client
}