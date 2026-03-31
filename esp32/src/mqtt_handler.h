#ifndef MQTT_HANDLER_H
#define MQTT_HANDLER_H

#include <PubSubClient.h>

/// Initialise WiFi and MQTT. Blocks until WiFi is connected.
void mqttInit();

/// Call every loop() iteration — handles reconnection and MQTT keep-alive.
void mqttLoop();

/// Publish a string payload to the given topic (QoS 0, non-retained).
void mqttPublish(const char* topic, const char* payload);

/// Returns a reference to the underlying PubSubClient (for advanced use).
PubSubClient& mqttClient();

#endif // MQTT_HANDLER_H
