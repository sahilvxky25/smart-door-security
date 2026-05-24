import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:provider/provider.dart';
import '../config/app_theme.dart';
import '../models/event.dart';
import '../providers/call_provider.dart';

class EventDetailScreen extends StatelessWidget {
  final Event event;

  const EventDetailScreen({super.key, required this.event});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      extendBodyBehindAppBar: true,
      appBar: AppBar(title: const Text('Event Details')),
      body: GradientBackground(
        child: SafeArea(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(20),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (event.imageUrl.isNotEmpty)
                  ClipRRect(
                    borderRadius: BorderRadius.circular(20),
                    child: CachedNetworkImage(
                      imageUrl: event.imageUrl,
                      height: 280,
                      width: double.infinity,
                      fit: BoxFit.cover,
                      placeholder: (_, _) =>
                          const Center(child: CircularProgressIndicator()),
                      errorWidget: (_, _, _) => Container(
                        height: 280,
                        color: AppColors.cardBg,
                        child: const Icon(
                          Icons.broken_image,
                          color: AppColors.textMuted,
                          size: 48,
                        ),
                      ),
                    ),
                  ),
                const SizedBox(height: 20),
                GlassCard(
                  child: Column(
                    children: [
                      _buildRow('Type', event.displayType),
                      _buildRow(
                        'Time',
                        DateFormat.yMd().add_jm().format(
                          event.timestamp.toLocal(),
                        ),
                      ),
                      if (event.user != null) ...[
                        _buildRow('User', event.user!.name),
                        _buildRow('Family Member', event.familyMember!),
                        _buildRow('Email', event.user!.email),
                      ],
                    ],
                  ),
                ),
                if (event.eventType == Event.typeUnknownVisitor ||
                    event.eventType == Event.typeSpoofAttempt)
                  Padding(
                    padding: const EdgeInsets.only(top: 20),
                    child: SizedBox(
                      width: double.infinity,
                      child: FilledButton.icon(
                        onPressed: () {
                          context.read<CallProvider>().startCall();
                          Navigator.pushNamed(context, '/call');
                        },
                        icon: const Icon(Icons.videocam_rounded),
                        label: const Text('Start Call'),
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: const TextStyle(
              color: AppColors.textMuted,
              fontWeight: FontWeight.w500,
            ),
          ),
          Flexible(
            child: Text(
              value,
              textAlign: TextAlign.end,
              style: const TextStyle(
                color: AppColors.textPrimary,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
