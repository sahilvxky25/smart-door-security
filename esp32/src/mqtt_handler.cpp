#include "mqtt_handler.h"
#include "config.h"
#include "actuators.h"
#include <WiFi.h>
#include <time.h>

// ──────────────────────────────────────────────
//  Internals
// ──────────────────────────────────────────────
static WiFiClient   wifiClient;
static PubSubClient client(wifiClient);
static unsigned long lastReconnectAttempt = 0;

// ──────────────────────────────────────────────
//  Time helpers (NTP-based evening check)
// ──────────────────────────────────────────────

/// Returns true if the current local time is in the "evening"
/// window (>= EVENING_HOUR or < MORNING_HOUR), i.e. dark hours.
/// Falls back to true if NTP hasn't synced yet (safe default).
static bool isEvening() {
    struct tm timeinfo;
    if (!getLocalTime(&timeinfo)) {
        Serial.println("[Time] NTP not synced yet – defaulting to evening=true");
        return true;   // safe default: activate LED if time unknown
    }
    int hour = timeinfo.tm_hour;
    return (hour >= EVENING_HOUR || hour < MORNING_HOUR);
}

// ──────────────────────────────────────────────
//  MQTT incoming message callback
//  Handles:
//    home/door/servo           → "UNLOCK" / "LOCK"
//    home/door/proximity_alert → "VISITOR_NEAR" (LED only after evening)
// ──────────────────────────────────────────────
static void mqttCallback(char* topic, byte* payload, unsigned int length) {
    // Null-terminate the payload so we can use string functions
    char msg[length + 1];
    memcpy(msg, payload, length);
    msg[length] = '\0';

    Serial.printf("[MQTT] ← %s  payload=\"%s\"\n", topic, msg);

    // ---- Door Servo Command ----
    if (strcmp(topic, TOPIC_SERVO) == 0) {
        if (strcmp(msg, "UNLOCK") == 0) {
            servoUnlock();
        } else if (strcmp(msg, "LOCK") == 0) {
            servoLock();
        } else {
            Serial.printf("[MQTT] Unknown servo command: %s\n", msg);
        }
        return;
    }

    // ---- Proximity Alert (LED indicator — evening only) ----
    if (strcmp(topic, TOPIC_PROXIMITY_ALERT) == 0) {
        if (strcmp(msg, "VISITOR_NEAR") == 0) {
            if (isEvening()) {
                Serial.println("[MQTT] Proximity alert → flashing LED (evening mode)");
                ledFlash(LED_FLASH_DURATION);
            } else {
                Serial.println("[MQTT] Proximity alert received but daytime – LED skipped");
            }
        }
        return;
    }
}

// ──────────────────────────────────────────────
//  WiFi connection (blocking)
// ──────────────────────────────────────────────
static void connectWiFi() {
    Serial.printf("[WiFi] Connecting to %s ", WIFI_SSID);
    WiFi.mode(WIFI_STA);
    WiFi.begin(WIFI_SSID, WIFI_PASSWORD);

    while (WiFi.status() != WL_CONNECTED) {
        Serial.print(".");
        delay(WIFI_RETRY_DELAY);
    }

    Serial.printf("\n[WiFi] Connected — IP: %s\n", WiFi.localIP().toString().c_str());
}

// ──────────────────────────────────────────────
//  NTP time sync
// ──────────────────────────────────────────────
static void initNTP() {
    configTime(GMT_OFFSET_SEC, DAYLIGHT_OFFSET, NTP_SERVER);
    Serial.println("[Time] NTP configured – waiting for sync …");

    // Wait up to 5 seconds for initial sync
    struct tm timeinfo;
    for (int i = 0; i < 10; i++) {
        if (getLocalTime(&timeinfo)) {
            Serial.printf("[Time] NTP synced: %04d-%02d-%02d %02d:%02d:%02d\n",
                          timeinfo.tm_year + 1900, timeinfo.tm_mon + 1, timeinfo.tm_mday,
                          timeinfo.tm_hour, timeinfo.tm_min, timeinfo.tm_sec);
            return;
        }
        delay(500);
    }
    Serial.println("[Time] NTP sync timeout – will retry in background");
}

// ──────────────────────────────────────────────
//  MQTT (re)connection — non-blocking after initial
// ──────────────────────────────────────────────
static bool mqttReconnect() {
    Serial.printf("[MQTT] Connecting to %s:%d as %s … ", MQTT_BROKER, MQTT_PORT, MQTT_CLIENT_ID);

    if (client.connect(MQTT_CLIENT_ID)) {
        Serial.println("connected");

        // Subscribe to topics the backend publishes to
        client.subscribe(TOPIC_SERVO);
        client.subscribe(TOPIC_PROXIMITY_ALERT);
        Serial.println("[MQTT] Subscribed: " TOPIC_SERVO ", " TOPIC_PROXIMITY_ALERT);
        return true;
    }

    Serial.printf("failed (rc=%d)\n", client.state());
    return false;
}

// ──────────────────────────────────────────────
//  Public API
// ──────────────────────────────────────────────
void mqttInit() {
    connectWiFi();
    initNTP();

    client.setServer(MQTT_BROKER, MQTT_PORT);
    client.setCallback(mqttCallback);
    client.setBufferSize(512);          // enough for any payload

    // Block until first MQTT connection succeeds
    while (!client.connected()) {
        if (mqttReconnect()) break;
        delay(MQTT_RECONNECT_DELAY);
    }
}

void mqttLoop() {
    // Reconnect WiFi if lost
    if (WiFi.status() != WL_CONNECTED) {
        Serial.println("[WiFi] Connection lost – reconnecting …");
        connectWiFi();
    }

    // Reconnect MQTT if lost (non-blocking retry with delay)
    if (!client.connected()) {
        unsigned long now = millis();
        if (now - lastReconnectAttempt >= MQTT_RECONNECT_DELAY) {
            lastReconnectAttempt = now;
            mqttReconnect();
        }
        return;  // skip loop() iteration if not connected
    }

    client.loop();
}

void mqttPublish(const char* topic, const char* payload) {
    if (!client.connected()) {
        Serial.printf("[MQTT] Not connected – cannot publish to %s\n", topic);
        return;
    }
    client.publish(topic, payload);
    Serial.printf("[MQTT] → %s  payload=\"%s\"\n", topic, payload);
}

PubSubClient& mqttClient() {
    return client;
}
