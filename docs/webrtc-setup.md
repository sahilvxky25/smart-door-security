# WebRTC Setup Guide

This document explains how the WebRTC connection is established between the **Go backend (DoorPeer)** and the **Flutter mobile app (Owner)**.

## Overview

The system uses a custom WebRTC signaling protocol over WebSocket to establish peer-to-peer connections between the door hardware (running as an in-process Go peer) and the owner's Flutter app.

```
┌──────────────┐     WebSocket      ┌──────────────────┐     WebSocket      ┌──────────────┐
│  Flutter App │ ◄──(signaling)──►  │   Signaling Hub  │ ◄──(signaling)──►  │   DoorPeer   │
│  (Owner)     │                    │   (Go backend)   │                    │ (Go in-proc) │
│              │ ◄──WebRTC P2P────► │                  │ ◄──WebRTC P2P────► │              │
└──────────────┘                    └──────────────────┘                    └──────────────┘
```

## Components

### 1. Signaling Hub (`signalling.go`)

- Runs as a WebSocket server at `/ws/signaling?role=<door|owner>`
- Relays JSON signaling messages between `door` and `owner` roles
- Supports both WebSocket clients (external door hardware) and an in-process local door peer
- Handles registration, unregistration, and message broadcasting

### 2. DoorPeer (`door_peer.go`)

- An in-process WebRTC peer acting as the door-side endpoint
- Uses **FFmpeg** subprocesses for media capture (no CGo required)
- Captures VP8 video from camera and Opus audio from microphone
- Receives the owner's audio and plays it through speakers via **ffplay**

### 3. Flutter WebRTCService (`webrtc_service.dart`)

- Manages the `RTCPeerConnection` on the mobile app side
- Acquires the owner's microphone for 2-way audio
- Receives and renders the door's video stream

### 4. Flutter CallProvider (`call_provider.dart`)

- State machine managing call lifecycle: `idle → ringing → requesting → connecting → inCall`
- Bridges between the signaling layer and the WebRTC media layer

## Signaling Flow

### Call Initiation (Owner starts call)

```
Owner App                Signaling Hub              DoorPeer
   │                          │                         │
   │── call_request ─────────►│── call_request ────────►│
   │                          │                         │── Setup PeerConnection
   │                          │                         │── Start FFmpeg (video+audio)
   │                          │                         │── Create SDP Offer
   │◄── offer ────────────────│◄── offer ───────────────│
   │── Set remote desc        │                         │
   │── Get mic permission     │                         │
   │── Create SDP Answer      │                         │
   │── answer ───────────────►│── answer ──────────────►│── Set remote desc
   │                          │                         │
   │◄─► ICE candidates ◄────► │◄─► ICE candidates ◄───► │
   │                          │                         │
   │════════ WebRTC P2P Media ═══════════════════════════│
```

### Incoming Call (Door detects visitor)

```
Camera Pipeline            Signaling Hub              Owner App
   │                          │                         │
   │── incoming_call ────────►│── incoming_call ───────►│── Show accept/decline UI
   │                          │                         │
   │                          │◄── call_accepted ───────│
   │◄── call_accepted ────────│                         │
   │                          │                         │
   │── (same flow as above from DoorPeer setup) ───────►│
```

## ICE Configuration

Both sides use the same STUN server for NAT traversal:

```
stun:stun.l.google.com:19302
```

For local network calls (same WiFi), ICE candidates resolve to local IPs directly.

## Prerequisites

- **FFmpeg** and **ffplay** must be installed and accessible
  - Windows: `C:\ffmpeg\bin\` or on PATH
  - Linux: Install via package manager (`apt install ffmpeg`)
- A camera and microphone connected to the door device
- The `DOOR_VIDEO_DEVICE` and `DOOR_AUDIO_DEVICE` environment variables can override auto-detection
