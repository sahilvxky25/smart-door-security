import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';
import '../providers/event_provider.dart';
import '../providers/signaling_provider.dart';
import '../services/api_service.dart';
import '../widgets/connection_status.dart';
import '../widgets/door_control_card.dart';
import '../widgets/event_tile.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() => context.read<EventProvider>().fetchEvents());
  }

  Future<void> _signOut() async {
    final auth = context.read<AuthProvider>();
    final api = context.read<ApiService>();
    await auth.signOut(api);
    if (!mounted) return;
    Navigator.of(context).pushNamedAndRemoveUntil('/login', (_) => false);
  }

  Widget _buildDrawer(BuildContext context) {
    final auth = context.watch<AuthProvider>();
    final user = auth.user;
    final colorScheme = Theme.of(context).colorScheme;
    final currentRoute = ModalRoute.of(context)?.settings.name ?? '/';

    void go(String route) {
      Navigator.pop(context); // close drawer
      if (currentRoute != route) Navigator.pushNamed(context, route);
    }

    return Drawer(
      child: ListView(
        padding: EdgeInsets.zero,
        children: [
          DrawerHeader(
            decoration:
                BoxDecoration(color: colorScheme.primaryContainer),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                CircleAvatar(
                  radius: 28,
                  backgroundColor: colorScheme.primary,
                  child: Text(
                    user != null && user.name.isNotEmpty
                        ? user.name[0].toUpperCase()
                        : '?',
                    style: TextStyle(
                        fontSize: 22,
                        fontWeight: FontWeight.bold,
                        color: colorScheme.onPrimary),
                  ),
                ),
                const SizedBox(height: 10),
                Text(
                  user?.name ?? '',
                  style: TextStyle(
                      fontWeight: FontWeight.bold,
                      fontSize: 16,
                      color: colorScheme.onPrimaryContainer),
                ),
                Text(
                  user?.email ?? '',
                  style: TextStyle(
                      fontSize: 12,
                      color: colorScheme.onPrimaryContainer
                          .withValues(alpha: 0.7)),
                ),
              ],
            ),
          ),
          ListTile(
            leading: const Icon(Icons.home_outlined),
            title: const Text('Home'),
            selected: true,
            onTap: () => Navigator.pop(context),
          ),
          ListTile(
            leading: const Icon(Icons.event_note_outlined),
            title: const Text('Events'),
            onTap: () => go('/events'),
          ),
          ListTile(
            leading: const Icon(Icons.people_outline),
            title: const Text('Family Members'),
            onTap: () => go('/family'),
          ),
          ListTile(
            leading: const Icon(Icons.person_outline),
            title: const Text('Profile'),
            onTap: () => go('/profile'),
          ),
          ListTile(
            leading: const Icon(Icons.settings_outlined),
            title: const Text('Settings'),
            onTap: () => go('/settings'),
          ),
          const Divider(),
          ListTile(
            leading: const Icon(Icons.logout, color: Colors.red),
            title: const Text('Sign Out',
                style: TextStyle(color: Colors.red)),
            onTap: () {
              Navigator.pop(context);
              _signOut();
            },
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final signaling = context.watch<SignalingProvider>();

    // Auto-navigate to call screen on unknown visitor
    if (signaling.latestNotification != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) {
          Navigator.pushNamed(context, '/call');
        }
      });
    }

    return Scaffold(
      appBar: AppBar(
        title: const Text('Smart Door'),
        actions: [
          const Padding(
            padding: EdgeInsets.only(right: 8),
            child: ConnectionStatus(),
          ),
        ],
      ),
      drawer: _buildDrawer(context),
      body: RefreshIndicator(
        onRefresh: () => context.read<EventProvider>().fetchEvents(),
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            const DoorControlCard(),
            const SizedBox(height: 16),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  'Recent Events',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                TextButton(
                  onPressed: () => Navigator.pushNamed(context, '/events'),
                  child: const Text('View All'),
                ),
              ],
            ),
            const SizedBox(height: 8),
            _buildEventList(),
          ],
        ),
      ),
    );
  }

  Widget _buildEventList() {
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

    final recent = provider.events.take(5).toList();
    return Card(
      child: Column(
        children: recent
            .map(
              (e) => EventTile(
                event: e,
                onTap: () =>
                    Navigator.pushNamed(context, '/event', arguments: e),
              ),
            )
            .toList(),
      ),
    );
  }
}
