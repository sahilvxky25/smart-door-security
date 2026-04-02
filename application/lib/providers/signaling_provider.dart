import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:awesome_notifications/awesome_notifications.dart';
import '../models/signaling_message.dart';
import '../providers/call_provider.dart';
import '../providers/event_provider.dart';
import '../providers/door_provider.dart';
import 'package:flutter_callkit_incoming/flutter_callkit_incoming.dart';
import 'package:flutter_callkit_incoming/entities/call_event.dart' as ck;
import '../services/fcm_service.dart';
import '../services/api_service.dart';
import '../services/signaling_service.dart';
import '../services/callkit_service.dart';

// Notification IDs – one slot per alert category.
const _kNotifIdSpoof = 2;

class SignalingProvider extends ChangeNotifier {
  final SignalingService _service;
  final GlobalKey<NavigatorState> _navigatorKey;
  final CallProvider _callProvider;
  final EventProvider _eventProvider;
  final DoorProvider _doorProvider;

  bool connected = false;
  SignalingMessage? latestNotification;

  StreamSubscription? _msgSub;
  StreamSubscription? _connSub;

  SignalingProvider({
    required SignalingService service,
    required GlobalKey<NavigatorState> navigatorKey,
    required CallProvider callProvider,
    required EventProvider eventProvider,
    required DoorProvider doorProvider,
  }) : _service = service,
       _navigatorKey = navigatorKey,
       _callProvider = callProvider,
       _eventProvider = eventProvider,
       _doorProvider = doorProvider {
    _connSub = _service.connectionState.listen((state) {
      connected = state;
      notifyListeners();
    });
    _msgSub = _service.messages.listen(_handleMessage);

    // Listen for notification actions (Accept/Decline buttons)
    AwesomeNotifications().setListeners(
      onActionReceivedMethod: _onActionReceivedMethod,
    );

    // Listen for CallKit events
    FlutterCallkitIncoming.onEvent.listen(_onCallKitEvent);
  }

  void initializeFCM(ApiService api) {
    FCMService().initialize(api);
  }

  void dismissIncomingCall(String? callId) {
    if (callId != null) {
      CallKitService().endCall(callId);
    }
  }

  void _onCallKitEvent(ck.CallEvent? event) {
    if (event == null) return;
    switch (event.event) {
      case ck.Event.actionCallAccept:
        _callProvider.acceptCall();
        _navigatorKey.currentState?.pushNamed('/call');
        break;
      case ck.Event.actionCallDecline:
        _callProvider.declineCall();
        break;
      default:
        break;
    }
  }

  static Future<void> _onActionReceivedMethod(ReceivedAction receivedAction) async {
    if (receivedAction.buttonKeyPressed == 'ACCEPT') {
      // If user accepts, they are already navigating to /incoming_call by the tap action
    } else if (receivedAction.buttonKeyPressed == 'DECLINE') {
      await AwesomeNotifications().dismiss(receivedAction.id!);
    }
  }

  void _handleMessage(SignalingMessage msg) {
    // Incoming call from backend (unknown visitor OR spoof detected)
    if (msg.type == 'incoming_call') {
      latestNotification = msg;
      notifyListeners();

      // Trigger the "Ringing" state in Provider
      _callProvider.onIncomingCall(msg.imageUrl, msg.callId);

      // Show native CallKit UI
      if (msg.callId != null) {
        CallKitService().showIncomingCall(
          callId: msg.callId!,
          imageUrl: msg.imageUrl,
          title: msg.title,
          body: msg.body,
        );
      }

      latestNotification = null;
      return;
    }

    // Call timed out or cancelled by backend
    if (msg.type == 'missed_call') {
      _callProvider.declineCall(); // reset State
      if (msg.callId != null) {
        CallKitService().endCall(msg.callId!);
      }
      return;
    }

    // Real-time event update — refresh dashboard and light up badge
    if (msg.type == 'event_update') {
      _eventProvider.fetchEventsFromWs();
      _doorProvider.fetchState();
      return;
    }

    // Security alert — show notification AND refresh event list
    if (msg.type == 'alert') {
      _showAlertNotification(msg);
      _eventProvider.fetchEventsFromWs();
      _doorProvider.fetchState();
    }
  }


  Future<void> _showAlertNotification(SignalingMessage msg) async {
    await AwesomeNotifications().createNotification(
      content: NotificationContent(
        id: msg.eventType.hashCode % 1000 + 100,
        channelKey: 'alerts_channel',
        title: msg.title ?? 'Security Alert',
        body: msg.body ?? 'A security event occurred at your door.',
        notificationLayout: NotificationLayout.Default,
      ),
    );
  }

  void connect(String wsUrl, {int? userId}) {
    final url = userId != null ? '$wsUrl&user_id=$userId' : wsUrl;
    _service.connect(url);
  }
  void disconnect() => _service.disconnect();

  @override
  void dispose() {
    _msgSub?.cancel();
    _connSub?.cancel();
    super.dispose();
  }
}
