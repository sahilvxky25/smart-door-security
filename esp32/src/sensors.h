#ifndef SENSORS_H
#define SENSORS_H

#include <Arduino.h>

/// Initialise all sensor GPIO pins.
void sensorsInit();

/// Returns true when the PIR sensor detects motion (active HIGH).
bool pirDetected();

/// Returns true when the vibration sensor detects impact (active HIGH).
bool vibrationDetected();

/// Returns distance in cm from the HC-SR04 ultrasonic sensor.
/// Returns -1.0 if the reading timed out.
float ultrasonicReadCm();

/// Returns true when the Hall effect sensor detects a magnetic field change.
bool hallDetected();

/// Returns true when the magnetic reed switch indicates the door is OPEN.
/// (reed switch open = door open, since the magnet moves away)
bool magneticDoorOpen();

#endif // SENSORS_H
