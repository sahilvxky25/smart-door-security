import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:awesome_notifications/awesome_notifications.dart';
import '../models/signaling_message.dart';
import '../providers/call_provider.dart';
import '../providers/event_provider.dart';
import '../providers/door_provider.dart';
import '../services/signaling_service.dart';

// Notification IDs – one slot per alert category.
const _kNotifIdIncomingCall = 1;
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

      // Trigger the "Ringing" state
      _callProvider.onIncomingCall(msg.imageUrl);

      // Show high-priority "Awesome Notification"
      _showIncomingCallNotification(msg);

      // In-app navigation to call UI
      _navigatorKey.currentState?.pushNamed(
        '/incoming_call',
        arguments: msg.imageUrl,
      );

      latestNotification = null;
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

  Future<void> _showIncomingCallNotification(SignalingMessage msg) async {
    await AwesomeNotifications().createNotification(
      content: NotificationContent(
        id: _kNotifIdIncomingCall,
        channelKey: 'call_channel',
        title: 'Incoming Video Call',
        body: 'Someone is at your door. Tap to answer.',
        bigPicture: msg.imageUrl,
        notificationLayout: NotificationLayout.BigPicture,
        fullScreenIntent: true,
        wakeUpScreen: true,
        category: NotificationCategory.Call,
        autoDismissible: false,
      ),
      actionButtons: [
        NotificationActionButton(
          key: 'ACCEPT',
          label: 'Accept',
          color: Colors.green,
          actionType: ActionType.Default,
        ),
        NotificationActionButton(
          key: 'DECLINE',
          label: 'Decline',
          color: Colors.red,
          actionType: ActionType.DismissAction,
        ),
      ],
    );
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
