#include "sensors.h"
#include "config.h"

// ──────────────────────────────────────────────
//  Initialisation
// ──────────────────────────────────────────────
void sensorsInit() {
    pinMode(PIN_PIR,             INPUT);
    pinMode(PIN_VIBRATION,       INPUT);
    pinMode(PIN_ULTRASONIC_TRIG, OUTPUT);
    pinMode(PIN_ULTRASONIC_ECHO, INPUT);
    pinMode(PIN_HALL,            INPUT);
    pinMode(PIN_MAGNETIC,        INPUT_PULLUP);  // reed switch needs pull-up

    digitalWrite(PIN_ULTRASONIC_TRIG, LOW);      // ensure trigger starts LOW
}

// ──────────────────────────────────────────────
//  PIR (HC-SR501) — active HIGH
// ──────────────────────────────────────────────
bool pirDetected() {
    return digitalRead(PIN_PIR) == HIGH;
}

// ──────────────────────────────────────────────
//  Vibration (801S) — active HIGH
// ──────────────────────────────────────────────
bool vibrationDetected() {
    return digitalRead(PIN_VIBRATION) == HIGH;
}

// ──────────────────────────────────────────────
//  Ultrasonic (HC-SR04) — returns distance in cm
//  Also used for proximity detection (< threshold).
//  Returns -1.0 on timeout.
// ──────────────────────────────────────────────
float ultrasonicReadCm() {
    // Send 10 µs trigger pulse
    digitalWrite(PIN_ULTRASONIC_TRIG, LOW);
    delayMicroseconds(2);
    digitalWrite(PIN_ULTRASONIC_TRIG, HIGH);
    delayMicroseconds(10);
    digitalWrite(PIN_ULTRASONIC_TRIG, LOW);

    // Measure echo pulse duration (timeout after 30 ms ≈ ~500 cm)
    unsigned long duration = pulseIn(PIN_ULTRASONIC_ECHO, HIGH, 30000);

    if (duration == 0) {
        return -1.0f;   // timeout — no object in range
    }

    // Speed of sound ≈ 343 m/s → distance = duration(µs) / 58.0
    return (float)duration / 58.0f;
}

// ──────────────────────────────────────────────
//  Hall Effect Sensor
// ──────────────────────────────────────────────
bool hallDetected() {
    return digitalRead(PIN_HALL) == HIGH;
}

// ──────────────────────────────────────────────
//  Magnetic Reed Switch — door open when switch
//  is open (HIGH with pull-up = magnet away = door open)
// ──────────────────────────────────────────────
bool magneticDoorOpen() {
    return digitalRead(PIN_MAGNETIC) == HIGH;
}
