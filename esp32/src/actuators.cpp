#include "actuators.h"
#include "config.h"
#include <ESP32Servo.h>

// ──────────────────────────────────────────────
//  Internals
// ──────────────────────────────────────────────
static Servo doorServo;
static bool          ledFlashing   = false;
static unsigned long ledOffTime    = 0;

// ──────────────────────────────────────────────
//  Initialisation
// ──────────────────────────────────────────────
void actuatorsInit() {
    // Servo
    doorServo.attach(PIN_SERVO);
    doorServo.write(SERVO_LOCKED);       // start locked

    // LED
    pinMode(PIN_STATUS_LED, OUTPUT);
    digitalWrite(PIN_STATUS_LED, LOW);
}

// ──────────────────────────────────────────────
//  Servo
// ──────────────────────────────────────────────
void servoUnlock() {
    Serial.println("[Actuator] Servo → UNLOCKED (90°)");
    doorServo.write(SERVO_UNLOCKED);
}

void servoLock() {
    Serial.println("[Actuator] Servo → LOCKED (0°)");
    doorServo.write(SERVO_LOCKED);
}

int servoRead() {
    return doorServo.read();
}

// ──────────────────────────────────────────────
//  Status LED
// ──────────────────────────────────────────────
void ledOn() {
    digitalWrite(PIN_STATUS_LED, HIGH);
}

void ledOff() {
    digitalWrite(PIN_STATUS_LED, LOW);
    ledFlashing = false;
}

void ledFlash(unsigned long durationMs) {
    ledOn();
    ledFlashing = true;
    ledOffTime  = millis() + durationMs;
}

void ledUpdate() {
    if (ledFlashing && millis() >= ledOffTime) {
        ledOff();
    }
}
