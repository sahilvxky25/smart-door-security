import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import '../models/signaling_message.dart';
import '../services/callkit_service.dart';
import '../services/signaling_service.dart';
import '../services/webrtc_service.dart';

enum CallState { idle, ringing, requesting, connecting, inCall, error }

class CallProvider extends ChangeNotifier {
  final WebRTCService _webrtc;
  final SignalingService _signaling;

  CallState state = CallState.idle;
  String? errorMessage;
  String? incomingImageUrl;
  String? activeCallId;
  RTCVideoRenderer? remoteRenderer;

  bool get isMuted => _webrtc.isMuted;

  void toggleMute() {
    _webrtc.toggleMute();
    notifyListeners();
  }

  StreamSubscription? _msgSub;
  StreamSubscription? _streamSub;
  Timer? _requestTimer;

  CallProvider({
    required WebRTCService webrtc,
    required SignalingService signaling,
  }) : _webrtc = webrtc,
       _signaling = signaling {
    _msgSub = _signaling.messages.listen(_handleMessage);
    _streamSub = _webrtc.remoteStream.listen(_handleRemoteStream);
  }

  /// Called by SignalingProvider when an incoming_call message arrives.
  /// Transitions to ringing state — the IncomingCallScreen reads this.
  void onIncomingCall(String? imageUrl, String? callId) {
    incomingImageUrl = imageUrl;
    activeCallId = callId;
    state = CallState.ringing;
    notifyListeners();
  }

  Future<void> resetCallState({bool endNativeCallUi = false}) async {
    _requestTimer?.cancel();
    final callId = activeCallId;
    state = CallState.idle;
    incomingImageUrl = null;
    activeCallId = null;
    await _disposeRenderer();
    if (endNativeCallUi && callId != null) {
      await CallKitService().endCall(callId);
    }
    notifyListeners();
  }

  /// User accepted the incoming call → tell the backend to start WebRTC.
  void acceptCall() {
    state = CallState.requesting;
    notifyListeners();
    _signaling.send({
      'type': 'call_accepted',
      'call_id': activeCallId,
    });
    _requestTimer?.cancel();
    _requestTimer = Timer(const Duration(seconds: 15), () {
      state = CallState.error;
      errorMessage =
          'No response from door. Make sure the door device is online.';
      notifyListeners();
    });
  }

  /// User declined the incoming call.
  Future<void> declineCall() async {
    final callId = activeCallId;
    if (activeCallId != null) {
      _signaling.send({
        'type': 'call_declined',
        'call_id': activeCallId,
      });
    }
    if (callId != null) {
      await CallKitService().endCall(callId);
    }
    await resetCallState();
  }

  /// Manual call request (e.g. from a "Call Door" button).
  Future<void> startCall() async {
    state = CallState.requesting;
    notifyListeners();
    _webrtc.sendCallRequest();
    _requestTimer?.cancel();
    _requestTimer = Timer(const Duration(seconds: 15), () {
      state = CallState.error;
      errorMessage =
          'No response from door. Make sure the door device is online.';
      notifyListeners();
    });
  }

  void _handleMessage(SignalingMessage msg) async {
    try {
      switch (msg.type) {
        case 'offer':
          _requestTimer?.cancel();
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
          await resetCallState(endNativeCallUi: true);
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
    _requestTimer?.cancel();
    final callId = activeCallId;
    if (activeCallId != null) {
      _signaling.send({
        'type': 'hangup',
        'call_id': activeCallId,
      });
    }
    await _webrtc.hangup();
    if (callId != null) {
      await CallKitService().endCall(callId);
    }
    await resetCallState();
  }

  Future<void> endFromNativeUi() async {
    _requestTimer?.cancel();
    if (activeCallId != null) {
      _signaling.send({
        'type': 'hangup',
        'call_id': activeCallId,
      });
    }
    await _webrtc.hangup();
    await resetCallState();
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
    _requestTimer?.cancel();
    _msgSub?.cancel();
    _streamSub?.cancel();
    _disposeRenderer();
    super.dispose();
  }
}
