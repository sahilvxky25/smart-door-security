import 'user.dart';

class Event {
  static const typeAuthorizedEntry = 'AUTHORIZED_ENTRY';
  static const typeUnknownVisitor = 'UNKNOWN_VISITOR';
  static const typeForcedEntry = 'FORCED_ENTRY';
  static const typeManualUnlock = 'MANUAL_UNLOCK';
  static const typeSpoofAttempt = 'SPOOF_ATTEMPT';

  final int id;
  final DateTime timestamp;
  final String eventType;
  final int? userId;
  final User? user;
  final String imageUrl;

  Event({
    required this.id,
    required this.timestamp,
    required this.eventType,
    this.userId,
    this.user,
    required this.imageUrl,
  });

  factory Event.fromJson(Map<String, dynamic> json) {
    return Event(
      id: json['id'] as int,
      timestamp: DateTime.parse(json['timestamp'] as String),
      eventType: json['event_type'] as String,
      userId: json['user_id'] as int?,
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
      case typeManualUnlock:
        return 'Manual Unlock';
      case typeSpoofAttempt:
        return 'Spoof Attempt';
      default:
        return eventType;
    }
  }
}
