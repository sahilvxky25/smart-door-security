import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../config/app_theme.dart';
import '../providers/door_provider.dart';

class DoorControlCard extends StatelessWidget {
  const DoorControlCard({super.key});

  @override
  Widget build(BuildContext context) {
    final door = context.watch<DoorProvider>();

    return GlassCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: AppColors.purpleSurface,
                  borderRadius: BorderRadius.circular(10),
                ),
                child: const Icon(Icons.door_front_door_outlined,
                    color: AppColors.purple, size: 20),
              ),
              const SizedBox(width: 12),
              const Text(
                'Door Control',
                style: TextStyle(
                  color: AppColors.textPrimary,
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(
                child: _GlassButton(
                  icon: Icons.lock_open_rounded,
                  label: 'Unlock',
                  color: AppColors.success,
                  loading: door.unlocking,
                  onTap: door.unlocking ? null : () => door.unlock(),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _GlassButton(
                  icon: Icons.lock_rounded,
                  label: 'Lock',
                  color: AppColors.error,
                  loading: door.locking,
                  onTap: door.locking ? null : () => door.lock(),
                ),
              ),
            ],
          ),
          if (door.lastStatus != null) ...[
            const SizedBox(height: 10),
            Text(
              door.lastStatus!,
              style: const TextStyle(
                color: AppColors.textMuted,
                fontSize: 12,
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _GlassButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;
  final bool loading;
  final VoidCallback? onTap;

  const _GlassButton({
    required this.icon,
    required this.label,
    required this.color,
    this.loading = false,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      type: MaterialType.transparency,
      child: InkWell(
        onTap: () {
          if (onTap != null) {
            HapticFeedback.lightImpact();
            onTap!();
          }
        },
        borderRadius: BorderRadius.circular(14),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 200),
          padding: const EdgeInsets.symmetric(vertical: 14),
          decoration: BoxDecoration(
            color: color.withValues(alpha: 0.12),
            borderRadius: BorderRadius.circular(14),
            border: Border.all(color: color.withValues(alpha: 0.3)),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              if (loading)
                SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: color,
                  ),
                )
              else
                Icon(icon, color: color, size: 20),
              const SizedBox(width: 8),
              Text(
                label,
                style: TextStyle(
                  color: color,
                  fontWeight: FontWeight.w600,
                  fontSize: 14,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
