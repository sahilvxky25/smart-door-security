import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:provider/provider.dart';
import '../config/app_theme.dart';
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
    WidgetsBinding.instance.addPostFrameCallback(
      (_) => context.read<FamilyProvider>().fetchMembers(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<FamilyProvider>();

    return Scaffold(
      backgroundColor: Colors.transparent,
      extendBodyBehindAppBar: true,
      appBar: AppBar(
        title: const Text('Family Members'),
        actions: [
          IconButton(
            icon: const Icon(Icons.add_rounded),
            onPressed: () => _showAddMemberDialog(context),
          ),
        ],
      ),
      body: GradientBackground(
        child: SafeArea(
          child: RefreshIndicator(
            color: AppColors.purple,
            onRefresh: () => context.read<FamilyProvider>().fetchMembers(),
            child: _buildBody(provider),
          ),
        ),
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
            Text('Error: ${provider.error}',
                style: const TextStyle(color: AppColors.error)),
            const SizedBox(height: 8),
            FilledButton(
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
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.people_outline, size: 48, color: AppColors.textMuted),
            const SizedBox(height: 12),
            const Text('No family members yet.\nTap + to add one.',
                textAlign: TextAlign.center,
                style: TextStyle(color: AppColors.textMuted)),
          ],
        ),
      );
    }
    return ListView.separated(
      padding: const EdgeInsets.all(16),
      itemCount: provider.members.length,
      separatorBuilder: (_, _) => const SizedBox(height: 10),
      itemBuilder: (_, i) => _MemberCard(member: provider.members[i]),
    );
  }

  Future<void> _showAddMemberDialog(BuildContext context) async {
    final ctrl = TextEditingController();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Add Family Member',
            style: TextStyle(color: AppColors.textPrimary)),
        content: TextField(
          controller: ctrl,
          style: const TextStyle(color: AppColors.textPrimary),
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
          backgroundColor: AppColors.error,
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

    return GlassCard(
      padding: const EdgeInsets.all(16),
      child: Row(
        children: [
          _buildAvatar(m),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(m.name,
                    style: const TextStyle(
                        color: AppColors.textPrimary,
                        fontSize: 15,
                        fontWeight: FontWeight.w600)),
                const SizedBox(height: 4),
                Row(
                  children: [
                    Icon(
                      m.faceEnrolled
                          ? Icons.face_rounded
                          : Icons.face_outlined,
                      size: 14,
                      color: m.faceEnrolled
                          ? AppColors.success
                          : AppColors.textMuted,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      m.faceEnrolled ? 'Face enrolled' : 'No face enrolled',
                      style: TextStyle(
                        fontSize: 12,
                        color: m.faceEnrolled
                            ? AppColors.success
                            : AppColors.textMuted,
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
          if (_busy)
            const SizedBox(
              width: 24,
              height: 24,
              child: CircularProgressIndicator(strokeWidth: 2),
            )
          else
            PopupMenuButton<String>(
              icon: const Icon(Icons.more_vert, color: AppColors.textMuted),
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
                PopupMenuItem(
                  value: 'delete',
                  child: Text('Delete Member',
                      style: TextStyle(color: AppColors.error)),
                ),
              ],
            ),
        ],
      ),
    );
  }

  Widget _buildAvatar(FamilyMember m) {
    if (m.photoUrl.isNotEmpty) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(22),
        child: CachedNetworkImage(
          imageUrl: m.photoUrl,
          width: 44,
          height: 44,
          fit: BoxFit.cover,
          placeholder: (_, _) => _defaultAvatar(m),
          errorWidget: (_, _, _) => _defaultAvatar(m),
        ),
      );
    }
    return _defaultAvatar(m);
  }

  Widget _defaultAvatar(FamilyMember m) {
    return CircleAvatar(
      radius: 22,
      backgroundColor: AppColors.purpleSurface,
      child: Text(
        m.name.isNotEmpty ? m.name[0].toUpperCase() : '?',
        style: const TextStyle(
            color: AppColors.purple,
            fontWeight: FontWeight.bold,
            fontSize: 16),
      ),
    );
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
              leading: const Icon(Icons.camera_alt_rounded),
              title: const Text('Take photo'),
              onTap: () => Navigator.pop(context, ImageSource.camera),
            ),
            ListTile(
              leading: const Icon(Icons.photo_library_rounded),
              title: const Text('Choose from gallery'),
              onTap: () => Navigator.pop(context, ImageSource.gallery),
            ),
          ],
        ),
      ),
    );

    if (choice == null || !context.mounted) return;

    final picked = await picker.pickImage(
      source: choice,
      imageQuality: 90,
      maxWidth: 1024,
      maxHeight: 1024,
    );
    if (picked == null || !context.mounted) return;

    setState(() => _busy = true);
    // Capture provider before async gap.
    final provider = context.read<FamilyProvider>();
    try {
      final bytes = await picked.readAsBytes();
      final errMsg = await provider.enrollFace(
        widget.member.id,
        bytes,
        picked.name,
      );
      if (!context.mounted) return;

      if (errMsg != null) {
        final clean = errMsg.replaceFirst('Exception: ', '');
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(clean), backgroundColor: AppColors.error),
        );
      } else {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('${widget.member.name} enrolled successfully'),
            backgroundColor: AppColors.success,
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
        title: const Text('Remove Face',
            style: TextStyle(color: AppColors.textPrimary)),
        content: Text('Remove face recognition for ${widget.member.name}?',
            style: const TextStyle(color: AppColors.textSecondary)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: FilledButton.styleFrom(backgroundColor: AppColors.error),
            child: const Text('Remove'),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;

    setState(() => _busy = true);
    try {
      final errMsg =
          await context.read<FamilyProvider>().unenrollFace(widget.member.id);
      if (!context.mounted) return;
      if (errMsg != null) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(errMsg.replaceFirst('Exception: ', '')),
            backgroundColor: AppColors.error,
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
        title: const Text('Delete Member',
            style: TextStyle(color: AppColors.textPrimary)),
        content: Text(
          'Delete ${widget.member.name}? This will also remove their face enrollment.',
          style: const TextStyle(color: AppColors.textSecondary),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: FilledButton.styleFrom(backgroundColor: AppColors.error),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;

    setState(() => _busy = true);
    // Capture provider before async gap.
    final familyProvider = context.read<FamilyProvider>();
    try {
      final ok = await familyProvider.deleteMember(widget.member.id);
      if (!context.mounted) return;
      if (!ok) {
        final err = familyProvider.error ?? 'Delete failed';
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(err.replaceFirst('Exception: ', '')),
            backgroundColor: AppColors.error,
          ),
        );
        familyProvider.clearError();
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}
