# Pipeline Edge-Case Audit

Scope reviewed:
- Known user path: `PIR -> CameraService -> FaceService -> DoorService -> MagneticService`
- Unknown user path: `PIR -> CameraService -> NotificationService -> CallManager`
- Theft path: `VibrationService` and `MagneticService` forced-entry flows

## Fixed in code

1. Repeated unlocks could schedule overlapping auto-lock goroutines.
- `DoorService` now keeps a single resettable auto-lock timer.
- Old timers are invalidated so a newer authorized unlock is not cut short by an older timer.

2. Door-open theft detection could miss real forced-entry cases.
- `MagneticService` now treats any door open outside the auth window as forced entry.
- The old condition depended on motor angle and could suppress a real theft event when the lock still read `0`.

3. Theft flows could stack duplicate incoming calls across sensors.
- `CallManager` now exposes live-call lookup by event type.
- `VibrationService`, `MagneticService`, and `CameraService` suppress duplicate calls while an equivalent call is already `ringing` or `accepted`.

4. Unauthorized opens could still start the left-open timer.
- `MagneticService` now starts the `DOOR_LEFT_OPEN` timer only for authorized opens.
- This prevents follow-up "door left open" noise during a theft/forced-entry incident.

5. Left-open timeout drifted from the documented behavior.
- `leftOpenTimeout` is set to `30s` to match the sensor reference docs.

6. Hardware events could fail to persist when no owner WebSocket session was active.
- `EventService` now falls back to the first registered user when there is no active owner connection.
- This keeps unknown-visitor, theft, and door-state events from disappearing entirely when the app is offline.

## Remaining design risks

1. Event ownership fallback assumes a single-owner deployment.
- If multiple owner accounts need separate event histories, event attribution should be redesigned instead of using the first registered user as fallback.

2. Motor tamper still logs only.
- `MotorService` currently records angle mismatches in logs but does not raise an alert.
- That may be intentional to avoid false positives, but it means servo-only tamper is not a full theft alarm path right now.

3. Auto-lock is still logged as `MANUAL_LOCK`.
- The system has no distinct `AUTO_LOCK` event type yet, so auto-lock history is semantically imprecise.

