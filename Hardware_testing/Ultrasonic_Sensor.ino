// ESP32 Ultrasonic Sensor Test (HC-SR04)

#define TRIG_PIN 5
#define ECHO_PIN 18

long duration;
float distance;

void setup() {
  Serial.begin(115200);

  pinMode(TRIG_PIN, OUTPUT);
  pinMode(ECHO_PIN, INPUT);

  Serial.println("Ultrasonic Sensor Test Started...");
}

void loop() {

  // Ensure trigger is LOW
  digitalWrite(TRIG_PIN, LOW);
  delayMicroseconds(2);

  // Send 10 µs pulse
  digitalWrite(TRIG_PIN, HIGH);
  delayMicroseconds(10);
  digitalWrite(TRIG_PIN, LOW);

  // Measure echo duration
  duration = pulseIn(ECHO_PIN, HIGH);

  // Calculate distance (cm)
  distance = duration * 0.0343 / 2;

  // Print result
  Serial.print("Distance: ");
  Serial.print(distance);
  Serial.println(" cm");

  delay(500);
}