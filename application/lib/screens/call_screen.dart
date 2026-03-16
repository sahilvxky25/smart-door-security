import 'package:flutter/material.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'package:provider/provider.dart';
import '../providers/call_provider.dart';
import '../providers/door_provider.dart';

class CallScreen extends StatefulWidget {
  const CallScreen({super.key});

  @override
  State<CallScreen> createState() => _CallScreenState();
}

class _CallScreenState extends State<CallScreen> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() {
      context.read<CallProvider>().startCall();
    });
  }

  @override
  Widget build(BuildContext context) {
    final callState = context.watch<CallProvider>();

    return Scaffold(
      appBar: AppBar(
        title: const Text('Live Call'),
        automaticallyImplyLeading: false,
      ),
      body: Stack(
        children: [
          // Video from door
          callState.remoteRenderer != null
              ? RTCVideoView(callState.remoteRenderer!)
              : Container(
                  color: Colors.black,
                  child: const Center(child: CircularProgressIndicator()),
                ),
          // Status overlay
          Positioned(
            top: 16,
            left: 16,
            right: 16,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              decoration: BoxDecoration(
                color: Colors.black87,
                borderRadius: BorderRadius.circular(20),
              ),
              child: Text(
                _statusText(callState.state),
                style: const TextStyle(color: Colors.white),
                textAlign: TextAlign.center,
              ),
            ),
          ),
          // Control buttons
          Positioned(
            bottom: 32,
            left: 0,
            right: 0,
            child: Center(
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: [
                  // Hang up
                  FloatingActionButton(
                    onPressed: () async {
                      await callState.hangup();
                      if (mounted) {
                        Navigator.pop(context);
                      }
                    },
                    backgroundColor: Colors.red,
                    child: const Icon(Icons.call_end),
                  ),
                  // Mute / unmute mic
                  FloatingActionButton(
                    onPressed: () => callState.toggleMute(),
                    backgroundColor: callState.isMuted
                        ? Colors.grey
                        : Colors.green,
                    child: Icon(callState.isMuted ? Icons.mic_off : Icons.mic),
                  ),
                  // Unlock
                  FloatingActionButton(
                    onPressed: () async {
                      final door = context.read<DoorProvider>();
                      await door.unlock();
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('Door unlocked')),
                      );
                    },
                    backgroundColor: Colors.blue,
                    child: const Icon(Icons.lock_open),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  String _statusText(CallState state) {
    switch (state) {
      case CallState.idle:
        return 'Idle';
      case CallState.requesting:
        return 'Requesting call...';
      case CallState.connecting:
        return 'Connecting...';
      case CallState.inCall:
        return 'In Call';
      case CallState.error:
        return 'Connection failed';
    }
  }
}
