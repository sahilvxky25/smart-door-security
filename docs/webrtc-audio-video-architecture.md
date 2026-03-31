# WebRTC Audio/Video Architecture

This document details the **1-way video + 2-way audio** architecture used for door-to-owner calls.

## Media Flow Diagram

```
                        ┌─────────────────────────────────────┐
                        │           DoorPeer (Go)             │
                        │                                     │
  Camera ──► FFmpeg ──► │ VP8 IVF ──► pion video track ─────────────► Owner sees door video
  (dshow/v4l2)          │                                     │
                        │                                     │
  Microphone ──► FFmpeg │ Opus OGG ──► pion audio track ────────────► Owner hears door audio
  (dshow/pulse)         │                                     │
                        │                                     │
                        │ pion OnTrack ◄──────────────────────────── Owner sends mic audio
                        │   ↓                                 │
                        │ RTP → OGG wrapper → ffplay ──► Speaker │
                        └─────────────────────────────────────┘

                        ┌─────────────────────────────────────┐
                        │        Flutter App (Owner)          │
                        │                                     │
                        │ RTCPeerConnection                   │
                        │   onTrack → remoteStream ──► Video  │
                        │   onTrack → remoteStream ──► Audio  │
                        │                                     │
                        │ getUserMedia({audio: true})          │
                        │   ↓                                 │
                        │ localStream → addTrack ──► DoorPeer │
                        └─────────────────────────────────────┘
```

## Track Summary

| Direction | Media | Codec | Container | Source | Destination |
|-----------|-------|-------|-----------|--------|-------------|
| Door → Owner | Video | VP8 | IVF (pipe) | FFmpeg camera capture | Flutter RTCVideoRenderer |
| Door → Owner | Audio | Opus | OGG (pipe) | FFmpeg mic capture | Flutter audio playback |
| Owner → Door | Audio | Opus | RTP (WebRTC) | Flutter getUserMedia | ffplay via OGG pipe |

## Door-Side Media Pipeline

### Video Capture (Door → Owner)
1. FFmpeg captures from camera (`dshow` on Windows, `v4l2` on Linux)
2. Encodes to VP8 with realtime settings, outputs IVF container to stdout
3. `pumpIVF()` reads IVF frames and writes them as `media.Sample` to pion's video track
4. pion transmits VP8 frames over RTP to the owner

### Audio Capture (Door → Owner)
1. FFmpeg captures from microphone
2. Encodes to Opus, outputs OGG container to stdout
3. `pumpOGG()` reads OGG pages and writes them as `media.Sample` to pion's audio track
4. pion transmits Opus frames over RTP to the owner

### Audio Playback (Owner → Door)
1. pion's `OnTrack` callback fires when the owner's audio track arrives
2. `playRemoteAudio()` reads raw RTP packets from the track
3. Each RTP packet is parsed using `pion/rtp` to extract the Opus payload
4. An `oggPipeWriter` wraps each Opus payload in valid OGG pages (with proper OpusHead/OpusTags headers)
5. The OGG stream is piped to `ffplay` stdin for real-time speaker playback

### Why OGG Wrapping is Required
FFmpeg/ffplay **cannot** decode raw Opus frames without a container. Opus is a packet-based codec, and individual packets need framing information (timing, sample rate, channel count) that only exists in the OGG or WebM container headers. Without the OGG wrapper, ffplay exits immediately, closing the pipe.

## Owner-Side (Flutter) Media Pipeline

### Receiving Door Media
1. `createPeerConnection()` registers an `onTrack` callback
2. When the door's video/audio tracks arrive, the callback emits the `MediaStream`
3. The `RTCVideoRenderer` widget renders the video
4. Audio plays automatically through the device speaker

### Sending Microphone Audio
1. `handleOffer()` calls `getUserMedia({audio: true, video: false})`
2. The local audio track is added to the PeerConnection via `addTrack()`
3. An SDP answer is created and sent back to the DoorPeer
4. pion receives the audio track via `OnTrack` on the Go side

## FFmpeg/ffplay Configuration

### Video Capture Args
```
ffmpeg -f dshow -video_size 640x480 -framerate 30 -i "video=<device>"
       -c:v libvpx -b:v 500000 -cpu-used 5 -deadline realtime
       -error-resilient 1 -auto-alt-ref 0 -f ivf pipe:1
```

### Audio Capture Args
```
ffmpeg -f dshow -i "audio=<device>"
       -c:a libopus -ar 48000 -ac 1 -b:a 64000 -f ogg pipe:1
```

### Audio Playback Args
```
ffplay -nodisp -autoexit -loglevel error -f ogg -i pipe:0
```

## Error Handling

- If no camera is detected, the call setup aborts with a log message
- If no microphone is available, the call proceeds as video-only
- If the owner's mic permission is denied, the call is receive-only (owner sees video, hears door, but can't speak)
- ICE connection failures trigger automatic cleanup of all FFmpeg processes
