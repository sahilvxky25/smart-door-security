import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../config/app_theme.dart';
import '../providers/call_provider.dart';

/// Full-screen incoming call UI with accept/decline buttons.
class IncomingCallScreen extends StatefulWidget {
  final String? imageUrl;

  const IncomingCallScreen({super.key, this.imageUrl});

  @override
  State<IncomingCallScreen> createState() => _IncomingCallScreenState();
}

class _IncomingCallScreenState extends State<IncomingCallScreen>
    with SingleTickerProviderStateMixin {
  late AnimationController _pulseController;
  Timer? _autoDeclineTimer;

  @override
  void initState() {
    super.initState();
    _pulseController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1200),
    )..repeat(reverse: true);

    _autoDeclineTimer = Timer(const Duration(seconds: 60), () {
      _decline();
    });
  }

  @override
  void dispose() {
    _autoDeclineTimer?.cancel();
    _pulseController.dispose();
    super.dispose();
  }

  void _accept() {
    _autoDeclineTimer?.cancel();
    context.read<CallProvider>().acceptCall();
    Navigator.of(context).pushReplacementNamed('/call');
  }

  void _decline() {
    _autoDeclineTimer?.cancel();
    context.read<CallProvider>().declineCall();
    if (mounted) Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: GradientBackground(
        child: SafeArea(
          child: Column(
            children: [
              const Spacer(flex: 2),

              // Visitor snapshot with pulse
              AnimatedBuilder(
                animation: _pulseController,
                builder: (context, child) {
                  final scale = 1.0 + (_pulseController.value * 0.06);
                  return Transform.scale(
                    scale: scale,
                    child: Container(
                      width: 130,
                      height: 130,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: AppColors.purple.withValues(
                            alpha: 0.5 + _pulseController.value * 0.5,
                          ),
                          width: 3,
                        ),
                        boxShadow: [
                          BoxShadow(
                            color: AppColors.purpleGlow.withValues(
                              alpha: 0.3 * _pulseController.value,
                            ),
                            blurRadius: 40,
                            spreadRadius: 12,
                          ),
                        ],
                      ),
                      child: ClipOval(
                        child: widget.imageUrl != null &&
                                widget.imageUrl!.isNotEmpty
                            ? Image.network(
                                widget.imageUrl!,
                                fit: BoxFit.cover,
                                errorBuilder: (_, _, _) =>
                                    _defaultAvatar(),
                              )
                            : _defaultAvatar(),
                      ),
                    ),
                  );
                },
              ),

              const SizedBox(height: 32),

              const Text(
                'Incoming Call',
                style: TextStyle(
                  color: AppColors.textPrimary,
                  fontSize: 28,
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 8),
              const Text(
                'Someone is at your door',
                style: TextStyle(color: AppColors.textMuted, fontSize: 16),
              ),

              const Spacer(flex: 3),

              // Accept / Decline
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 48),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                  children: [
                    _CallAction(
                      icon: Icons.call_end_rounded,
                      label: 'Decline',
                      color: AppColors.error,
                      onTap: _decline,
                    ),
                    _CallAction(
                      icon: Icons.videocam_rounded,
                      label: 'Accept',
                      color: AppColors.success,
                      onTap: _accept,
                    ),
                  ],
                ),
              ),

              const Spacer(flex: 1),
            ],
          ),
        ),
      ),
    );
  }

  Widget _defaultAvatar() {
    return Container(
      color: AppColors.surfaceDark,
      child: const Icon(Icons.person, size: 64, color: AppColors.textMuted),
    );
  }
}

class _CallAction extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;
  final VoidCallback onTap;

  const _CallAction({
    required this.icon,
    required this.label,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Material(
          type: MaterialType.transparency,
          child: InkWell(
            onTap: () {
              HapticFeedback.lightImpact();
              onTap();
            },
            customBorder: const CircleBorder(),
            child: Container(
              width: 68,
              height: 68,
              decoration: BoxDecoration(
                color: color,
                shape: BoxShape.circle,
                boxShadow: [
                  BoxShadow(
                    color: color.withValues(alpha: 0.4),
                    blurRadius: 20,
                    spreadRadius: 2,
                  ),
                ],
              ),
              child: Icon(icon, color: Colors.white, size: 32),
            ),
          ),
        ),
        const SizedBox(height: 10),
        Text(label,
            style: const TextStyle(color: AppColors.textSecondary, fontSize: 13)),
      ],
    );
  }
}
