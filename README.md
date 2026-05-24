# Smart Door Security System

An end-to-end IoT smart door platform that combines **ESP32 hardware**, a **Go/Python backend**, and a **Flutter mobile app** for secure and responsive home access control.

## 🎬 Project Showcase

<a href="https://youtu.be/wrQUuxkTJ_Y"><img src="https://img.youtube.com/vi/wrQUuxkTJ_Y/maxresdefault.jpg"></a>

## ✨ Core Features

- **Biometric access control** with face recognition and anti-spoofing checks.
- **Live two-way communication** using WebRTC audio/video.
- **Instant alerts** via Firebase push notifications.
- **Remote door control** (lock/unlock) through MQTT-driven commands.
- **Real-time event pipeline** between door hardware, backend, and mobile clients.

## 🧱 Architecture

### 1) Hardware (`/esp32`)
- Built with **C++** (PlatformIO) for ESP32.
- Reads sensors (PIR, reed switch) and drives actuators (lock, alarm).
- Publishes/subscribes through **MQTT** for low-latency state and command exchange.

### 2) Backend (`/backend`)
- **Go API server** for business logic, MQTT coordination, REST/WebSocket APIs, and WebRTC signaling.
- **Python face service** for recognition + anti-spoofing (OpenCV/MediaPipe).
- Infra via **Docker Compose** (database + Mosquitto broker + backend stack).

### 3) Mobile App (`/application`)
- Cross-platform app built with **Flutter**.
- Supports incoming-call-like UX, live monitoring, notification handling, and remote unlock.

## 🛠️ Tech Stack

- **Languages:** Dart, Go, C++, Python  
- **Frameworks/Libraries:** Flutter, Gin, OpenCV, MediaPipe  
- **Protocols:** WebRTC, MQTT, WebSockets, REST  
- **Infrastructure/Tools:** Docker, Firebase, PlatformIO

## 🚀 Getting Started

### 1. Backend

See: [`/backend/RUNNING.md`](./backend/RUNNING.md)

Quick start (recommended):

```bash
cd backend
docker compose up --build
```

The API is exposed at `http://localhost:8080`.

### 2. ESP32 Firmware

1. Open `/esp32` in PlatformIO (VS Code extension).
2. Configure network/broker values in [`/esp32/src/config.h`](./esp32/src/config.h).
3. Build/flash firmware to your ESP32 board.

### 3. Flutter App

```bash
cd application
flutter pub get
flutter run
```

## 📚 Documentation

- Backend run guide: [`/backend/RUNNING.md`](./backend/RUNNING.md)
- Database schema: [`/docs/DATABASE_SCHEMA.md`](./docs/DATABASE_SCHEMA.md)
- WebRTC setup: [`/docs/webrtc-setup.md`](./docs/webrtc-setup.md)
- WebRTC audio/video architecture: [`/docs/webrtc-audio-video-architecture.md`](./docs/webrtc-audio-video-architecture.md)

## 📁 Repository Structure

```text
.
├── application/   # Flutter mobile app
├── backend/       # Go API + Python face service + docker stack
├── docs/          # Architecture and protocol documentation
└── esp32/         # Firmware for smart door hardware
```
