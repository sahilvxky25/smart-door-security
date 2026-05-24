import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../config/app_config.dart';
import '../config/app_theme.dart';
import '../providers/auth_provider.dart';
import '../providers/signaling_provider.dart';
import '../services/api_service.dart';

// ── Shared field validators ──────────────────────────────────────────────────
class _Validators {
  /// Username: 3–30 chars, letters / digits / underscores / dots / hyphens
  static String? username(String? v) {
    if (v == null || v.trim().isEmpty) return 'Name is required';
    final s = v.trim();
    if (s.length < 3) return 'Name must be at least 3 characters';
    if (s.length > 30) return 'Name must be 30 characters or fewer';
    if (!RegExp(r'^[a-zA-Z0-9._-]+$').hasMatch(s)) {
      return 'Only letters, digits, . _ - are allowed';
    }
    return null;
  }

  /// Email: standard RFC-5322-like pattern
  static String? email(String? v) {
    if (v == null || v.trim().isEmpty) return 'Email is required';
    final s = v.trim();
    if (!RegExp(
      r'^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$',
    ).hasMatch(s)) {
      return 'Enter a valid email address';
    }
    return null;
  }

  /// Password: 8+ chars, at least one uppercase, lowercase, digit, special char
  static String? password(String? v) {
    if (v == null || v.isEmpty) return 'Password is required';
    if (v.length < 8) return 'Minimum 8 characters';
    if (!RegExp(r'[A-Z]').hasMatch(v))
      return 'Include at least one uppercase letter';
    if (!RegExp(r'[a-z]').hasMatch(v))
      return 'Include at least one lowercase letter';
    if (!RegExp(r'[0-9]').hasMatch(v)) return 'Include at least one digit';
    if (!RegExp(r'[^a-zA-Z0-9]').hasMatch(v))
      return 'Include at least one special character';
    return null;
  }
}

class LoginScreen extends StatelessWidget {
  const LoginScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 2,
      child: Scaffold(
        backgroundColor: Colors.transparent,
        body: GradientBackground(
          child: SafeArea(
            child: Column(
              children: [
                const SizedBox(height: 56),
                // Shield icon with glow
                Container(
                  width: 80,
                  height: 80,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: AppColors.purpleSurface,
                    boxShadow: [
                      BoxShadow(
                        color: AppColors.purpleGlow,
                        blurRadius: 40,
                        spreadRadius: 4,
                      ),
                    ],
                  ),
                  child: const Icon(
                    Icons.shield_outlined,
                    size: 40,
                    color: AppColors.purple,
                  ),
                ),
                const SizedBox(height: 20),
                const Text(
                  'Smart Door',
                  style: TextStyle(
                    color: AppColors.textPrimary,
                    fontSize: 28,
                    fontWeight: FontWeight.bold,
                    letterSpacing: 0.5,
                  ),
                ),
                const SizedBox(height: 6),
                const Text(
                  'Secure access for your home',
                  style: TextStyle(color: AppColors.textMuted, fontSize: 14),
                ),
                const SizedBox(height: 36),
                const TabBar(
                  tabs: [
                    Tab(text: 'Sign In'),
                    Tab(text: 'Sign Up'),
                  ],
                  labelStyle: TextStyle(fontWeight: FontWeight.w600),
                ),
                Expanded(
                  child: TabBarView(children: [_SignInForm(), _SignUpForm()]),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _SignInForm extends StatefulWidget {
  @override
  State<_SignInForm> createState() => _SignInFormState();
}

class _SignInFormState extends State<_SignInForm> {
  final _formKey = GlobalKey<FormState>();
  final _nameCtrl = TextEditingController();
  final _passCtrl = TextEditingController();
  bool _obscure = true;
  bool _submitting = false;

  @override
  void dispose() {
    _nameCtrl.dispose();
    _passCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _submitting = true);
    final auth = context.read<AuthProvider>();
    final api = context.read<ApiService>();
    final ok = await auth.signIn(api, _nameCtrl.text.trim(), _passCtrl.text);
    if (!mounted) return;
    setState(() => _submitting = false);
    if (ok) {
      // Connect WebSocket and init FCM immediately after login
      final signaling = context.read<SignalingProvider>();
      final config = context.read<AppConfig>();
      signaling.connect(config.wsUrl, userId: auth.user?.id);
      signaling.initializeFCM(api);
      if (!mounted) return;
      Navigator.of(context).pushNamedAndRemoveUntil('/home', (_) => false);
    } else if (auth.error != null) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(auth.error!)));
    }
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 24),
      child: Form(
        key: _formKey,
        child: Column(
          children: [
            TextFormField(
              controller: _nameCtrl,
              decoration: const InputDecoration(
                labelText: 'Name',
                hintText: 'e.g. john_doe',
                prefixIcon: Icon(Icons.person_outline),
              ),
              style: const TextStyle(color: AppColors.textPrimary),
              textInputAction: TextInputAction.next,
              validator: _Validators.username,
            ),
            const SizedBox(height: 16),
            TextFormField(
              controller: _passCtrl,
              decoration: InputDecoration(
                labelText: 'Password',
                prefixIcon: const Icon(Icons.lock_outline),
                suffixIcon: IconButton(
                  icon: Icon(
                    _obscure ? Icons.visibility_off : Icons.visibility,
                  ),
                  onPressed: () => setState(() => _obscure = !_obscure),
                ),
              ),
              style: const TextStyle(color: AppColors.textPrimary),
              obscureText: _obscure,
              textInputAction: TextInputAction.done,
              onFieldSubmitted: (_) => _submit(),
              validator: (v) =>
                  (v == null || v.isEmpty) ? 'Password is required' : null,
            ),
            const SizedBox(height: 28),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: _submitting ? null : _submit,
                child: _submitting
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.black,
                        ),
                      )
                    : const Text('Sign In'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SignUpForm extends StatefulWidget {
  @override
  State<_SignUpForm> createState() => _SignUpFormState();
}

class _SignUpFormState extends State<_SignUpForm> {
  final _formKey = GlobalKey<FormState>();
  final _nameCtrl = TextEditingController();
  final _emailCtrl = TextEditingController();
  final _passCtrl = TextEditingController();
  bool _obscure = true;
  bool _submitting = false;

  @override
  void dispose() {
    _nameCtrl.dispose();
    _emailCtrl.dispose();
    _passCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _submitting = true);
    final auth = context.read<AuthProvider>();
    final api = context.read<ApiService>();
    final ok = await auth.signUp(
      api,
      _nameCtrl.text.trim(),
      _emailCtrl.text.trim(),
      _passCtrl.text,
    );
    if (!mounted) return;
    setState(() => _submitting = false);
    if (ok) {
      // Connect WebSocket and init FCM immediately after registration
      final signaling = context.read<SignalingProvider>();
      final config = context.read<AppConfig>();
      signaling.connect(config.wsUrl, userId: auth.user?.id);
      signaling.initializeFCM(api);
      if (!mounted) return;
      DefaultTabController.of(context).animateTo(0);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Registration successful. Please sign in.'),
        ),
      );
    } else if (auth.error != null) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(auth.error!)));
    }
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 24),
      child: Form(
        key: _formKey,
        child: Column(
          children: [
            TextFormField(
              controller: _nameCtrl,
              decoration: const InputDecoration(
                labelText: 'Name',
                hintText: 'e.g. john_doe',
                prefixIcon: Icon(Icons.person_outline),
              ),
              style: const TextStyle(color: AppColors.textPrimary),
              textInputAction: TextInputAction.next,
              validator: _Validators.username,
            ),
            const SizedBox(height: 16),
            TextFormField(
              controller: _emailCtrl,
              decoration: const InputDecoration(
                labelText: 'Email',
                prefixIcon: Icon(Icons.email_outlined),
              ),
              style: const TextStyle(color: AppColors.textPrimary),
              keyboardType: TextInputType.emailAddress,
              textInputAction: TextInputAction.next,
              validator: _Validators.email,
            ),
            const SizedBox(height: 16),
            TextFormField(
              controller: _passCtrl,
              decoration: InputDecoration(
                labelText: 'Password',
                hintText: 'Min 8 chars, A-Z, a-z, 0-9, symbol',
                prefixIcon: const Icon(Icons.lock_outline),
                suffixIcon: IconButton(
                  icon: Icon(
                    _obscure ? Icons.visibility_off : Icons.visibility,
                  ),
                  onPressed: () => setState(() => _obscure = !_obscure),
                ),
              ),
              style: const TextStyle(color: AppColors.textPrimary),
              obscureText: _obscure,
              textInputAction: TextInputAction.done,
              onFieldSubmitted: (_) => _submit(),
              validator: _Validators.password,
            ),
            const SizedBox(height: 28),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: _submitting ? null : _submit,
                child: _submitting
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.black,
                        ),
                      )
                    : const Text('Sign Up'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
