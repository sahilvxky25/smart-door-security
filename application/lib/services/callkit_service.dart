import 'package:flutter_callkit_incoming/flutter_callkit_incoming.dart';
import 'package:flutter_callkit_incoming/entities/entities.dart';

class CallKitService {
  static final CallKitService _instance = CallKitService._internal();
  factory CallKitService() => _instance;
  CallKitService._internal();

  Future<void> showIncomingCall({
    required String callId,
    required String? imageUrl,
    String? title,
    String? body,
  }) async {
    final params = CallKitParams(
      id: callId,
      nameCaller: title ?? 'Smart Door Security',
      appName: 'Smart Door',
      avatar: imageUrl,
      handle: body ?? 'Incoming Video Call',
      type: 0, // Audio/Video toggle depends on platform support
      duration: 60000,
      textAccept: 'Accept',
      textDecline: 'Decline',
      missedCallNotification: const NotificationParams(
        showNotification: true,
        isShowCallback: true,
        subtitle: 'Missed security call',
        callbackText: 'Call back',
      ),
      extra: <String, dynamic>{'callId': callId},
      headers: <String, dynamic>{'apiKey': 'Abc@123', 'platform': 'flutter'},
      android: AndroidParams(
        isCustomNotification: true,
        isShowCallID: true,
        isShowLogo: false,
        ringtonePath: 'makabhosda_aag',
        backgroundColor: '#09121C',
        backgroundUrl: 'https://dummyimage.com/1x1/000000/000000.png',
        actionColor: '#4CAF50',
      ),
      ios: const IOSParams(
        iconName: 'CallKitLogo',
        handleType: 'generic',
        supportsVideo: true,
        maximumCallGroups: 2,
        maximumCallsPerCallGroup: 1,
        audioSessionMode: 'default',
        audioSessionActive: true,
        audioSessionPreferredSampleRate: 44100.0,
        audioSessionPreferredIOBufferDuration: 0.005,
        supportsDTMF: true,
        supportsHolding: true,
        supportsGrouping: false,
        supportsUngrouping: false,
        ringtonePath: 'system_ringtone_default',
      ),
    );

    await FlutterCallkitIncoming.showCallkitIncoming(params);
  }

  Future<void> endCall(String callId) async {
    await FlutterCallkitIncoming.endCall(callId);
  }

  Future<void> endAllCalls() async {
    await FlutterCallkitIncoming.endAllCalls();
  }
}
