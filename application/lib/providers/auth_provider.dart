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
    if (token != null && id != null) {
      _token = token;
      _user = User(id: id, name: name, email: email, createdAt: DateTime.now());
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
      _user = User.fromJson(result['user'] as Map<String, dynamic>);
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
      _user = User.fromJson(result['user'] as Map<String, dynamic>);
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
    notifyListeners();
  }

  Future<void> _save() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('auth_token', _token!);
    await prefs.setInt('user_id', _user!.id);
    await prefs.setString('user_name', _user!.name);
    await prefs.setString('user_email', _user!.email);
  }
}
