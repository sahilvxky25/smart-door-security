import 'package:flutter/foundation.dart';
import '../models/event.dart';
import '../services/api_service.dart';

class EventProvider extends ChangeNotifier {
  final ApiService _api;

  List<Event> events = [];
  bool loading = false;
  String? error;

  /// True when a new event arrived via WebSocket and hasn't been seen yet.
  bool hasNewEvent = false;

  EventProvider({required ApiService api}) : _api = api;

  /// Fetch events triggered by the user (pull-to-refresh, initState).
  /// Does NOT set hasNewEvent.
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

  /// Fetch events triggered by a WebSocket event_update or alert.
  /// Sets hasNewEvent = true so the dashboard can show a badge.
  Future<void> fetchEventsFromWs() async {
    try {
      events = await _api.getEvents();
      hasNewEvent = true;
    } catch (_) {
      // Silently swallow WS-triggered refresh errors
    }
    notifyListeners();
  }

  /// Call after the user has seen the new-event badge.
  void clearNewEvent() {
    if (!hasNewEvent) return;
    hasNewEvent = false;
    notifyListeners();
  }
}
