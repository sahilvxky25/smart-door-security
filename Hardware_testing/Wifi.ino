// ESP32 WiFi Test Code

#include <WiFi.h>

// Replace with your network credentials
const char* ssid = "LAPTOP-KH5BNQLP 5885";
const char* password = "987654321";

void setup() {
  Serial.begin(115200);
  
  Serial.println("Connecting to WiFi...");
  
  WiFi.begin(ssid, password);

  // Wait for connection
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }

  Serial.println("\nWiFi Connected!");
  Serial.print("IP Address: ");
  Serial.println(WiFi.localIP());
}

void loop() {
  Serial.println("ESP32 WiFi is working...");
  Serial.println(WiFi.localIP());
  delay(3000);
}