package webrtc

// This file is intentionally minimal.
//
// WebRTC peer connections (RTCPeerConnection) are established directly
// between the door-side browser (web/door.html) and the owner-side
// browser (web/owner.html). The Go backend acts only as a signaling
// relay via WebSocket — it never touches media streams.
//
// See web/door.html and web/owner.html for the browser-side WebRTC logic.
