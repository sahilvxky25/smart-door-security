import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import '../models/signaling_message.dart';
import '../services/signaling_service.dart';

// Notification IDs – one slot per alert category.
const _kNotifIdUnknownVisitor = 1;
const _kNotifIdSpoof = 2;

class SignalingProvider extends ChangeNotifier {
  final SignalingService _service;
  final FlutterLocalNotificationsPlugin _notifications;
  final GlobalKey<NavigatorState> _navigatorKey;

  bool connected = false;
  SignalingMessage? latestNotification;

  StreamSubscription? _msgSub;
  StreamSubscription? _connSub;

  SignalingProvider({
    required SignalingService service,
    required FlutterLocalNotificationsPlugin notifications,
    required GlobalKey<NavigatorState> navigatorKey,
  }) : _service = service,
       _notifications = notifications,
       _navigatorKey = navigatorKey {
    _connSub = _service.connectionState.listen((state) {
      connected = state;
      notifyListeners();
    });
    _msgSub = _service.messages.listen(_handleMessage);
  }

  void _handleMessage(SignalingMessage msg) {
    final isCallTrigger =
        msg.type == 'unknown_visitor' ||
        (msg.type == 'alert' &&
            (msg.eventType == 'UNKNOWN_VISITOR' ||
                msg.eventType == 'SPOOF_ATTEMPT'));

    if (isCallTrigger) {
      latestNotification = msg;
      notifyListeners();
      _showIncomingVisitorNotification(msg);
      // Navigate to call screen so user can start a video call immediately.
      _navigatorKey.currentState?.pushNamed('/call');
    }
  }

  Future<void> _showIncomingVisitorNotification(SignalingMessage msg) async {
    final isSpoof = msg.type == 'alert' && msg.eventType == 'SPOOF_ATTEMPT';

    final title =
        msg.title ?? (isSpoof ? 'Spoof Attempt Detected' : 'Unknown Visitor');
    final body =
        msg.body ??
        (isSpoof
            ? 'A spoof was detected at your door.'
            : 'Someone is at your door. Tap to start a video call.');

    final vibration = Int64List.fromList([0, 500, 200, 500, 200, 500]);

    final details = NotificationDetails(
      android: AndroidNotificationDetails(
        'incoming_visitor',
        'Incoming Visitor',
        channelDescription: 'Video call alerts when someone is at the door',
        importance: Importance.max,
        priority: Priority.high,
        playSound: true,
        enableVibration: true,
        vibrationPattern: vibration,
        fullScreenIntent: true,
        category: AndroidNotificationCategory.call,
      ),
    );

    await _notifications.show(
      isSpoof ? _kNotifIdSpoof : _kNotifIdUnknownVisitor,
      title,
      body,
      details,
    );
  }

  void connect(String wsUrl) => _service.connect(wsUrl);
  void disconnect() => _service.disconnect();

  @override
  void dispose() {
    _msgSub?.cancel();
    _connSub?.cancel();
    super.dispose();
  }
}
