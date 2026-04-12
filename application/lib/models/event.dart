import 'user.dart';

class Event {
  static const typeAuthorizedEntry = 'AUTHORIZED_ENTRY';
  static const typeUnknownVisitor = 'UNKNOWN_VISITOR';
  static const typeForcedEntry = 'FORCED_ENTRY';
  static const typeIntrusionCleared = 'INTRUSION_CLEARED';
  static const typeManualUnlock = 'MANUAL_UNLOCK';
  static const typeManualLock = 'MANUAL_LOCK';
  static const typeSpoofAttempt = 'SPOOF_ATTEMPT';
  static const typeDoorOpened = 'DOOR_OPENED';
  static const typeDoorClosed = 'DOOR_CLOSED';
  static const typeDoorLeftOpen = 'DOOR_LEFT_OPEN';
  static const typeVisitorApproaching = 'VISITOR_APPROACHING';
  static const typeHandleTamper = 'HANDLE_TAMPER';
  static const typeMotorTamper = 'MOTOR_TAMPER';

  final int id;
  final DateTime timestamp;
  final String eventType;
  final int userId;
  final User? user;
  final String imageUrl;

  Event({
    required this.id,
    required this.timestamp,
    required this.eventType,
    required this.userId,
    this.user,
    required this.imageUrl,
  });

  factory Event.fromJson(Map<String, dynamic> json) {
    return Event(
      id: json['id'] as int,
      timestamp: DateTime.parse(json['timestamp'] as String),
      eventType: json['event_type'] as String,
      userId: json['user_id'] as int,
      user: json['user'] != null ? User.fromJson(json['user']) : null,
      imageUrl: (json['image_url'] as String?) ?? '',
    );
  }

  String get displayType {
    switch (eventType) {
      case typeAuthorizedEntry:
        return 'Authorized Entry';
      case typeUnknownVisitor:
        return 'Unknown Visitor';
      case typeForcedEntry:
        return 'Forced Entry';
      case typeIntrusionCleared:
        return 'Intrusion Cleared';
      case typeManualUnlock:
        return 'Manual Unlock';
      case typeManualLock:
        return 'Manual Lock';
      case typeSpoofAttempt:
        return 'Spoof Attempt';
      case typeDoorOpened:
        return 'Door Opened';
      case typeDoorClosed:
        return 'Door Closed';
      case typeDoorLeftOpen:
        return 'Door Left Open';
      case typeVisitorApproaching:
        return 'Visitor Approaching';
      case typeHandleTamper:
        return 'Handle Tamper';
      case typeMotorTamper:
        return 'Motor Tamper';
      default:
        return eventType;
    }
  }
}
