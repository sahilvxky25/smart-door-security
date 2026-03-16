import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
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
    Future.microtask(
        () => context.read<EventProvider>().fetchEvents());
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Events')),
      body: RefreshIndicator(
        onRefresh: () => context.read<EventProvider>().fetchEvents(),
        child: _buildList(),
      ),
    );
  }

  Widget _buildList() {
    final provider = context.watch<EventProvider>();

    if (provider.loading && provider.events.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (provider.error != null && provider.events.isEmpty) {
      return Center(child: Text('Error: ${provider.error}'));
    }

    if (provider.events.isEmpty) {
      return const Center(child: Text('No events yet'));
    }

    return ListView.builder(
      itemCount: provider.events.length,
      itemBuilder: (context, i) => EventTile(
        event: provider.events[i],
        onTap: () => Navigator.pushNamed(context, '/event',
            arguments: provider.events[i]),
      ),
    );
  }
}
