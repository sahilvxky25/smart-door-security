import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../models/user.dart';
import '../services/api_service.dart';

class AuthProvider extends ChangeNotifier {
  User? _user;
  String? _token;
  bool _loading = true; // true on startup until loadFromStorage completes
  String? _error;

  User? get user => _user;
  bool get isLoading => _loading;
  bool get isAuthenticated => _token != null;
  String? get error => _error;

  /// Called once at startup – restores session from SharedPreferences.
  Future<void> loadFromStorage(ApiService api) async {
    final prefs = await SharedPreferences.getInstance();
    final token = prefs.getString('auth_token');
    final id = prefs.getInt('user_id');
    final name = prefs.getString('user_name') ?? '';
    final email = prefs.getString('user_email') ?? '';
    final familyMemberName = prefs.getString('user_family_member_name');
    final photoUrl = prefs.getString('user_photo_url');
    if (token != null && id != null) {
      _token = token;
      _user = User(
        id: id,
        name: name,
        email: email,
        familyMemberName: familyMemberName,
        photoUrl: photoUrl,
        createdAt: DateTime.now(),
      );
      api.token = token;
    }
    _loading = false;
    notifyListeners();
  }

  Future<bool> signIn(ApiService api, String name, String password) async {
    _error = null;
    notifyListeners();
    try {
      final result = await api.signIn(name, password);
      _token = result['token'] as String;
      final userJson = Map<String, dynamic>.from(
        result['user'] as Map<String, dynamic>,
      );
      userJson['family_member_name'] ??=
          result['family_member_name'] ?? result['family_member'];
      _user = User.fromJson(userJson);
      api.token = _token!;
      await _save();
      notifyListeners();
      return true;
    } catch (e) {
      _error = e.toString().replaceFirst('Exception: ', '');
      notifyListeners();
      return false;
    }
  }

  Future<bool> signUp(
    ApiService api,
    String name,
    String email,
    String password,
  ) async {
    _error = null;
    notifyListeners();
    try {
      final result = await api.signUp(name, email, password);
      _token = result['token'] as String;
      final userJson = Map<String, dynamic>.from(
        result['user'] as Map<String, dynamic>,
      );
      userJson['family_member_name'] ??=
          result['family_member_name'] ?? result['family_member'];
      _user = User.fromJson(userJson);
      api.token = _token!;
      await _save();
      notifyListeners();
      return true;
    } catch (e) {
      _error = e.toString().replaceFirst('Exception: ', '');
      notifyListeners();
      return false;
    }
  }

  /// Update the local user's photo URL after a successful upload.
  void updatePhotoUrl(String url) {
    if (_user == null) return;
    _user = User(
      id: _user!.id,
      name: _user!.name,
      email: _user!.email,
      familyMemberName: _user!.familyMemberName,
      photoUrl: url,
      createdAt: _user!.createdAt,
    );
    _save(); // persist photo URL to survive app restarts
    notifyListeners();
  }

  Future<void> signOut(ApiService api) async {
    _token = null;
    _user = null;
    _error = null;
    api.token = null;
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('auth_token');
    await prefs.remove('user_id');
    await prefs.remove('user_name');
    await prefs.remove('user_email');
    await prefs.remove('user_family_member_name');
    await prefs.remove('user_photo_url');
    notifyListeners();
  }

  Future<void> signOutToLogin(
    ApiService api,
    GlobalKey<NavigatorState> navigatorKey,
  ) async {
    await signOut(api);
    navigatorKey.currentState?.pushNamedAndRemoveUntil('/login', (_) => false);
  }

  Future<void> _save() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('auth_token', _token!);
    await prefs.setInt('user_id', _user!.id);
    await prefs.setString('user_name', _user!.name);
    await prefs.setString('user_email', _user!.email);
    if (_user!.familyMemberName != null &&
        _user!.familyMemberName!.isNotEmpty) {
      await prefs.setString(
        'user_family_member_name',
        _user!.familyMemberName!,
      );
    } else {
      await prefs.remove('user_family_member_name');
    }
    if (_user!.photoUrl != null) {
      await prefs.setString('user_photo_url', _user!.photoUrl!);
    } else {
      await prefs.remove('user_photo_url');
    }
  }
}
