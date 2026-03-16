import 'package:shared_preferences/shared_preferences.dart';

class AppConfig {
  static const _keyBaseUrl = 'backend_base_url';
  static const defaultBaseUrl = 'http://172.29.160.1:8080';

  String _baseUrl;

  AppConfig({String? baseUrl}) : _baseUrl = baseUrl ?? defaultBaseUrl;

  String get baseUrl => _baseUrl;

  String get wsUrl {
    final uri = Uri.parse(_baseUrl);
    return 'ws://${uri.host}:${uri.port}/ws/signaling?role=owner';
  }

  Map<String, dynamic> get iceServers => {
    'iceServers': [
      {'urls': 'stun:stun.l.google.com:19302'},
    ],
  };

  set baseUrl(String url) {
    _baseUrl = url;
  }

  static Future<AppConfig> load() async {
    final prefs = await SharedPreferences.getInstance();
    final url = prefs.getString(_keyBaseUrl) ?? defaultBaseUrl;
    return AppConfig(baseUrl: url);
  }

  Future<void> save() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_keyBaseUrl, _baseUrl);
  }
}
