import 'package:shared_preferences/shared_preferences.dart';

class AppConfig {
  static const _keyBaseUrl = 'backend_base_url';
  static const defaultBaseUrl = 'http://172.23.26.147:8080';

  String baseUrl;

  AppConfig({String? baseUrl}) : baseUrl = baseUrl ?? defaultBaseUrl;

  String get wsUrl {
    final uri = Uri.parse(baseUrl);
    return 'ws://${uri.host}:${uri.port}/ws/signaling?role=owner';
  }

  Map<String, dynamic> get iceServers => {
    'iceServers': [
      {'urls': 'stun:stun.l.google.com:19302'},
    ],
  };

  static Future<AppConfig> load() async {
    final prefs = await SharedPreferences.getInstance();
    final url = prefs.getString(_keyBaseUrl) ?? defaultBaseUrl;
    return AppConfig(baseUrl: url);
  }

  Future<void> save() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_keyBaseUrl, baseUrl);
  }
}
