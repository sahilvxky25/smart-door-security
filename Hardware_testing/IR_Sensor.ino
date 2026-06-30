// ESP32 IR Sensor Test Code

#define IR_PIN 23

void setup() {
  Serial.begin(115200);
  pinMode(IR_PIN, INPUT);

  Serial.println("IR Sensor Test Started...");
  delay(1000);
}

void loop() {

  int irState = digitalRead(IR_PIN);

  if (irState == LOW) {
    Serial.println("Obstacle Detected");
  } else {
    Serial.println("No Obstacle");
  }

  delay(300);
}