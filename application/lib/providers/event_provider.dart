import 'package:flutter/foundation.dart';
import '../models/event.dart';
import '../services/api_service.dart';

class EventProvider extends ChangeNotifier {
  final ApiService _api;

  List<Event> events = [];
  bool loading = false;
  String? error;

  EventProvider({required ApiService api}) : _api = api;

  Future<void> fetchEvents() async {
    loading = true;
    error = null;
    notifyListeners();

    try {
      events = await _api.getEvents();
    } catch (e) {
      error = e.toString();
    }

    loading = false;
    notifyListeners();
  }
}
