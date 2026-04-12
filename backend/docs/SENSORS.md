# Smart Door Security - Sensor Reference

## Overview

The ESP32 reads sensors and publishes events over MQTT. The backend subscribes to those topics, updates runtime state, logs events to PostgreSQL, sends notifications, and publishes actuator commands back to the ESP32 when needed.

The current runtime splits responsibilities like this:

- `IntrusionFlow` owns vibration and magnetic intrusion rules.
- `VisitorAuthFlow` owns the PIR-triggered visitor authentication path.
- `UltrasonicService`, `ProximityService`, `DoorService`, and `MotorService` are lower-level sensor/actuator services.

---

## Sensors and Actuators

### 1. 801S Vibration Sensor

| Property | Value |
| --- | --- |
| GPIO | IO14 |
| Logic | Active HIGH on vibration |
| MQTT topic (publish) | `home/door/vibration` |
| Payload | `"INTRUSION"` |
| Backend flow | `IntrusionFlow.HandleVibrationDetected()` |
| Actions | May activate intrusion state, play SOS, log `FORCED_ENTRY`, and trigger a forced-entry call |
| Debounce (ESP32) | 3 s |
| Debounce (backend) | 10 s |

Purpose: Detects forceful vibration or tampering. During the authorization window, vibration is suppressed only while the motor has not yet reached the locked position.

---

### 2. MC-38 Magnetic Reed Switch

| Property | Value |
| --- | --- |
| GPIO | IO33 (`INPUT_PULLUP`) |
| Logic | HIGH = door open |
| MQTT topics | `home/door/magnetic/open` and `home/door/magnetic/closed` |
| Payload | None |
| Backend flow | `IntrusionFlow.HandleDoorOpened()` and `IntrusionFlow.HandleDoorClosed()` |
| Open behavior | Authorized open logs `DOOR_OPENED` and starts the left-open timer. Unauthorized open logs `FORCED_ENTRY`, plays SOS, and may trigger a forced-entry call |
| Close behavior | Logs `DOOR_CLOSED` and cancels any active left-open timer |
| Left-open timeout | 18 s |
| Left-open alert | After 18 s on an authorized open: plays SOS, logs `DOOR_LEFT_OPEN`, sends notification |
| Edge detection | Yes, ESP32 publishes on state change |

Purpose: Tracks door open/closed state and participates in forced-entry detection when the door opens outside the authorization window.

---

### 3. HC-SR501 PIR Motion Sensor

| Property | Value |
| --- | --- |
| GPIO | IO27 |
| Logic | Active HIGH on motion |
| MQTT topic (publish) | `home/door/pir` |
| Payload | `"DETECTED"` |
| Backend flow | `VisitorAuthFlow.HandleMotionDetected()` |
| Actions | If intrusion is inactive, no call is active, and ultrasonic confirms visitor-at-door: capture frame, run face recognition, then produce `AUTHORIZED_ENTRY`, `UNKNOWN_VISITOR`, or `SPOOF_ATTEMPT` |
| Debounce (ESP32) | 3 s |

Purpose: Starts the visitor authentication flow, but only after backend policy checks pass.

---

### 4. HC-SR04 Ultrasonic Sensor

| Property | Value |
| --- | --- |
| GPIO TRIG | IO23 |
| GPIO ECHO | IO22 |
| MQTT topic | `home/door/ultrasonic` |
| Payload | Distance in cm, for example `"85.5"` |
| Backend service | `UltrasonicService.HandleDistance()` |
| Current backend use | Stores the latest distance reading and reports `IsAtDoor()` when the last value is below `20 cm` |
| Read interval (ESP32) | Every 2 s |

Purpose: Maintains the latest distance-to-door reading. `VisitorAuthFlow` uses it as a gate before face recognition.

Note: The current backend does not implement the older multi-tier distance behavior described in previous docs. It only uses the `< 20 cm` at-door threshold.

---

### 5. IR Proximity Sensor

| Property | Value |
| --- | --- |
| MQTT topic | `home/door/proximity` |
| Payload | `"DETECTED"` |
| Backend service | `ProximityService.HandleProximityDetected()` |
| Current backend action | Publishes `home/door/proximity_alert` with `VISITOR_NEAR` |

Purpose: Drives local door-area feedback on the ESP32 side. It does not currently log backend events or trigger visitor authentication on its own.

---

### 6. SG90 Servo Motor

| Property | Value |
| --- | --- |
| GPIO | IO32 (PWM) |
| MQTT topic (subscribe) | `home/door/servo` |
| Payload | `"UNLOCK"` or `"LOCK"` |
| Backend service | `DoorService.UnlockDoor()` and `DoorService.LockDoor()` |
| Auto-lock delay | 15 s |
| Authorization window | 15 s, tracked through `SecurityStateService` |

Purpose: Physically locks and unlocks the door.

---

### 7. Motor Angle Feedback

| Property | Value |
| --- | --- |
| MQTT topic (publish) | `home/door/motor` |
| Payload | Current motor angle, for example `"0"` or `"55"` |
| Backend service | `MotorService.HandleMotorReading()` |
| Tolerance | 5 degrees |
| Current behavior | Updates latest motor angle and logs mismatches only |

Purpose: Feeds the latest motor angle back to the backend. The current runtime does not raise `MOTOR_TAMPER` alarms from motor mismatch; it only logs deviation.

---

### 8. Door Camera

| Property | Value |
| --- | --- |
| Interface | USB webcam |
| Backend context | `webrtc.DoorPeer` and `VisitorAuthFlow` |
| Streaming logic | WebRTC video stream to the mobile app |
| Snapshot logic | Triggered from the PIR path through `VisitorAuthFlow` and `FaceService` |
| Events produced | `AUTHORIZED_ENTRY`, `UNKNOWN_VISITOR`, `SPOOF_ATTEMPT` |

Purpose: Supports live door video and snapshot-based face recognition.

---

### 9. Door Microphone and Speaker

| Property | Value |
| --- | --- |
| Interface | USB / audio jack |
| Backend context | `webrtc.DoorPeer` |

Purpose: Provides two-way audio during WebRTC calls between the door device and the owner's app.

---

### 10. Doorbell / Smart Button

| Property | Value |
| --- | --- |
| GPIO | IO0 |
| Status | Present in hardware, currently unused by backend runtime |

Purpose: Reserved for future use.

---

## Event Types

| Constant | String | Current runtime producer |
| --- | --- | --- |
| `EventAuthorizedEntry` | `AUTHORIZED_ENTRY` | `VisitorAuthFlow` on recognized face |
| `EventUnknownVisitor` | `UNKNOWN_VISITOR` | `VisitorAuthFlow` on unrecognized face |
| `EventForcedEntry` | `FORCED_ENTRY` | `IntrusionFlow` on vibration or unauthorized magnetic open |
| `EventIntrusionCleared` | `INTRUSION_CLEARED` | Manual clear path or forced-entry call decline after intrusion is cleared |
| `EventManualUnlock` | `MANUAL_UNLOCK` | `POST /door/unlock` |
| `EventManualLock` | `MANUAL_LOCK` | `POST /door/lock` and current auto-lock path |
| `EventSpoofAttempt` | `SPOOF_ATTEMPT` | `VisitorAuthFlow` on liveness failure |
| `EventDoorOpened` | `DOOR_OPENED` | `IntrusionFlow` on authorized magnetic open |
| `EventDoorClosed` | `DOOR_CLOSED` | `IntrusionFlow` on magnetic close |
| `EventDoorLeftOpen` | `DOOR_LEFT_OPEN` | `IntrusionFlow` authorized-open timeout |
| `EventVisitorApproaching` | `VISITOR_APPROACHING` | `VisitorAuthFlow` after PIR plus ultrasonic-at-door confirmation |
| `EventHandleTamper` | `HANDLE_TAMPER` | Defined constant, not emitted by current runtime |
| `EventMotorTamper` | `MOTOR_TAMPER` | Defined constant, not emitted by current runtime |

---

## API Endpoints

All endpoints require `Authorization: Bearer <jwt>` except `/health`.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/health` | Server health check |
| `POST` | `/door/unlock` | Manually unlock door, clear active intrusion state, and auto-lock after 15 s |
| `POST` | `/door/lock` | Manually lock door and clear active intrusion state |
| `POST` | `/security/clear-intrusion` | Explicitly clear active intrusion state without changing the door state |
| `GET` | `/door/state` | Get current expected servo state and angle |
| `GET` | `/events` | List recent events |
| `GET` | `/events/:id` | Get a single event by ID |

---

## MQTT Runtime Diagram

```text
ESP32 Sensor/Event              Backend Flow or Service         Result
---------------------           -----------------------         ----------------------------
home/door/pir                  -> VisitorAuthFlow             -> face auth / unlock / call
home/door/vibration            -> IntrusionFlow              -> forced-entry alert / call
home/door/magnetic/open        -> IntrusionFlow              -> door-open or forced-entry path
home/door/magnetic/closed      -> IntrusionFlow              -> door-closed path
home/door/ultrasonic           -> UltrasonicService          -> update latest distance
home/door/proximity            -> ProximityService           -> publish proximity_alert
POST /door/unlock              -> DoorService                -> publish UNLOCK
POST /door/lock                -> DoorService                -> publish LOCK
home/door/motor                -> MotorService               -> update angle / log mismatch
```
