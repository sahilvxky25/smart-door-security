# Smart Door Security — Sensor Reference

## Overview

The ESP32 reads all sensors and publishes events via MQTT to the backend. The backend subscribes to each topic, applies debounce logic, logs events to PostgreSQL, sends push notifications, and optionally publishes commands back to the ESP32.

---

## Sensors & Actuators

### 1. 801S Vibration Sensor

| Property                 | Value                                                     |
| ------------------------ | --------------------------------------------------------- |
| **GPIO**                 | IO14                                                      |
| **Logic**                | Active HIGH on vibration                                  |
| **MQTT topic (publish)** | `home/door/vibration`                                     |
| **Payload**              | `"INTRUSION"`                                             |
| **Backend service**      | `VibrationService.HandleVibration()`                      |
| **Actions**              | Plays SOS alert · Logs `FORCED_ENTRY` · Push notification |
| **Debounce (ESP32)**     | 3 s                                                       |

**Purpose**: Detects forceful door knocks or tampering attempts. Any significant vibration is treated as a potential forced-entry attempt.

---

### 2. MC-38 Magnetic Reed Switch (Door Sensor)

| Property                 | Value                                                       |
| ------------------------ | ----------------------------------------------------------- |
| **GPIO**                 | IO33 (INPUT_PULLUP)                                         |
| **Logic**                | HIGH = door open (magnet removed)                           |
| **MQTT topics**          | `home/door/magnetic/open` \| `home/door/magnetic/closed`    |
| **Payload**              | _None (Empty)_                                              |
| **Backend service**      | `MagneticService.HandleDoorOpen()` / `HandleDoorClose()`    |
| **Actions (OPEN)**       | Logs `DOOR_OPENED` · Starts 30 s left-open timer            |
| **Actions (CLOSED)**     | Logs `DOOR_CLOSED` · Cancels left-open timer                |
| **Left-open alert**      | After 30 s open: SOS + logs `DOOR_LEFT_OPEN` + notification |
| **Edge detection**       | Yes — only publishes on state change                        |

**Purpose**: Tracks whether the physical door is open or closed. Raises an alert if the door is left open for more than 30 seconds.

---

### 3. HC-SR501 PIR Motion Sensor

| Property                 | Value                                                                                         |
| ------------------------ | --------------------------------------------------------------------------------------------- |
| **GPIO**                 | IO27                                                                                          |
| **Logic**                | Active HIGH on motion                                                                         |
| **MQTT topic (publish)** | `home/door/pir`                                                                               |
| **Payload**              | `"DETECTED"`                                                                                  |
| **Backend service**      | `CameraService.HandleMotion()`                                                                |
| **Actions**              | Captures camera frame → face recognition → AUTHORIZED_ENTRY / UNKNOWN_VISITOR / SPOOF_ATTEMPT |
| **Debounce (ESP32)**     | 3 s                                                                                           |

**Purpose**: Primary motion detector. Triggers the face recognition pipeline whenever someone moves near the door.

---

### 4. Ultrasonic Sensor (HC-SR04)

| Property                       | Value                                                                                                                                |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------ |
| **GPIO TRIG**                  | IO23                                                                                                                                 |
| **GPIO ECHO**                  | IO22                                                                                                                                 |
| **MQTT topic — raw distance**  | `home/door/ultrasonic`                                                                                                               |
| **Payload**                    | Distance in cm, e.g. `"85.5"`                                                                                                        |
| **MQTT topic — proximity**     | `home/door/proximity`                                                                                                                |
| **Payload**                    | `"DETECTED"` (when < 50 cm)                                                                                                          |
| **Backend service**            | `UltrasonicService.HandleDistance()`                                                                                                 |
| **Distance tiers**             | < 80 cm → triggers face pipeline · 80–200 cm → logs `VISITOR_APPROACHING` · > 200 cm → no action                                     |
| **Proximity service**          | `ProximityService.HandleProximityDetected()` → publishes `home/door/proximity_alert VISITOR_NEAR` → ESP32 flashes LED (evening only) |
| **Read interval (ESP32)**      | Every 2 s                                                                                                                            |
| **Debounce (ESP32 proximity)** | 5 s                                                                                                                                  |
| **Debounce (backend)**         | 10 s                                                                                                                                 |

**Purpose**: Secondary presence detection. Provides distance data for tiered response (approaching vs. at-door) and drives the evening proximity LED indicator.

---

### 5. SG90 Servo Motor (Door Lock Actuator)

| Property                   | Value                                                                                                                                  |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| **GPIO**                   | IO32 (PWM)                                                                                                                             |
| **MQTT topic (subscribe)** | `home/door/servo`                                                                                                                      |
| **Payload**                | `"UNLOCK"` → 90° \| `"LOCK"` → 0°                                                                                                      |
| **Backend service**        | `DoorService.UnlockDoor()` / `LockDoor()`                                                                                              |
| **Auto-lock**              | 5 s after unlock                                                                                                                       |
| **Tamper detection**       | `MotorService` monitors `home/door/motor`: if reported angle deviates > 15° from commanded angle → SOS + `MOTOR_TAMPER` + notification |

**Purpose**: Physically locks and unlocks the door. Paired with MotorService to detect if the servo is moved physically without a backend command.

---

### 6. Door Camera (USB/Webcam)

| Property                 | Value                                                              |
| ------------------------ | ------------------------------------------------------------------ |
| **Interface**            | USB (accessed via FFmpeg `dshow` / `v4l2`)                         |
| **Backend context**      | `webrtc.DoorPeer` / `CameraService`                                |
| **Streaming logic**      | WebRTC VP8 video sent directly to Flutter app                      |
| **Snapshot logic**       | Triggered by ESP32 `home/door/pir` → posts to Face Service         |
| **Events produced**      | `AUTHORIZED_ENTRY` \| `UNKNOWN_VISITOR` \| `SPOOF_ATTEMPT`         |

**Purpose**: Provides live streaming during a WebRTC call, and captures snapshots for facial recognition and liveness detection when motion is detected by the PIR sensor.

---

### 7. Door Microphone & Speaker (Two-Way Audio)

| Property                 | Value                                                              |
| ------------------------ | ------------------------------------------------------------------ |
| **Interface**            | USB/Audio Jack (accessed via FFmpeg/`ffplay`)                      |
| **Backend context**      | `webrtc.DoorPeer`                                                  |
| **Laptop → Phone**       | Microphone → FFmpeg (`-f rtp`) → UDP Port → `pionwebrtc`           |
| **Phone → Laptop**       | App → WebRTC RTP → `pionwebrtc.oggwriter` → FFplay (`-f ogg`)      |

**Purpose**: Enables real-time, two-way WebRTC audio communication between the door (Host PC) and the owner's mobile app.

**Implementation Details (`door_peer.go`)**:
- **`detectDShowDevices()`**: Auto-discovers available physical audio/video hardware using `ffmpeg -list_devices`, parsing both legacy and modern log outputs. Uses `.env` vars (`DOOR_AUDIO_DEVICE`) as an exact match target.
- **`buildAudioArgs()`**: Configures FFmpeg to capture audio, encode to `libopus` at 48000Hz, and package it natively into standard Opus RTP packets (`-f rtp rtp://127.0.0.1:<port>`). 
- **`pumpAudioRTP()`**: Bridges the Laptop → Phone audio gap. Opens a local UDP listener, catches the raw RTP packets generated by FFmpeg, and writes them directly to Pion's `TrackLocalStaticRTP`. Bypassing an intermediate `.ogg` container prevents multi-frame RTP packetization errors.
- **`playRemoteAudio()`**: Bridges the Phone → Laptop audio gap. Receives RTP packets from the Flutter app, safely wraps them into an OGG container stream using Pion's `oggwriter`, and pipes it into an invisible `ffplay` subprocess to play through the local speakers.

---

### 8. Doorbell / Smart Button (IO0_BTN)

| Property                 | Value                                                              |
| ------------------------ | ------------------------------------------------------------------ |
| **GPIO**                 | IO0                                                                |
| **Status**               | Present in physical circuit, but currently **unhandled** (reserved)|

**Purpose**: Intended to be used as a smart doorbell or manual unlock button. Currently unhandled in firmware because it operates on the ESP32 boot strap pin (`IO0`) and requires careful handling.

---

## Event Types Reference

| Constant                  | String                | Produced By                      |
| ------------------------- | --------------------- | -------------------------------- |
| `EventAuthorizedEntry`    | `AUTHORIZED_ENTRY`    | Face recognition (known face)    |
| `EventUnknownVisitor`     | `UNKNOWN_VISITOR`     | Face recognition (no match)      |
| `EventForcedEntry`        | `FORCED_ENTRY`        | Vibration sensor                 |
| `EventManualUnlock`       | `MANUAL_UNLOCK`       | POST /door/unlock                |
| `EventManualLock`         | `MANUAL_LOCK`         | POST /door/lock                  |
| `EventSpoofAttempt`       | `SPOOF_ATTEMPT`       | Face recognition (liveness fail) |
| `EventDoorOpened`         | `DOOR_OPENED`         | Magnetic sensor                  |
| `EventDoorClosed`         | `DOOR_CLOSED`         | Magnetic sensor                  |
| `EventDoorLeftOpen`       | `DOOR_LEFT_OPEN`      | Magnetic sensor (30 s timer)     |
| `EventVisitorApproaching` | `VISITOR_APPROACHING` | Ultrasonic / Proximity           |
| `EventMotorTamper`        | `MOTOR_TAMPER`        | Motor tamper detection           |

---

## API Endpoints (Sensor-Related)

All endpoints require `Authorization: Bearer <jwt>` except `/health`.

| Method | Path           | Description                                                 |
| ------ | -------------- | ----------------------------------------------------------- |
| `GET`  | `/health`      | Server health check                                         |
| `POST` | `/door/unlock` | Manually unlock door (→ servo UNLOCK, auto-locks after 5 s) |
| `POST` | `/door/lock`   | Manually lock door (→ servo LOCK)                           |
| `GET`  | `/door/state`  | Get current servo state (`LOCKED` / `UNLOCKED`) and angle   |
| `GET`  | `/events`      | List recent events (all sensor-triggered + manual)          |
| `GET`  | `/events/:id`  | Get a single event by ID                                    |

### Swagger UI

Visit [`http://localhost:8080/swagger/index.html`](http://localhost:8080/swagger/index.html) to explore all endpoints interactively. Click **Authorize** and enter `Bearer <your-jwt>` to test protected routes.

---

## MQTT Flow Diagram

```
ESP32 Sensors                 Backend Services              Flutter App
─────────────                 ────────────────              ───────────
PIR          →  pir         → CameraService         →  push notification
Vibration    →  vibration   → VibrationService       →  push notification
Magnetic     →  magnetic/open/closed  → MagneticService    →  WebSocket event
Ultrasonic   →  ultrasonic  → UltrasonicService      →  push notification
             →  proximity   → ProximityService
                            → (publishes proximity_alert)
Servo        ←  servo       ← DoorService            ←  POST /door/unlock
Motor        →  motor       → MotorService            →  push notification
```
