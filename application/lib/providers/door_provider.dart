import 'package:flutter/foundation.dart';
import '../services/api_service.dart';

class DoorProvider extends ChangeNotifier {
  final ApiService _api;

  bool unlocking = false;
  bool locking = false;
  String? lastStatus;

  DoorProvider({required ApiService api}) : _api = api;

  Future<void> unlock() async {
    unlocking = true;
    notifyListeners();

    try {
      lastStatus = await _api.unlockDoor();
    } catch (e) {
      lastStatus = 'Error: $e';
    }

    unlocking = false;
    notifyListeners();
  }

  Future<void> lock() async {
    locking = true;
    notifyListeners();

    try {
      lastStatus = await _api.lockDoor();
    } catch (e) {
      lastStatus = 'Error: $e';
    }

    locking = false;
    notifyListeners();
  }
}
