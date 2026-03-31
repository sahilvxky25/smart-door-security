import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../config/app_theme.dart';
import '../models/event.dart';

class EventTile extends StatelessWidget {
  final Event event;
  final VoidCallback? onTap;

  const EventTile({super.key, required this.event, this.onTap});

  @override
  Widget build(BuildContext context) {
    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 2),
      leading: Container(
        width: 40,
        height: 40,
        decoration: BoxDecoration(
          color: _colorForType(event.eventType).withValues(alpha: 0.15),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Icon(
          _iconForType(event.eventType),
          color: _colorForType(event.eventType),
          size: 20,
        ),
      ),
      title: Text(
        event.displayType,
        style: const TextStyle(
          color: AppColors.textPrimary,
          fontSize: 14,
          fontWeight: FontWeight.w500,
        ),
      ),
      subtitle: Text(
        '${DateFormat.yMd().add_jm().format(event.timestamp.toLocal())}'
        '${event.user != null ? ' · ${event.user!.name}' : ''}',
        style: const TextStyle(color: AppColors.textMuted, fontSize: 12),
      ),
      trailing: event.imageUrl.isNotEmpty
          ? Icon(Icons.image_outlined, size: 18, color: AppColors.textMuted)
          : null,
      onTap: onTap,
    );
  }

  IconData _iconForType(String type) {
    switch (type) {
      case Event.typeAuthorizedEntry:
        return Icons.check_circle_outline;
      case Event.typeUnknownVisitor:
        return Icons.person_off_outlined;
      case Event.typeForcedEntry:
        return Icons.warning_amber_rounded;
      case Event.typeManualUnlock:
        return Icons.key_rounded;
      case Event.typeSpoofAttempt:
        return Icons.masks_outlined;
      default:
        return Icons.info_outline;
    }
  }

  Color _colorForType(String type) {
    switch (type) {
      case Event.typeAuthorizedEntry:
        return AppColors.success;
      case Event.typeUnknownVisitor:
        return AppColors.warning;
      case Event.typeForcedEntry:
        return AppColors.error;
      case Event.typeManualUnlock:
        return AppColors.purple;
      case Event.typeSpoofAttempt:
        return const Color(0xFFE040FB);
      default:
        return AppColors.textMuted;
    }
  }
}
