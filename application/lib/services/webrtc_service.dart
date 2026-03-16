import 'dart:async';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import '../config/app_config.dart';
import 'signaling_service.dart';

class WebRTCService {
  final SignalingService _signaling;
  final AppConfig _config;

  RTCPeerConnection? _pc;
  MediaStream? _localStream;
  final _remoteStreamController = StreamController<MediaStream?>.broadcast();

  Stream<MediaStream?> get remoteStream => _remoteStreamController.stream;

  WebRTCService({
    required SignalingService signaling,
    required AppConfig config,
  })  : _signaling = signaling,
        _config = config;

  Future<void> handleOffer(Map<String, dynamic> sdp) async {
    await _createPeerConnection();

    // Get owner's microphone (no video — owner only sends audio).
    // Gracefully continue without audio if the device denies/lacks a mic.
    try {
      _localStream = await navigator.mediaDevices.getUserMedia({'audio': true, 'video': false});
      for (final track in _localStream!.getTracks()) {
        await _pc!.addTrack(track, _localStream!);
      }
    } catch (_) {
      // Mic unavailable — proceed as receive-only (user can still see the door).
      _localStream = null;
    }

    // Set remote description from door's offer
    await _pc!.setRemoteDescription(
      RTCSessionDescription(sdp['sdp'] as String?, sdp['type'] as String?),
    );

    // Create and send answer
    final answer = await _pc!.createAnswer();
    await _pc!.setLocalDescription(answer);
    _signaling.send({
      'type': 'answer',
      'sdp': {'type': answer.type, 'sdp': answer.sdp},
    });
  }

  Future<void> handleIceCandidate(Map<String, dynamic> candidate) async {
    if (_pc == null) return;
    await _pc!.addCandidate(
      RTCIceCandidate(
        candidate['candidate'] as String?,
        candidate['sdpMid'] as String?,
        candidate['sdpMLineIndex'] as int?,
      ),
    );
  }

  bool _isMuted = false;
  bool get isMuted => _isMuted;

  void sendCallRequest() {
    _signaling.send({'type': 'call_request'});
  }

  void toggleMute() {
    if (_localStream == null) return;
    _isMuted = !_isMuted;
    for (final track in _localStream!.getAudioTracks()) {
      track.enabled = !_isMuted;
    }
  }

  Future<void> hangup() async {
    _signaling.send({'type': 'hangup'});
    await _cleanup();
  }

  Future<void> onRemoteHangup() async {
    await _cleanup();
  }

  Future<void> _createPeerConnection() async {
    _pc = await createPeerConnection(_config.iceServers);

    _pc!.onTrack = (RTCTrackEvent event) {
      if (event.streams.isNotEmpty) {
        _remoteStreamController.add(event.streams.first);
      }
    };

    _pc!.onIceCandidate = (RTCIceCandidate candidate) {
      _signaling.send({
        'type': 'ice-candidate',
        'candidate': {
          'candidate': candidate.candidate,
          'sdpMid': candidate.sdpMid,
          'sdpMLineIndex': candidate.sdpMLineIndex,
        },
      });
    };
  }

  Future<void> _cleanup() async {
    if (_localStream != null) {
      for (final track in _localStream!.getTracks()) {
        await track.stop();
      }
      _localStream = null;
    }
    await _pc?.close();
    _pc = null;
    _remoteStreamController.add(null);
  }

  void dispose() {
    _cleanup();
    _remoteStreamController.close();
  }
}
