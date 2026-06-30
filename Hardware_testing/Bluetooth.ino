// ESP32 Bluetooth Test Code

#include "BluetoothSerial.h"

BluetoothSerial SerialBT;

void setup() {
  Serial.begin(115200);

  // Start Bluetooth with device name
  SerialBT.begin("ESP32_TEST");

  Serial.println("Bluetooth Started! Pair with ESP32_TEST");
}

void loop() {

  // Receive data from phone
  if (SerialBT.available()) {
    char incoming = SerialBT.read();
    Serial.print("Received: ");
    Serial.println(incoming);

    // Send response back
    SerialBT.print("You sent: ");
    SerialBT.println(incoming);
  }

  // Send periodic message
  SerialBT.println("ESP32 is working...");
  delay(2000);
}