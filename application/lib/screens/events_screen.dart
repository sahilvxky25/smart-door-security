import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../config/app_theme.dart';
import '../providers/event_provider.dart';
import '../widgets/event_tile.dart';

class EventsScreen extends StatefulWidget {
  const EventsScreen({super.key});

  @override
  State<EventsScreen> createState() => _EventsScreenState();
}

class _EventsScreenState extends State<EventsScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback(
      (_) => context.read<EventProvider>().fetchEvents(),
    );
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // If a WS event arrived while this screen is open, auto-refresh
    final ep = context.watch<EventProvider>();
    if (ep.hasNewEvent) {
      Future.microtask(() {
        if (mounted) ep.clearNewEvent();
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      extendBodyBehindAppBar: true,
      appBar: AppBar(title: const Text('Events')),
      body: GradientBackground(
        child: SafeArea(
          child: RefreshIndicator(
            color: AppColors.purple,
            onRefresh: () => context.read<EventProvider>().fetchEvents(),
            child: _buildList(),
          ),
        ),
      ),
    );
  }

  Widget _buildList() {
    final provider = context.watch<EventProvider>();

    if (provider.loading && provider.events.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (provider.error != null && provider.events.isEmpty) {
      return Center(
        child: Text(
          'Error: ${provider.error}',
          style: const TextStyle(color: AppColors.error),
        ),
      );
    }

    if (provider.events.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.history_rounded, size: 48, color: AppColors.textMuted),
            const SizedBox(height: 12),
            const Text('No events yet',
                style: TextStyle(color: AppColors.textMuted)),
          ],
        ),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      itemCount: provider.events.length,
      itemBuilder: (context, i) {
        final event = provider.events[i];
        return Padding(
          padding: const EdgeInsets.only(bottom: 8),
          child: GlassCard(
            padding: const EdgeInsets.symmetric(vertical: 4, horizontal: 4),
            child: EventTile(
              event: event,
              onTap: () => Navigator.pushNamed(context, '/event',
                  arguments: event),
            ),
          ),
        );
      },
    );
  }
}
