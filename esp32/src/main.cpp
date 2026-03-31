// ─────────────────────────────────────────────────────────────
//  Smart Door Security – ESP32 Firmware
//  Main entry point
//
//  Reads sensors, publishes changes to the backend via MQTT,
//  and reacts to backend commands (servo lock/unlock, LED).
// ─────────────────────────────────────────────────────────────

#include <Arduino.h>
#include "config.h"
#include "sensors.h"
#include "actuators.h"
#include "mqtt_handler.h"

// ──────────────────────────────────────────────
//  State tracking for debounce & edge detection
// ──────────────────────────────────────────────
static unsigned long lastPir        = 0;
static unsigned long lastVibration  = 0;
static unsigned long lastProximity  = 0;
static unsigned long lastHall       = 0;
static unsigned long lastUltrasonic = 0;
static unsigned long lastMotorRead  = 0;

// Track previous door state for edge detection (only publish on change)
static bool prevDoorOpen = false;
static bool doorStateInitialised = false;

// Track previous motor position (only publish on change)
static int prevMotorPos = -1;

// ──────────────────────────────────────────────
//  setup()
// ──────────────────────────────────────────────
void setup() {
    Serial.begin(115200);
    Serial.println("\n========================================");
    Serial.println("  Smart Door Security – ESP32");
    Serial.println("========================================\n");

    sensorsInit();
    actuatorsInit();
    mqttInit();

    // Read initial door state
    prevDoorOpen = magneticDoorOpen();
    doorStateInitialised = true;
    mqttPublish(prevDoorOpen ? TOPIC_MAGNETIC_OPEN : TOPIC_MAGNETIC_CLOSED, "");

    // Read initial motor position
    prevMotorPos = servoRead();
    char buf[8];
    snprintf(buf, sizeof(buf), "%d", prevMotorPos);
    mqttPublish(TOPIC_MOTOR, buf);

    Serial.println("[Main] Setup complete – entering loop\n");
}

// ──────────────────────────────────────────────
//  loop()
// ──────────────────────────────────────────────
void loop() {
    mqttLoop();       // handle MQTT keep-alive & reconnection
    ledUpdate();      // handle non-blocking LED flash timeout

    unsigned long now = millis();

    // ── PIR Motion Sensor ──────────────────────
    if (pirDetected() && (now - lastPir >= DEBOUNCE_PIR)) {
        lastPir = now;
        mqttPublish(TOPIC_PIR, "DETECTED");
    }

    // ── Vibration Sensor (intrusion detection) ─
    if (vibrationDetected() && (now - lastVibration >= DEBOUNCE_VIBRATION)) {
        lastVibration = now;
        mqttPublish(TOPIC_VIBRATION, "INTRUSION");
    }

    // ── Hall Effect Sensor (reserved – not in final circuit, code kept for future use) ─
    // if (hallDetected() && (now - lastHall >= DEBOUNCE_HALL)) {
    //     lastHall = now;
    //     mqttPublish(TOPIC_HALL, "DETECTED");
    // }

    // ── Ultrasonic Distance (periodic) ─────────
    //    Also handles proximity detection:
    //    if distance < threshold → publish to proximity topic
    if (now - lastUltrasonic >= ULTRASONIC_INTERVAL) {
        lastUltrasonic = now;

        float distanceCm = ultrasonicReadCm();
        if (distanceCm >= 0) {
            // Publish raw distance (e.g. "85.5") – backend parses as float
            char buf[16];
            snprintf(buf, sizeof(buf), "%.1f", distanceCm);
            mqttPublish(TOPIC_ULTRASONIC, buf);

            // Proximity detection via ultrasonic
            if (distanceCm < PROXIMITY_THRESHOLD_CM &&
                (now - lastProximity >= DEBOUNCE_PROXIMITY)) {
                lastProximity = now;
                mqttPublish(TOPIC_PROXIMITY, "DETECTED");
            }
        }
    }

    // ── Magnetic Reed Switch (edge-triggered) ──
    bool currentDoorOpen = magneticDoorOpen();
    if (doorStateInitialised && currentDoorOpen != prevDoorOpen) {
        prevDoorOpen = currentDoorOpen;
        mqttPublish(currentDoorOpen ? TOPIC_MAGNETIC_OPEN : TOPIC_MAGNETIC_CLOSED, "");
    }

    // ── Motor Position Reading (periodic) ──────
    if (now - lastMotorRead >= MOTOR_READ_INTERVAL) {
        lastMotorRead = now;

        int currentPos = servoRead();
        if (currentPos != prevMotorPos) {
            prevMotorPos = currentPos;
            char buf[8];
            snprintf(buf, sizeof(buf), "%d", currentPos);
            mqttPublish(TOPIC_MOTOR, buf);
        }
    }

    // Small yield to avoid watchdog resets on tight loops
    delay(10);
}
