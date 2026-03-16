import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:provider/provider.dart';
import '../models/family_member.dart';
import '../providers/family_provider.dart';

class FamilyScreen extends StatefulWidget {
  const FamilyScreen({super.key});

  @override
  State<FamilyScreen> createState() => _FamilyScreenState();
}

class _FamilyScreenState extends State<FamilyScreen> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() => context.read<FamilyProvider>().fetchMembers());
  }

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<FamilyProvider>();

    return Scaffold(
      appBar: AppBar(
        title: const Text('Family Members'),
        actions: [
          IconButton(
            icon: const Icon(Icons.add),
            onPressed: () => _showAddMemberDialog(context),
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: () => context.read<FamilyProvider>().fetchMembers(),
        child: _buildBody(provider),
      ),
    );
  }

  Widget _buildBody(FamilyProvider provider) {
    if (provider.loading && provider.members.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    if (provider.error != null && provider.members.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('Error: ${provider.error}'),
            const SizedBox(height: 8),
            ElevatedButton(
              onPressed: () {
                provider.clearError();
                provider.fetchMembers();
              },
              child: const Text('Retry'),
            ),
          ],
        ),
      );
    }
    if (provider.members.isEmpty) {
      return const Center(
        child: Text(
          'No family members yet.\nTap + to add one.',
          textAlign: TextAlign.center,
        ),
      );
    }
    return ListView.separated(
      padding: const EdgeInsets.all(12),
      itemCount: provider.members.length,
      separatorBuilder: (_, __) => const SizedBox(height: 8),
      itemBuilder: (context, i) => _MemberCard(member: provider.members[i]),
    );
  }

  Future<void> _showAddMemberDialog(BuildContext context) async {
    final ctrl = TextEditingController();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Add Family Member'),
        content: TextField(
          controller: ctrl,
          decoration: const InputDecoration(labelText: 'Name'),
          textCapitalization: TextCapitalization.words,
          autofocus: true,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Add'),
          ),
        ],
      ),
    );

    if (confirmed != true || !context.mounted) return;
    final name = ctrl.text.trim();
    if (name.isEmpty) return;

    final provider = context.read<FamilyProvider>();
    final member = await provider.createMember(name);
    if (!context.mounted) return;

    if (member == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(provider.error ?? 'Failed to add member'),
          backgroundColor: Colors.red,
        ),
      );
      provider.clearError();
    }
  }
}

class _MemberCard extends StatefulWidget {
  final FamilyMember member;
  const _MemberCard({required this.member});

  @override
  State<_MemberCard> createState() => _MemberCardState();
}

class _MemberCardState extends State<_MemberCard> {
  bool _busy = false;

  @override
  Widget build(BuildContext context) {
    final m = widget.member;

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            // Avatar
            _buildAvatar(m),
            const SizedBox(width: 12),
            // Info
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(m.name, style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 2),
                  Row(
                    children: [
                      Icon(
                        m.faceEnrolled ? Icons.face : Icons.face_outlined,
                        size: 14,
                        color: m.faceEnrolled ? Colors.green : Colors.grey,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        m.faceEnrolled ? 'Face enrolled' : 'No face enrolled',
                        style: TextStyle(
                          fontSize: 12,
                          color: m.faceEnrolled ? Colors.green : Colors.grey,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            // Actions
            if (_busy)
              const SizedBox(
                width: 24,
                height: 24,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            else
              PopupMenuButton<String>(
                onSelected: (action) => _handleAction(context, action),
                itemBuilder: (_) => [
                  const PopupMenuItem(
                    value: 'enroll',
                    child: Text('Enroll / Update Face'),
                  ),
                  if (widget.member.faceEnrolled)
                    const PopupMenuItem(
                      value: 'unenroll',
                      child: Text('Remove Face'),
                    ),
                  const PopupMenuItem(
                    value: 'delete',
                    child: Text(
                      'Delete Member',
                      style: TextStyle(color: Colors.red),
                    ),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildAvatar(FamilyMember m) {
    if (m.photoUrl.isNotEmpty) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(24),
        child: CachedNetworkImage(
          imageUrl: m.photoUrl,
          width: 48,
          height: 48,
          fit: BoxFit.cover,
          placeholder: (_, __) =>
              const CircleAvatar(radius: 24, child: Icon(Icons.person)),
          errorWidget: (_, __, ___) =>
              const CircleAvatar(radius: 24, child: Icon(Icons.person)),
        ),
      );
    }
    return const CircleAvatar(radius: 24, child: Icon(Icons.person));
  }

  Future<void> _handleAction(BuildContext context, String action) async {
    switch (action) {
      case 'enroll':
        await _enrollFace(context);
        break;
      case 'unenroll':
        await _unenrollFace(context);
        break;
      case 'delete':
        await _deleteMember(context);
        break;
    }
  }

  Future<void> _enrollFace(BuildContext context) async {
    final picker = ImagePicker();
    final choice = await showModalBottomSheet<ImageSource>(
      context: context,
      builder: (_) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.camera_alt),
              title: const Text('Take photo'),
              onTap: () => Navigator.pop(context, ImageSource.camera),
            ),
            ListTile(
              leading: const Icon(Icons.photo_library),
              title: const Text('Choose from gallery'),
              onTap: () => Navigator.pop(context, ImageSource.gallery),
            ),
          ],
        ),
      ),
    );

    if (choice == null || !context.mounted) return;

    final XFile? picked = await picker.pickImage(
      source: choice,
      imageQuality: 90,
      maxWidth: 1024,
      maxHeight: 1024,
    );
    if (picked == null || !context.mounted) return;

    setState(() => _busy = true);
    try {
      final bytes = await picked.readAsBytes();
      final provider = context.read<FamilyProvider>();
      final errMsg = await provider.enrollFace(
        widget.member.id,
        bytes,
        picked.name,
      );
      if (!context.mounted) return;

      if (errMsg != null) {
        // Strip "Exception: " prefix for cleaner display
        final clean = errMsg.replaceFirst('Exception: ', '');
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(clean), backgroundColor: Colors.red),
        );
      } else {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('${widget.member.name} enrolled successfully'),
            backgroundColor: Colors.green,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _unenrollFace(BuildContext context) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Remove Face Enrollment'),
        content: Text('Remove face recognition for ${widget.member.name}?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            child: const Text('Remove'),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;

    setState(() => _busy = true);
    try {
      final errMsg = await context.read<FamilyProvider>().unenrollFace(
        widget.member.id,
      );
      if (!context.mounted) return;
      if (errMsg != null) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(errMsg.replaceFirst('Exception: ', '')),
            backgroundColor: Colors.red,
          ),
        );
      } else {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Face enrollment removed')),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _deleteMember(BuildContext context) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete Member'),
        content: Text(
          'Delete ${widget.member.name}? This will also remove their face enrollment.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;

    setState(() => _busy = true);
    try {
      final ok = await context.read<FamilyProvider>().deleteMember(
        widget.member.id,
      );
      if (!context.mounted) return;
      if (!ok) {
        final err = context.read<FamilyProvider>().error ?? 'Delete failed';
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(err.replaceFirst('Exception: ', '')),
            backgroundColor: Colors.red,
          ),
        );
        context.read<FamilyProvider>().clearError();
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}
