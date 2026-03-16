import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/signaling_provider.dart';

class ConnectionStatus extends StatelessWidget {
  const ConnectionStatus({super.key});

  @override
  Widget build(BuildContext context) {
    final connected = context.watch<SignalingProvider>().connected;

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: 10,
          height: 10,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: connected ? Colors.green : Colors.red,
          ),
        ),
        const SizedBox(width: 6),
        Text(
          connected ? 'Connected' : 'Disconnected',
          style: Theme.of(context).textTheme.bodySmall,
        ),
      ],
    );
  }
}
