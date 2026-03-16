import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import '../models/signaling_message.dart';
import '../services/signaling_service.dart';
import '../services/webrtc_service.dart';

enum CallState { idle, requesting, connecting, inCall, error }

class CallProvider extends ChangeNotifier {
  final WebRTCService _webrtc;
  final SignalingService _signaling;

  CallState state = CallState.idle;
  String? errorMessage;
  RTCVideoRenderer? remoteRenderer;

  bool get isMuted => _webrtc.isMuted;

  void toggleMute() {
    _webrtc.toggleMute();
    notifyListeners();
  }

  StreamSubscription? _msgSub;
  StreamSubscription? _streamSub;

  CallProvider({
    required WebRTCService webrtc,
    required SignalingService signaling,
  }) : _webrtc = webrtc,
       _signaling = signaling {
    _msgSub = _signaling.messages.listen(_handleMessage);
    _streamSub = _webrtc.remoteStream.listen(_handleRemoteStream);
  }

  Future<void> startCall() async {
    state = CallState.requesting;
    notifyListeners();
    _webrtc.sendCallRequest();
  }

  void _handleMessage(SignalingMessage msg) async {
    try {
      switch (msg.type) {
        case 'offer':
          state = CallState.connecting;
          notifyListeners();
          await _initRenderer();
          await _webrtc.handleOffer(msg.sdp!);
          break;
        case 'ice-candidate':
          if (msg.candidate != null) {
            await _webrtc.handleIceCandidate(msg.candidate!);
          }
          break;
        case 'hangup':
          await _webrtc.onRemoteHangup();
          state = CallState.idle;
          await _disposeRenderer();
          notifyListeners();
          break;
      }
    } catch (e) {
      errorMessage = e.toString();
      state = CallState.error;
      notifyListeners();
    }
  }

  void _handleRemoteStream(MediaStream? stream) {
    if (stream != null && remoteRenderer != null) {
      remoteRenderer!.srcObject = stream;
      state = CallState.inCall;
      notifyListeners();
    }
  }

  Future<void> hangup() async {
    await _webrtc.hangup();
    state = CallState.idle;
    await _disposeRenderer();
    notifyListeners();
  }

  Future<void> _initRenderer() async {
    remoteRenderer = RTCVideoRenderer();
    await remoteRenderer!.initialize();
  }

  Future<void> _disposeRenderer() async {
    remoteRenderer?.srcObject = null;
    await remoteRenderer?.dispose();
    remoteRenderer = null;
  }

  @override
  void dispose() {
    _msgSub?.cancel();
    _streamSub?.cancel();
    _disposeRenderer();
    super.dispose();
  }
}
