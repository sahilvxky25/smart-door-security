#include <Arduino.h>
#include <ESP32Servo.h>

Servo myServo;
const int servoPin = 4;

int currentAngle = 0;  // Track current position

// Function to move servo smoothly
void moveSmooth(int targetAngle) {
  if (currentAngle < targetAngle) {
    for (int pos = currentAngle; pos <= targetAngle; pos++) {
      myServo.write(pos);
      delay(15); // Adjust speed (smaller = faster)
    }
  } else {
    for (int pos = currentAngle; pos >= targetAngle; pos--) {
      myServo.write(pos);
      delay(15);
    }
  }
  currentAngle = targetAngle; // Update position
}

void setup() {
  Serial.begin(115200);
  myServo.attach(servoPin);
  myServo.write(currentAngle);
  Serial.println("Servo initialized at 0°. Send 'OPEN' or 'CLOSE'");
}

void loop() {
  if (Serial.available() > 0) {
    String command = Serial.readStringUntil('\n');
    command.trim();

    if (command.equalsIgnoreCase("O")) {
      moveSmooth(55); // Smooth move to 90°
      Serial.println("Servo smoothly moved to 90°");
    } 
    else if (command.equalsIgnoreCase("P")) {
      moveSmooth(0);  // Smooth move back to 0°
      Serial.println("Servo smoothly returned to 0°");
    
      
    } 
    else {
      Serial.println("Unknown command. Use 'OPEN' or 'CLOSE'");
    }
  }
}