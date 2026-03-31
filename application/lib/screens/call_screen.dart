import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'package:provider/provider.dart';
import '../config/app_theme.dart';
import '../providers/call_provider.dart';
import '../providers/door_provider.dart';

class CallScreen extends StatefulWidget {
  const CallScreen({super.key});

  @override
  State<CallScreen> createState() => _CallScreenState();
}

class _CallScreenState extends State<CallScreen> {
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: Consumer<CallProvider>(
        builder: (context, call, _) {
          return Stack(
            fit: StackFit.expand,
            children: [
              // ── Remote video ──
              if (call.remoteRenderer != null &&
                  call.state == CallState.inCall)
                RTCVideoView(
                  call.remoteRenderer!,
                  objectFit: RTCVideoViewObjectFit.RTCVideoViewObjectFitCover,
                )
              else
                GradientBackground(
                  child: Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          width: 80,
                          height: 80,
                          decoration: BoxDecoration(
                            shape: BoxShape.circle,
                            color: AppColors.purpleSurface,
                            boxShadow: [
                              BoxShadow(
                                color: AppColors.purpleGlow,
                                blurRadius: 30,
                              ),
                            ],
                          ),
                          child: const Icon(Icons.videocam_rounded,
                              color: AppColors.purple, size: 36),
                        ),
                        const SizedBox(height: 20),
                        Text(
                          _statusText(call.state),
                          style: const TextStyle(
                            color: AppColors.textPrimary,
                            fontSize: 18,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        if (call.errorMessage != null) ...[
                          const SizedBox(height: 8),
                          Padding(
                            padding: const EdgeInsets.symmetric(horizontal: 32),
                            child: Text(
                              call.errorMessage!,
                              textAlign: TextAlign.center,
                              style: const TextStyle(
                                  color: AppColors.error, fontSize: 13),
                            ),
                          ),
                        ],
                        if (call.state == CallState.connecting ||
                            call.state == CallState.requesting)
                          const Padding(
                            padding: EdgeInsets.only(top: 20),
                            child: CircularProgressIndicator(),
                          ),
                      ],
                    ),
                  ),
                ),

              // ── Top status bar ──
              Positioned(
                top: 0,
                left: 0,
                right: 0,
                child: SafeArea(
                  child: Padding(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 20, vertical: 12),
                    child: Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 10, vertical: 5),
                          decoration: BoxDecoration(
                            color: Colors.black45,
                            borderRadius: BorderRadius.circular(20),
                          ),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Container(
                                width: 7,
                                height: 7,
                                decoration: BoxDecoration(
                                  shape: BoxShape.circle,
                                  color: call.state == CallState.inCall
                                      ? AppColors.success
                                      : AppColors.warning,
                                ),
                              ),
                              const SizedBox(width: 6),
                              Text(
                                _statusText(call.state),
                                style: const TextStyle(
                                    color: Colors.white,
                                    fontSize: 12,
                                    fontWeight: FontWeight.w500),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),

              // ── Bottom controls ──
              Positioned(
                bottom: 0,
                left: 0,
                right: 0,
                child: SafeArea(
                  child: Padding(
                    padding: const EdgeInsets.only(bottom: 24),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                      children: [
                        _ControlButton(
                          icon: Icons.call_end_rounded,
                          color: AppColors.error,
                          label: 'End',
                          onTap: () async {
                            await call.hangup();
                            if (context.mounted) Navigator.pop(context);
                          },
                        ),
                        _ControlButton(
                          icon: call.isMuted
                              ? Icons.mic_off_rounded
                              : Icons.mic_rounded,
                          color: call.isMuted
                              ? AppColors.warning
                              : AppColors.textSecondary,
                          label: call.isMuted ? 'Unmute' : 'Mute',
                          onTap: () => call.toggleMute(),
                        ),
                        _ControlButton(
                          icon: Icons.lock_open_rounded,
                          color: AppColors.success,
                          label: 'Unlock',
                          onTap: () =>
                              context.read<DoorProvider>().unlock(),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ],
          );
        },
      ),
    );
  }

  String _statusText(CallState state) {
    switch (state) {
      case CallState.idle:
        return 'Idle';
      case CallState.ringing:
        return 'Ringing...';
      case CallState.requesting:
        return 'Requesting...';
      case CallState.connecting:
        return 'Connecting...';
      case CallState.inCall:
        return 'In Call';
      case CallState.error:
        return 'Failed';
    }
  }
}

class _ControlButton extends StatelessWidget {
  final IconData icon;
  final Color color;
  final String label;
  final VoidCallback onTap;

  const _ControlButton({
    required this.icon,
    required this.color,
    required this.label,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      type: MaterialType.transparency,
      child: InkWell(
        onTap: () {
          HapticFeedback.lightImpact();
          onTap();
        },
        customBorder: const CircleBorder(),
        child: Padding(
          padding: const EdgeInsets.all(8.0),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 56,
                height: 56,
                decoration: BoxDecoration(
                  color: color.withValues(alpha: 0.2),
                  shape: BoxShape.circle,
                  border: Border.all(color: color.withValues(alpha: 0.4)),
                ),
                child: Icon(icon, color: color, size: 26),
              ),
              const SizedBox(height: 6),
              Text(label,
                  style: const TextStyle(color: Colors.white70, fontSize: 11)),
            ],
          ),
        ),
      ),
    );
  }
}
