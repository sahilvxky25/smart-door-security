# WebSocket Architecture

The WebSocket implementation in this project acts as the central **Signaling Hub & Event Router** for the smart door system. It is implemented in Go under `internal/webrtc/signalling.go` using the `gorilla/websocket` library.

The pipeline is designed to handle two critical real-time functions over a single connection: **WebRTC connection brokering** (for live VoIP calls) and **Real-time Dashboard Notifications** (for pushing security alerts).

## Pipeline Step-by-Step

### 1. Hub Initialization
At application startup, a central memory space called the `Hub` is created (`webrtc.NewHub()`) and started in the background. The `Hub` tracks:
- All active WebSocket `Client` connections mapped by their `role` (either `"owner"` or `"door"`).
- The `user_id` of the currently connected homeowner.
- Go Channels (`localDoorRecv`) to route messages to internal server processes rather than physical hardware. 

### 2. Client Connection & Upgrade
When the Flutter app (the `"owner"`) connects to the backend:
1. **The Request:** The app sends an HTTP GET request to the WebSocket route (`/ws`), passing identification parameters in the URL (e.g., `?role=owner&user_id=1`).
2. **The Upgrade:** The `HandleWebSocket` controller intercepts it and calls `websocket.Upgrader.Upgrade()`. This changes the network protocol from standard HTTP to a persistent, full-duplex TCP WebSocket connection.
3. **The Registration:** A `Client` struct is created, tracking the target user and mapping its `send` capabilities. A lock is acquired to safely register the new user inside the `Hub`.
4. **The Pumps:** For every connected user, two concurrent background loops ("goroutines") are immediately spawned to handle traffic:
   - **`readPump()`:** Blocks indefinitely, listening for any upstream data coming from the mobile app.
   - **`writePump()`:** Listens to the client's local memory channel and writes data down the physical network socket to the mobile app. It also runs a 54-second "Ping" cadence to ensure the phone hasn't died or disconnected.

### 3. The Messaging Architecture 
Once the connection is "up", the pipeline serves its two primary purposes:

#### A. WebRTC Signaling (Bi-directional)
To establish a zero-latency video/audio connection between the homeowner's phone and the door, WebRTC needs to exchange connection handshakes ("SDP Offers/Answers").
* **The App speaks:** The Flutter app generates an SDP Offer and fires it up the WebSocket. 
* **The Backend routes:** The `readPump` catches the message. Observing that the sender is an `"owner"`, it determines the message must be routed to the `"door"`.
* **The Local Door Proxy:** Instead of sending this out to the ESP32, the `Hub` recognizes that a "local door peer" is running natively inside the Go backend (`RegisterLocalDoor`). The Go backend bridging the RTSP camera stream intercepts the signal directly via Go channels.
* **The Response:** The backend WebRTC streamer formulates an SDP Answer and pipes it into the Hub, which broadcasts it back out of the WebSocket to the `"owner"`. A direct peer-to-peer media connection is then formed.

#### B. Server-initiated Notifications (Uni-directional Push)
The WebSocket is the backbone that replaces API polling. When background hardware (like an ESP32 publishing over MQTT) triggers states in the Go backend, the backend forcibly writes messages down the WebSocket:
* **Dashboard State (`BroadcastEventUpdate`)**: If the magnetic sensor detects the door opening, it pings the Go backend, which immediately pushes `{type: "event_update", event_type: "door_opened"}` down the pipe to tell the UI to instantly refresh its status icon.
* **Security Feeds (`BroadcastAlert`)**: If the backend processes a spoofing attempt, it pumps an `"alert"` JSON frame down the websocket. The flutter websocket listener intercepts this frame and pops up an alert banner.
* **VoIP Calls (`BroadcastIncomingCall`)**: When face recognition fails but motion is high, the backend fires an `"incoming_call"` frame containing the snapshot URL, prompting the Flutter app to ring visually.

### 4. Teardown
1. If the homeowner minimizes the app, their phone loses cellular data, or they fail to respond to the 54-second backend ping, the physical connection times out. 
2. The `readPump()` loop catches the network EOF or standard closure error.
3. The `Client` is pushed into an `unregister` channel cleanly.
4. The `Hub` permanently removes them from the map, preventing the server from leaking memory or crashing by attempting to send alerts to a dead connection.
