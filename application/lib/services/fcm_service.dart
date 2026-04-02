import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:uuid/uuid.dart';
import 'api_service.dart';
import 'callkit_service.dart';
import 'dart:developer';

class FCMService {
  static final FCMService _instance = FCMService._internal();
  factory FCMService() => _instance;
  FCMService._internal();

  final FirebaseMessaging _fcm = FirebaseMessaging.instance;

  Future<void> initialize(ApiService api) async {
    // Request permissions for iOS
    NotificationSettings settings = await _fcm.requestPermission(
      alert: true,
      badge: true,
      sound: true,
    );

    if (settings.authorizationStatus == AuthorizationStatus.authorized) {
      log('User granted permission');
    }

    // Get token and send to backend
    String? token = await _fcm.getToken();
    if (token != null) {
      log('FCM Token: $token');
      try {
        await api.updateFCMToken(token);
      } catch (e) {
        log('Error updating FCM token on backend: $e');
      }
    }

    // Listen for token refreshes
    _fcm.onTokenRefresh.listen((newToken) {
      api.updateFCMToken(newToken);
    });

    // Handle messages when app is in foreground
    FirebaseMessaging.onMessage.listen((RemoteMessage message) {
      log('Got a message whilst in the foreground!');
      _handleMessage(message);
    });

    // Handle messages when app is opened from a notification
    FirebaseMessaging.onMessageOpenedApp.listen((RemoteMessage message) {
      log('App opened from notification: ${message.data}');
    });
  }

  static Future<void> handleBackgroundMessage(RemoteMessage message) async {
    log("Handling a background message: ${message.messageId}");
    _instance._handleMessage(message);
  }

  void _handleMessage(RemoteMessage message) {
    final data = message.data;
    final type = data['type'];

    if (type == 'incoming_call') {
      final callId = data['call_id'] ?? const Uuid().v4();
      CallKitService().showIncomingCall(
        callId: callId,
        imageUrl: data['image_url'],
        title: data['title'] ?? 'Smart Door Alert',
        body: data['body'] ?? 'Incoming video call',
      );
    } else if (type == 'missed_call') {
      final callId = data['call_id'];
      if (callId != null) {
        CallKitService().endCall(callId);
      }
    }
  }
}
