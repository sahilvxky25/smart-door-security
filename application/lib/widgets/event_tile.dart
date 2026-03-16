import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../models/event.dart';

class EventTile extends StatelessWidget {
  final Event event;
  final VoidCallback? onTap;

  const EventTile({super.key, required this.event, this.onTap});

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: CircleAvatar(
        backgroundColor: _colorForType(event.eventType),
        child: Icon(
          _iconForType(event.eventType),
          color: Colors.white,
          size: 20,
        ),
      ),
      title: Text(event.displayType),
      subtitle: Text(
        '${DateFormat.yMd().add_jm().format(event.timestamp.toLocal())}'
        '${event.user != null ? ' - ${event.user!.name}' : ''}',
      ),
      trailing: event.imageUrl.isNotEmpty
          ? const Icon(Icons.image, size: 20, color: Colors.grey)
          : null,
      onTap: onTap,
    );
  }

  IconData _iconForType(String type) {
    switch (type) {
      case Event.typeAuthorizedEntry:
        return Icons.check_circle;
      case Event.typeUnknownVisitor:
        return Icons.person_off;
      case Event.typeForcedEntry:
        return Icons.warning;
      case Event.typeManualUnlock:
        return Icons.key;
      case Event.typeSpoofAttempt:
        return Icons.masks;
      default:
        return Icons.info;
    }
  }

  Color _colorForType(String type) {
    switch (type) {
      case Event.typeAuthorizedEntry:
        return Colors.green;
      case Event.typeUnknownVisitor:
        return Colors.orange;
      case Event.typeForcedEntry:
        return Colors.red;
      case Event.typeManualUnlock:
        return Colors.blue;
      case Event.typeSpoofAttempt:
        return Colors.purple;
      default:
        return Colors.grey;
    }
  }
}
