void setup() {
  Serial.begin(115200);  // Start Serial communication
}

void loop() {
  if (Serial.available() > 0) {
    char input = Serial.read();   // Read incoming character
    char output;

    // Mapping logic
    switch (input) {
      case 'a':
        output = 'x';
        break;
      case 'b':
        output = 'y';
        break;
      case 'c':
        output = 'z';
        break;
      default:
        output = '?';  // Unknown input
        break;
    }

    Serial.print("Received: ");
    Serial.print(input);
    Serial.print(" -> Sent: ");
    Serial.println(output);

    Serial.write(output);  // Send back the mapped character
  }
}