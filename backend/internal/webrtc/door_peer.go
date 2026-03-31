package webrtc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	pionwebrtc "github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
)

// signalingMsg is the JSON envelope for all signaling messages.
// Must match the Flutter app's SignalingMessage schema exactly.
type signalingMsg struct {
	Type      string      `json:"type"`
	SDP       *sdpPayload `json:"sdp,omitempty"`
	Candidate *icePayload `json:"candidate,omitempty"`
}

type sdpPayload struct {
	Type string `json:"type"` // "offer" or "answer"
	SDP  string `json:"sdp"`
}

type icePayload struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid"`
	SDPMLineIndex int    `json:"sdpMLineIndex"`
}

// DefaultICEServers matches the Flutter app's AppConfig.iceServers.
var DefaultICEServers = []pionwebrtc.ICEServer{
	{URLs: []string{"stun:stun.l.google.com:19302"}},
}

// DoorPeer is an in-process WebRTC peer that acts as the door-side endpoint.
// It captures via FFmpeg subprocesses (no CGo required) and streams
// VP8 video + Opus audio to the owner's Flutter app through pion/webrtc.
type DoorPeer struct {
	recv   <-chan []byte
	sendFn func([]byte)

	mu          sync.Mutex
	pc          *pionwebrtc.PeerConnection
	ffmpegProcs []*exec.Cmd

	iceServers []pionwebrtc.ICEServer
}

// IsCallActive returns true if the DoorPeer is currently in an active WebRTC session.
func (d *DoorPeer) IsCallActive() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pc != nil
}

func NewDoorPeer(recv <-chan []byte, sendFn func([]byte), iceServers []pionwebrtc.ICEServer) *DoorPeer {
	if len(iceServers) == 0 {
		iceServers = DefaultICEServers
	}
	return &DoorPeer{
		recv:       recv,
		sendFn:     sendFn,
		iceServers: iceServers,
	}
}

// Run blocks and processes messages from the Hub until the recv channel is closed.
func (d *DoorPeer) Run() {
	log.Println("[DoorPeer] Started, waiting for call requests...")
	for raw := range d.recv {
		d.handleMessage(raw)
	}
	log.Println("[DoorPeer] Recv channel closed, shutting down")
	d.mu.Lock()
	d.cleanupLocked()
	d.mu.Unlock()
}

func (d *DoorPeer) handleMessage(raw []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(raw, &msg); err != nil {
		log.Printf("[DoorPeer] Failed to unmarshal message: %v", err)
		return
	}

	msgType, _ := msg["type"].(string)
	log.Printf("[DoorPeer] Received: %s", msgType)

	switch msgType {
	case "call_request", "call_accepted":
		d.handleCallAccepted()
	case "call_declined":
		log.Println("[DoorPeer] Call declined by owner")
	case "answer":
		d.handleAnswer(msg)
	case "ice-candidate":
		d.handleIceCandidate(msg)
	case "hangup":
		d.handleHangup()
	}
}

func (d *DoorPeer) handleCallAccepted() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Clean up any existing call (handles rapid re-call)
	d.cleanupLocked()

	log.Println("[DoorPeer] Call accepted by owner, setting up media...")

	// 1. Detect camera and microphone devices
	videoDevice, audioDevice := detectDevices()
	if videoDevice == "" {
		log.Println("[DoorPeer] No video device found, cannot start call")
		return
	}
	log.Printf("[DoorPeer] Devices: video=%q audio=%q", videoDevice, audioDevice)

	// 2. Create PeerConnection
	pc, err := pionwebrtc.NewPeerConnection(pionwebrtc.Configuration{
		ICEServers: d.iceServers,
	})
	if err != nil {
		log.Printf("[DoorPeer] Failed to create PeerConnection: %v", err)
		return
	}
	d.pc = pc

	// 3. Create and add video track (VP8)
	videoTrack, err := pionwebrtc.NewTrackLocalStaticSample(
		pionwebrtc.RTPCodecCapability{MimeType: pionwebrtc.MimeTypeVP8},
		"video", "door-camera",
	)
	if err != nil {
		log.Printf("[DoorPeer] Failed to create video track: %v", err)
		d.cleanupLocked()
		return
	}
	if _, err := pc.AddTrack(videoTrack); err != nil {
		log.Printf("[DoorPeer] Failed to add video track: %v", err)
		d.cleanupLocked()
		return
	}

	// 4. Start FFmpeg for video capture → VP8 → IVF pipe
	videoCmd := exec.Command(ffmpegBin(), buildVideoArgs(videoDevice)...)
	videoCmd.Stderr = os.Stderr // pipe FFmpeg logs to console for debugging
	videoPipe, err := videoCmd.StdoutPipe()
	if err != nil {
		log.Printf("[DoorPeer] Failed to create video pipe: %v", err)
		d.cleanupLocked()
		return
	}
	if err := videoCmd.Start(); err != nil {
		log.Printf("[DoorPeer] Failed to start video FFmpeg: %v", err)
		d.cleanupLocked()
		return
	}
	d.ffmpegProcs = append(d.ffmpegProcs, videoCmd)
	go d.pumpIVF(videoPipe, videoTrack)

	// 5. Audio track (optional — continues video-only if mic unavailable)
	if audioDevice != "" {
		// Open a UDP listener for incoming RTP from FFmpeg
		udpAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
		udpListener, err := net.ListenUDP("udp", udpAddr)
		if err == nil {
			localPort := udpListener.LocalAddr().(*net.UDPAddr).Port

			audioTrack, err := pionwebrtc.NewTrackLocalStaticRTP(
				pionwebrtc.RTPCodecCapability{MimeType: pionwebrtc.MimeTypeOpus},
				"audio", "door-mic",
			)
			if err == nil {
				if _, addErr := pc.AddTrack(audioTrack); addErr == nil {
					// Audio command now writes to UDP instead of stdout pipe
					audioCmd := exec.Command(ffmpegBin(), buildAudioArgs(audioDevice, localPort)...)
					audioCmd.Stderr = os.Stderr // pipe FFmpeg audio logs to console
					
					if startErr := audioCmd.Start(); startErr == nil {
						d.ffmpegProcs = append(d.ffmpegProcs, audioCmd)
						go d.pumpAudioRTP(udpListener, audioTrack)
					} else {
						log.Printf("[DoorPeer] Audio FFmpeg failed to start (video-only): %v", startErr)
						udpListener.Close()
					}
				} else {
					udpListener.Close()
				}
			} else {
				udpListener.Close()
			}
		} else {
			log.Printf("[DoorPeer] Failed to open UDP port for audio RTP: %v", err)
		}
	}

	// 6. Receive owner's audio track (2-way audio)
	pc.OnTrack(func(track *pionwebrtc.TrackRemote, receiver *pionwebrtc.RTPReceiver) {
		log.Printf("[DoorPeer] Received remote track: kind=%s codec=%s", track.Kind().String(), track.Codec().MimeType)
		if track.Kind() != pionwebrtc.RTPCodecTypeAudio {
			log.Printf("[DoorPeer] Ignoring non-audio remote track")
			return
		}
		// Play owner's audio through system speaker via FFmpeg
		go d.playRemoteAudio(track)
	})

	// 7. ICE candidate handler — send each candidate to the owner
	pc.OnICECandidate(func(candidate *pionwebrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		init := candidate.ToJSON()
		sdpMid := ""
		if init.SDPMid != nil {
			sdpMid = *init.SDPMid
		}
		sdpMLineIndex := 0
		if init.SDPMLineIndex != nil {
			sdpMLineIndex = int(*init.SDPMLineIndex)
		}
		d.sendJSON(signalingMsg{
			Type: "ice-candidate",
			Candidate: &icePayload{
				Candidate:     init.Candidate,
				SDPMid:        sdpMid,
				SDPMLineIndex: sdpMLineIndex,
			},
		})
	})

	// 8. ICE connection state handler
	pc.OnICEConnectionStateChange(func(state pionwebrtc.ICEConnectionState) {
		log.Printf("[DoorPeer] ICE connection state: %s", state.String())
		switch state {
		case pionwebrtc.ICEConnectionStateConnected:
			log.Println("[DoorPeer] Call connected, streaming 2-way audio + 1-way video")
		case pionwebrtc.ICEConnectionStateDisconnected,
			pionwebrtc.ICEConnectionStateFailed:
			d.sendJSON(signalingMsg{Type: "hangup"})
			go func() {
				d.mu.Lock()
				defer d.mu.Unlock()
				d.cleanupLocked()
			}()
		}
	})

	// 8. Create SDP offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		log.Printf("[DoorPeer] Failed to create offer: %v", err)
		d.cleanupLocked()
		return
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		log.Printf("[DoorPeer] Failed to set local description: %v", err)
		d.cleanupLocked()
		return
	}

	// 9. Send offer to owner
	d.sendJSON(signalingMsg{
		Type: "offer",
		SDP: &sdpPayload{
			Type: offer.Type.String(),
			SDP:  offer.SDP,
		},
	})

	log.Println("[DoorPeer] Offer sent, waiting for answer...")
}

func (d *DoorPeer) handleAnswer(msg map[string]interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.pc == nil {
		log.Println("[DoorPeer] Received answer but no PeerConnection exists, ignoring")
		return
	}

	sdpMap, ok := msg["sdp"].(map[string]interface{})
	if !ok {
		log.Println("[DoorPeer] Invalid answer message: missing 'sdp' object")
		return
	}

	sdpType, _ := sdpMap["type"].(string)
	sdpStr, _ := sdpMap["sdp"].(string)

	err := d.pc.SetRemoteDescription(pionwebrtc.SessionDescription{
		Type: pionwebrtc.NewSDPType(sdpType),
		SDP:  sdpStr,
	})
	if err != nil {
		log.Printf("[DoorPeer] Failed to set remote description: %v", err)
		return
	}

	log.Println("[DoorPeer] Remote description set (answer)")
}

func (d *DoorPeer) handleIceCandidate(msg map[string]interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.pc == nil {
		return
	}

	candMap, ok := msg["candidate"].(map[string]interface{})
	if !ok {
		return
	}

	candidate, _ := candMap["candidate"].(string)
	sdpMid, _ := candMap["sdpMid"].(string)
	sdpMLineIndexF, _ := candMap["sdpMLineIndex"].(float64)
	sdpMLineIndex := uint16(sdpMLineIndexF)

	err := d.pc.AddICECandidate(pionwebrtc.ICECandidateInit{
		Candidate:     candidate,
		SDPMid:        &sdpMid,
		SDPMLineIndex: &sdpMLineIndex,
	})
	if err != nil {
		log.Printf("[DoorPeer] Failed to add ICE candidate: %v", err)
	}
}

func (d *DoorPeer) handleHangup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	log.Println("[DoorPeer] Hangup received, cleaning up")
	d.cleanupLocked()
}

// cleanupLocked kills FFmpeg subprocesses and closes the PeerConnection.
// MUST be called with d.mu held.
func (d *DoorPeer) cleanupLocked() {
	for _, cmd := range d.ffmpegProcs {
		if cmd.Process != nil {
			cmd.Process.Kill()
			go cmd.Wait() // collect exit status asynchronously
		}
	}
	d.ffmpegProcs = nil

	if d.pc != nil {
		d.pc.Close()
		d.pc = nil
	}
	log.Println("[DoorPeer] Cleanup complete (camera+mic released)")
}

func (d *DoorPeer) sendJSON(msg signalingMsg) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[DoorPeer] Failed to marshal message: %v", err)
		return
	}
	d.sendFn(data)
}

// ---------------------------------------------------------------------------
// FFmpeg media pumps — read encoded frames from FFmpeg stdout pipes and
// write them as samples to pion WebRTC tracks.
// ---------------------------------------------------------------------------

// pumpIVF reads VP8 frames from an IVF pipe and writes them to a video track.
func (d *DoorPeer) pumpIVF(r io.ReadCloser, track *pionwebrtc.TrackLocalStaticSample) {
	ivf, _, err := ivfreader.NewWith(r)
	if err != nil {
		log.Printf("[DoorPeer] IVF reader init failed: %v", err)
		return
	}

	for {
		frame, _, err := ivf.ParseNextFrame()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("[DoorPeer] IVF read error: %v", err)
			}
			return
		}
		if err := track.WriteSample(media.Sample{
			Data:     frame,
			Duration: time.Second / 30,
		}); err != nil {
			log.Printf("[DoorPeer] Video write error: %v", err)
			return
		}
	}
}

// pumpAudioRTP reads raw RTP packets from FFmpeg over UDP and writes them to the Pion track.
func (d *DoorPeer) pumpAudioRTP(listener *net.UDPConn, track *pionwebrtc.TrackLocalStaticRTP) {
	defer listener.Close()

	buf := make([]byte, 1500) // MTU size
	for {
		n, _, err := listener.ReadFromUDP(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				log.Printf("[DoorPeer] UDP RTP read error: %v", err)
			}
			return
		}

		// TrackLocalStaticRTP.Write handles raw unmarshaling, SSRC/PayloadType mapping,
		// and WebRTC bridging seamlessly.
		if _, writeErr := track.Write(buf[:n]); writeErr != nil {
			log.Printf("[DoorPeer] Audio RTP write error: %v", writeErr)
			return
		}
	}
}

// playRemoteAudio reads Opus RTP packets from the owner's audio track,
// wraps them in an OGG/Opus container, and pipes the result to ffplay
// for real-time speaker playback.
func (d *DoorPeer) playRemoteAudio(track *pionwebrtc.TrackRemote) {
	log.Println("[DoorPeer] Starting remote audio playback via ffplay (OGG pipe)")

	// Start ffplay to decode OGG/Opus from stdin and play through speakers
	playBin := ffplayBin()
	playArgs := []string{
		"-nodisp",        // no video window
		"-autoexit",      // exit when stream ends
		"-loglevel", "error",
		"-f", "ogg",      // input format: OGG container
		"-i", "pipe:0",   // read from stdin
	}

	cmd := exec.Command(playBin, playArgs...)
	cmd.Stderr = os.Stderr // surface ffplay errors for debugging

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("[DoorPeer] Failed to create ffplay stdin pipe: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[DoorPeer] Failed to start ffplay: %v", err)
		return
	}
	log.Printf("[DoorPeer] ffplay started (PID %d)", cmd.Process.Pid)

	// Write a proper OGG/Opus stream to ffplay's stdin using Pion's oggwriter
	oggWriter, err := oggwriter.NewWith(stdinPipe, 48000, 1)
	if err != nil {
		log.Printf("[DoorPeer] Failed to init oggwriter: %v", err)
		return
	}
	defer func() {
		oggWriter.Close()
		stdinPipe.Close()
		cmd.Wait()
		log.Println("[DoorPeer] Remote audio playback ended")
	}()

	for {
		rtpPacket, _, readErr := track.ReadRTP()
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				log.Printf("[DoorPeer] Remote audio read error: %v", readErr)
			}
			return
		}

		if len(rtpPacket.Payload) == 0 {
			continue
		}

		if err := oggWriter.WriteRTP(rtpPacket); err != nil {
			log.Printf("[DoorPeer] OGG write error: %v", err)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// FFmpeg argument builders — platform-specific capture flags.
// ---------------------------------------------------------------------------

func buildVideoArgs(device string) []string {
	common := []string{
		"-c:v", "libvpx",
		"-b:v", "500000",
		"-cpu-used", "5",
		"-deadline", "realtime",
		"-error-resilient", "1",
		"-auto-alt-ref", "0",
		"-f", "ivf",
		"pipe:1",
	}
	switch runtime.GOOS {
	case "windows":
		return append([]string{
			"-hide_banner", "-loglevel", "error",
			"-f", "dshow",
			"-video_size", "640x480",
			"-framerate", "30",
			"-i", fmt.Sprintf("video=%s", device),
		}, common...)
	default: // linux
		return append([]string{
			"-hide_banner", "-loglevel", "error",
			"-f", "v4l2",
			"-video_size", "640x480",
			"-framerate", "30",
			"-i", device,
		}, common...)
	}
}

func buildAudioArgs(device string, port int) []string {
	common := []string{
		"-c:a", "libopus",
		"-ar", "48000",
		"-ac", "1",
		"-b:a", "64000",
		"-vbr", "on",
		"-application", "voip",
		"-f", "rtp",
		fmt.Sprintf("rtp://127.0.0.1:%d", port),
	}
	switch runtime.GOOS {
	case "windows":
		return append([]string{
			"-hide_banner", "-loglevel", "error",
			"-f", "dshow",
			"-i", fmt.Sprintf("audio=%s", device),
		}, common...)
	default: // linux (PulseAudio)
		return append([]string{
			"-hide_banner", "-loglevel", "error",
			"-f", "pulse",
			"-i", device,
		}, common...)
	}
}

// buildPlaybackArgs is no longer used — playRemoteAudio now pipes OGG
// directly to ffplay. Kept as a stub for backward compatibility.
// func buildPlaybackArgs() []string {
// 	return []string{
// 		"-nodisp", "-autoexit", "-loglevel", "error",
// 		"-f", "ogg", "-i", "pipe:0",
// 	}
// }

// ---------------------------------------------------------------------------
// FFmpeg path resolution — finds the ffmpeg binary on PATH or common locations.
// ---------------------------------------------------------------------------

// ffmpegBin caches the resolved path to the ffmpeg executable.
var (
	ffmpegOnce sync.Once
	ffmpegPath string
)

func ffmpegBin() string {
	ffmpegOnce.Do(func() {
		// Check env override
		if p := os.Getenv("FFMPEG_PATH"); p != "" {
			ffmpegPath = p
			return
		}
		// Check PATH
		if p, err := exec.LookPath("ffmpeg"); err == nil {
			ffmpegPath = p
			return
		}
		// Check common Windows install locations
		if runtime.GOOS == "windows" {
			candidates := []string{
				`C:\ffmpeg\bin\ffmpeg.exe`,
				`C:\Program Files\ffmpeg\bin\ffmpeg.exe`,
				`C:\ProgramData\chocolatey\bin\ffmpeg.exe`,
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					ffmpegPath = c
					return
				}
			}
		}
		// Fallback — will fail at runtime with a clear error
		ffmpegPath = "ffmpeg"
	})
	log.Printf("[DoorPeer] Using ffmpeg: %s", ffmpegPath)
	return ffmpegPath
}

// ffplayBin caches the resolved path to the ffplay executable (ships with FFmpeg).
var (
	ffplayOnce sync.Once
	ffplayPath string
)

func ffplayBin() string {
	ffplayOnce.Do(func() {
		if p := os.Getenv("FFPLAY_PATH"); p != "" {
			ffplayPath = p
			return
		}
		// Derive from the ffmpeg path — ffplay is always alongside ffmpeg
		fp := ffmpegBin()
		if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(fp), "ffmpeg.exe") {
			candidate := fp[:len(fp)-len("ffmpeg.exe")] + "ffplay.exe"
			if _, err := os.Stat(candidate); err == nil {
				ffplayPath = candidate
				return
			}
		} else if strings.HasSuffix(fp, "ffmpeg") {
			candidate := fp[:len(fp)-len("ffmpeg")] + "ffplay"
			if _, err := os.Stat(candidate); err == nil {
				ffplayPath = candidate
				return
			}
		}
		if p, err := exec.LookPath("ffplay"); err == nil {
			ffplayPath = p
			return
		}
		if runtime.GOOS == "windows" {
			for _, c := range []string{
				`C:\ffmpeg\bin\ffplay.exe`,
				`C:\Program Files\ffmpeg\bin\ffplay.exe`,
			} {
				if _, err := os.Stat(c); err == nil {
					ffplayPath = c
					return
				}
			}
		}
		ffplayPath = "ffplay"
	})
	log.Printf("[DoorPeer] Using ffplay: %s", ffplayPath)
	return ffplayPath
}

// ---------------------------------------------------------------------------
// Device detection — auto-discovers camera and microphone names.
// Environment variables DOOR_VIDEO_DEVICE / DOOR_AUDIO_DEVICE override.
// ---------------------------------------------------------------------------

func detectDevices() (video, audio string) {
	prefVideo := os.Getenv("DOOR_VIDEO_DEVICE")
	prefAudio := os.Getenv("DOOR_AUDIO_DEVICE")

	switch runtime.GOOS {
	case "windows":
		video, audio = detectDShowDevices(prefVideo, prefAudio)
	default:
		video, audio = detectLinuxDevices()
	}

	// Fallbacks if discovery couldn't find them but env vars were provided
	if video == "" && prefVideo != "" {
		video = prefVideo
	}
	if audio == "" && prefAudio != "" {
		audio = prefAudio
	}
	return
}

func detectDShowDevices(prefVideo, prefAudio string) (video, audio string) {
	cmd := exec.Command(ffmpegBin(), "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	out, _ := cmd.CombinedOutput()

	// New FFmpeg formats often append '(video)' or '(audio)' instead of grouping by section headers.
	re := regexp.MustCompile(`\]\s+"(.+?)"\s+\((video|audio)\)`)
	altRe := regexp.MustCompile(`(?i)alternative name`)
	legacyRe := regexp.MustCompile(`\]\s+"(.+)"`)
	
	section := "" // For legacy grouped formatting

	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "directshow video devices") {
			section = "video"
			continue
		}
		if strings.Contains(lower, "directshow audio devices") {
			section = "audio"
			continue
		}
		if altRe.MatchString(line) {
			continue
		}

		// Try new format first: "... "Device Name" (audio)"
		devName, devType := "", ""
		if m := re.FindStringSubmatch(line); len(m) > 2 {
			devName, devType = m[1], m[2]
		} else if m := legacyRe.FindStringSubmatch(line); len(m) > 1 {
			// Fallback to legacy parsing if section headers were present
			devName, devType = m[1], section
		}

		if devName != "" && devType != "" {
			switch devType {
			case "video":
				if prefVideo != "" && strings.Contains(strings.ToLower(devName), strings.ToLower(prefVideo)) {
					video = devName
				} else if video == "" && prefVideo == "" {
					video = devName
				}
			case "audio":
				if prefAudio != "" && strings.Contains(strings.ToLower(devName), strings.ToLower(prefAudio)) {
					audio = devName
				} else if audio == "" && prefAudio == "" {
					audio = devName
				}
			}
		}
	}
	return
}

func detectLinuxDevices() (video, audio string) {
	// Use first available V4L2 camera device
	for i := 0; i < 4; i++ {
		dev := fmt.Sprintf("/dev/video%d", i)
		if _, err := os.Stat(dev); err == nil {
			video = dev
			break
		}
	}
	// PulseAudio default source
	audio = "default"
	return
}
