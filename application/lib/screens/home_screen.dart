import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../config/app_theme.dart';
import '../providers/auth_provider.dart';
import '../providers/call_provider.dart';
import '../providers/door_provider.dart';
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
    WidgetsBinding.instance.addPostFrameCallback(
      (_) => context.read<EventProvider>().fetchEvents(),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      extendBodyBehindAppBar: true,
      appBar: AppBar(
        title: const Text('Smart Door'),
        actions: const [
          Padding(
            padding: EdgeInsets.only(right: 16),
            child: ConnectionStatus(),
          ),
        ],
      ),
      drawer: _buildDrawer(context),
      body: GradientBackground(
        child: SafeArea(
          child: RefreshIndicator(
            color: AppColors.purple,
            onRefresh: () => context.read<EventProvider>().fetchEvents(),
            child: ListView(
              padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
              children: [
                // ── Status Row ──
                _StatusRow(),
                const SizedBox(height: 20),

                // ── Door Control ──
                const DoorControlCard(),
                const SizedBox(height: 20),

                // ── Quick Actions ──
                _QuickActions(),
                const SizedBox(height: 24),

    // ── Recent Events ──
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Row(
                      children: [
                        const Text(
                          'Recent Events',
                          style: TextStyle(
                            color: AppColors.textPrimary,
                            fontSize: 17,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        // Live badge — appears when a new event arrives via WebSocket
                        Consumer<EventProvider>(
                          builder: (_, ep, _) {
                            if (!ep.hasNewEvent) return const SizedBox.shrink();
                            return GestureDetector(
                              onTap: ep.clearNewEvent,
                              child: Container(
                                margin: const EdgeInsets.only(left: 8),
                                padding: const EdgeInsets.symmetric(
                                    horizontal: 7, vertical: 2),
                                decoration: BoxDecoration(
                                  color: AppColors.purple,
                                  borderRadius: BorderRadius.circular(10),
                                ),
                                child: const Text(
                                  'NEW',
                                  style: TextStyle(
                                    color: Colors.white,
                                    fontSize: 10,
                                    fontWeight: FontWeight.w700,
                                    letterSpacing: 0.5,
                                  ),
                                ),
                              ),
                            );
                          },
                        ),
                      ],
                    ),
                    TextButton(
                      onPressed: () => Navigator.pushNamed(context, '/events'),
                      child: const Text('See All'),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                _RecentEvents(),
                const SizedBox(height: 24),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildDrawer(BuildContext context) {
    final auth = context.watch<AuthProvider>();
    return Drawer(
      child: Column(
        children: [
          DrawerHeader(
            decoration: const BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [Color(0xFF1A0A2E), Color(0xFF0A0A14)],
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                CircleAvatar(
                  radius: 28,
                  backgroundColor: AppColors.purpleSurface,
                  backgroundImage: auth.user?.photoUrl != null &&
                          auth.user!.photoUrl!.isNotEmpty
                      ? NetworkImage(auth.user!.photoUrl!)
                      : null,
                  child: auth.user?.photoUrl == null ||
                          auth.user!.photoUrl!.isEmpty
                      ? Text(
                          auth.user?.name.isNotEmpty == true
                              ? auth.user!.name[0].toUpperCase()
                              : '?',
                          style: const TextStyle(
                            color: AppColors.purple,
                            fontSize: 24,
                            fontWeight: FontWeight.bold,
                          ),
                        )
                      : null,
                ),
                const SizedBox(height: 12),
                Text(
                  auth.user?.name ?? 'User',
                  style: const TextStyle(
                    color: AppColors.textPrimary,
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                Text(
                  auth.user?.email ?? '',
                  style: const TextStyle(
                    color: AppColors.textMuted,
                    fontSize: 13,
                  ),
                ),
                if ((auth.user?.familyMemberName ?? '').isNotEmpty)
                  Text(
                    'Family Member: ${auth.user!.familyMemberName}',
                    style: const TextStyle(
                      color: AppColors.textMuted,
                      fontSize: 12,
                    ),
                  ),
              ],
            ),
          ),
          _drawerItem(Icons.home_rounded, 'Dashboard', () => Navigator.pop(context)),
          _drawerItem(Icons.history, 'Events', () {
            Navigator.pop(context);
            Navigator.pushNamed(context, '/events');
          }),
          _drawerItem(Icons.people_outline, 'Family', () {
            Navigator.pop(context);
            Navigator.pushNamed(context, '/family');
          }),
          _drawerItem(Icons.person_outline, 'Profile', () {
            Navigator.pop(context);
            Navigator.pushNamed(context, '/profile');
          }),
          const Spacer(),
          _drawerItem(Icons.settings_outlined, 'Settings', () {
            Navigator.pop(context);
            Navigator.pushNamed(context, '/settings');
          }),
          _drawerItem(
            Icons.logout_rounded,
            'Sign Out',
            () {
              // Disconnect WebSocket before signing out
              context.read<SignalingProvider>().disconnect();
              auth.signOut(context.read<ApiService>());
              Navigator.pop(context);
            },
          ),
          const SizedBox(height: 16),
        ],
      ),
    );
  }

  Widget _drawerItem(IconData icon, String label, VoidCallback onTap) {
    return ListTile(
      leading: Icon(icon, color: AppColors.textSecondary, size: 22),
      title: Text(label, style: const TextStyle(color: AppColors.textPrimary)),
      onTap: onTap,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      contentPadding: const EdgeInsets.symmetric(horizontal: 24),
    );
  }
}

// ── Status Row ─────────────────────────────────────────────────────────────
class _StatusRow extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final connected = context.watch<SignalingProvider>().connected;
    final door = context.watch<DoorProvider>();

    return Row(
      children: [
        Expanded(
          child: _StatusTile(
            icon: Icons.wifi,
            label: 'Connection',
            value: connected ? 'Online' : 'Offline',
            color: connected ? AppColors.success : AppColors.error,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: _StatusTile(
            icon: door.lastStatus?.contains('unlock') == true
                ? Icons.lock_open_rounded
                : Icons.lock_rounded,
            label: 'Door',
            value: door.lastStatus?.contains('unlock') == true
                ? 'Unlocked'
                : 'Locked',
            color: door.lastStatus?.contains('unlock') == true
                ? AppColors.warning
                : AppColors.success,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Consumer<EventProvider>(
            builder: (_, ep, _) => _StatusTile(
              icon: Icons.shield_outlined,
              label: 'Events',
              value: '${ep.events.length}',
              color: AppColors.purple,
            ),
          ),
        ),
      ],
    );
  }
}

class _StatusTile extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;
  final Color color;

  const _StatusTile({
    required this.icon,
    required this.label,
    required this.value,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return GlassCard(
      padding: const EdgeInsets.all(14),
      child: Column(
        children: [
          Icon(icon, color: color, size: 22),
          const SizedBox(height: 8),
          Text(
            value,
            style: TextStyle(
              color: color,
              fontSize: 15,
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            label,
            style: const TextStyle(
              color: AppColors.textMuted,
              fontSize: 11,
            ),
          ),
        ],
      ),
    );
  }
}

// ── Quick Actions ───────────────────────────────────────────────────────────
class _QuickActions extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return GlassCard(
      padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 12),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
        children: [
          _ActionButton(
            icon: Icons.videocam_rounded,
            label: 'Call',
            onTap: () {
              context.read<CallProvider>().startCall();
              Navigator.pushNamed(context, '/call');
            },
          ),
          _ActionButton(
            icon: Icons.history_rounded,
            label: 'Events',
            onTap: () => Navigator.pushNamed(context, '/events'),
          ),
          _ActionButton(
            icon: Icons.people_rounded,
            label: 'Family',
            onTap: () => Navigator.pushNamed(context, '/family'),
          ),
          _ActionButton(
            icon: Icons.settings_rounded,
            label: 'Settings',
            onTap: () => Navigator.pushNamed(context, '/settings'),
          ),
        ],
      ),
    );
  }
}

class _ActionButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  const _ActionButton({
    required this.icon,
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
        borderRadius: BorderRadius.circular(14),
        child: Padding(
          padding: const EdgeInsets.all(4.0),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: AppColors.purpleSurface,
                  borderRadius: BorderRadius.circular(14),
                  border: Border.all(color: AppColors.glassBorder),
                ),
                child: Icon(icon, color: AppColors.purple, size: 22),
              ),
              const SizedBox(height: 6),
              Text(
                label,
                style: const TextStyle(
                  color: AppColors.textSecondary,
                  fontSize: 11,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ── Recent Events ──────────────────────────────────────────────────────────
class _RecentEvents extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final provider = context.watch<EventProvider>();

    if (provider.loading && provider.events.isEmpty) {
      return const Center(
        child: Padding(
          padding: EdgeInsets.all(32),
          child: CircularProgressIndicator(),
        ),
      );
    }

    if (provider.events.isEmpty) {
      return GlassCard(
        child: Center(
          child: Text(
            'No events yet',
            style: TextStyle(color: AppColors.textMuted),
          ),
        ),
      );
    }

    final recent = provider.events.take(5).toList();
    return GlassCard(
      padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 4),
      child: Column(
        children: [
          for (int i = 0; i < recent.length; i++) ...[
            EventTile(
              event: recent[i],
              onTap: () => Navigator.pushNamed(
                context,
                '/event',
                arguments: recent[i],
              ),
            ),
            if (i < recent.length - 1)
              Divider(
                height: 1,
                indent: 56,
                color: AppColors.glassBorder,
              ),
          ],
        ],
      ),
    );
  }
}
