# Pipeline Edge-Case Audit

This file documents the current backend behavior after the flow refactor.

## Runtime Paths

- Known visitor path: `PIR -> VisitorAuthFlow -> FaceService -> DoorService`
- Unknown visitor path: `PIR -> VisitorAuthFlow -> NotificationService -> CallManager`
- Intrusion path: `Vibration or unauthorized magnetic open -> IntrusionFlow`

## Current Guarantees

1. Visitor authentication is blocked while intrusion is active.

- `SecurityStateService` owns the `intrusionActive` flag.
- `VisitorAuthFlow` exits early when intrusion is active.

2. Repeated unlocks do not stack auto-lock timers.

- `DoorService` keeps a single resettable timer.
- The current auto-lock delay is `15s`.

3. Unauthorized magnetic open is treated as forced entry.

- `IntrusionFlow` checks the shared authorization window.
- If the door opens outside that window, it activates intrusion, logs `FORCED_ENTRY`, plays SOS, and may trigger a forced-entry call.

4. Authorized magnetic open can still produce a left-open alert.

- `IntrusionFlow` starts a left-open timer only for authorized opens.
- The current left-open timeout is `18s`, i.e, `18s`-`15s`=`3s` is left to check whether intrusion service will activate or not.

5. Forced-entry calls are deduplicated.

- `IntrusionFlow` suppresses duplicate vibration-triggered calls while a forced-entry call is already live.
- `VisitorAuthFlow` suppresses duplicate unknown-visitor and spoof-attempt calls by event type.

6. Declined vibration calls re-arm future vibration incidents.

- If the previous forced-entry call from vibration was declined, the next vibration can still create a new call after debounce.

7. Manual owner actions clear intrusion state.

- `POST /door/unlock` clears intrusion before unlocking.
- `POST /door/lock` clears intrusion before locking.
- `POST /security/clear-intrusion` clears intrusion without changing the door state.(also sends /door/lock ping, fallback for now fix this later)

## Important Current Limitations

1. Motor tamper is log-only.

- `MotorService` updates the latest motor angle and logs mismatches.
- It does not currently emit `MOTOR_TAMPER`, play SOS, or trigger notifications.

2. Auto-lock still logs `MANUAL_LOCK`.

- The system does not have a distinct `AUTO_LOCK` event type yet.

3. Event constants are broader than runtime behavior.

- `HANDLE_TAMPER` and `MOTOR_TAMPER` exist as constants but are not currently produced by the runtime path. (will be used later ffor scaling this project)

4. Ultrasonic behavior is simpler than older docs implied.

- The backend stores the latest distance reading.
- `VisitorAuthFlow` only checks whether the last reading is below `20 cm`.
- The old multi-tier `VISITOR_APPROACHING` distance model is not active in current code.
