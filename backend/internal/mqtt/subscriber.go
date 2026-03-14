func StartSubscribers(
	client mqtt.Client,
	cameraService *services.CameraService,
	intrusionService *services.IntrusionService,
) {

	client.Subscribe("home/door/pir", 0, func(client mqtt.Client, msg mqtt.Message) {

		cameraService.HandleMotion()
	})

	client.Subscribe("home/door/vibration", 0, func(client mqtt.Client, msg mqtt.Message) {

		intrusionService.HandleIntrusion()
	})
}