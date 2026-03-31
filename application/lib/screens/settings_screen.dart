import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../config/app_theme.dart';
import '../config/app_config.dart';
import '../providers/auth_provider.dart';
import '../providers/signaling_provider.dart';
import '../services/api_service.dart';

class SettingsScreen extends StatefulWidget {
  final AppConfig config;

  const SettingsScreen({super.key, required this.config});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  late TextEditingController _urlController;
  bool _testing = false;
  String? _testResult;

  @override
  void initState() {
    super.initState();
    _urlController = TextEditingController(text: widget.config.baseUrl);
  }

  @override
  void dispose() {
    _urlController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      extendBodyBehindAppBar: true,
      appBar: AppBar(title: const Text('Settings')),
      body: GradientBackground(
        child: SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(20),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                GlassCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Container(
                            width: 36,
                            height: 36,
                            decoration: BoxDecoration(
                              color: AppColors.purpleSurface,
                              borderRadius: BorderRadius.circular(10),
                            ),
                            child: const Icon(Icons.dns_outlined,
                                color: AppColors.purple, size: 20),
                          ),
                          const SizedBox(width: 12),
                          const Text(
                            'Backend URL',
                            style: TextStyle(
                              color: AppColors.textPrimary,
                              fontSize: 16,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 16),
                      TextField(
                        controller: _urlController,
                        style: const TextStyle(color: AppColors.textPrimary),
                        decoration: const InputDecoration(
                          hintText: 'http://192.168.1.x:8080',
                        ),
                      ),
                      const SizedBox(height: 16),
                      Row(
                        children: [
                          Expanded(
                            child: FilledButton(
                              onPressed: _testing ? null : _testConnection,
                              child: _testing
                                  ? const SizedBox(
                                      width: 20,
                                      height: 20,
                                      child: CircularProgressIndicator(
                                          strokeWidth: 2,
                                          color: Colors.black),
                                    )
                                  : const Text('Test'),
                            ),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: FilledButton(
                              onPressed: _saveSettings,
                              child: const Text('Save'),
                            ),
                          ),
                        ],
                      ),
                      if (_testResult != null) ...[
                        const SizedBox(height: 16),
                        Container(
                          width: double.infinity,
                          padding: const EdgeInsets.all(12),
                          decoration: BoxDecoration(
                            color: _testResult!.contains('✓')
                                ? AppColors.success.withValues(alpha: 0.1)
                                : AppColors.error.withValues(alpha: 0.1),
                            borderRadius: BorderRadius.circular(12),
                            border: Border.all(
                              color: _testResult!.contains('✓')
                                  ? AppColors.success.withValues(alpha: 0.3)
                                  : AppColors.error.withValues(alpha: 0.3),
                            ),
                          ),
                          child: Text(
                            _testResult!,
                            style: TextStyle(
                              color: _testResult!.contains('✓')
                                  ? AppColors.success
                                  : AppColors.error,
                              fontSize: 13,
                            ),
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _testConnection() async {
    setState(() {
      _testing = true;
      _testResult = null;
    });
    try {
      final api = ApiService(baseUrl: _urlController.text.trim());
      final ok = await api.getHealth();
      setState(() {
        _testResult = ok ? '✓ Connection successful' : '✗ Connection failed';
      });
    } catch (e) {
      setState(() => _testResult = '✗ Error: $e');
    } finally {
      setState(() => _testing = false);
    }
  }

  Future<void> _saveSettings() async {
    widget.config.baseUrl = _urlController.text.trim();
    await widget.config.save();

    if (!mounted) return;

    // Dynamically update the running API Service
    context.read<ApiService>().baseUrl = widget.config.baseUrl;

    // Reactively refresh the connection if user is logged in
    if (context.read<AuthProvider>().isAuthenticated) {
      final signaling = context.read<SignalingProvider>();
      signaling.disconnect();
      signaling.connect(widget.config.wsUrl, userId: context.read<AuthProvider>().user?.id);
    }

    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Settings saved')),
      );
    }
  }
}
