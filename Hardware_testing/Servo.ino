#include <ESP32Servo.h>

Servo myServo;

int servoPin = 13;

void setup() {
  Serial.begin(115200);

  myServo.attach(servoPin);  // Attach servo to GPIO 18
  Serial.println("Servo Test Started");
}

void loop() {

  // Move from 0° to 180°
  for (int pos = 0; pos <= 180; pos += 5) {
    myServo.write(pos);
    Serial.print("Angle: ");
    Serial.println(pos);
    delay(20);
  }

  delay(1000);

  // Move from 180° to 0°
  for (int pos = 180; pos >= 0; pos -= 5) {
    myServo.write(pos);
    Serial.print("Angle: ");
    Serial.println(pos);
    delay(20);
  }

  delay(1000);
}