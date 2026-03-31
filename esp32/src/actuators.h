#ifndef ACTUATORS_H
#define ACTUATORS_H

#include <Arduino.h>

/// Initialise servo and LED pins.
void actuatorsInit();

/// Move the servo to the UNLOCKED position (90°).
void servoUnlock();

/// Move the servo to the LOCKED position (0°).
void servoLock();

/// Read the current servo position in degrees using motor.read().
int servoRead();

/// Turn the status LED ON.
void ledOn();

/// Turn the status LED OFF.
void ledOff();

/// Flash the status LED for `durationMs` milliseconds (non-blocking).
/// Call `ledUpdate()` in loop() to manage timing.
void ledFlash(unsigned long durationMs);

/// Call this every loop() iteration to handle non-blocking LED flash timeout.
void ledUpdate();

#endif // ACTUATORS_H
