import 'dart:typed_data';
import 'package:flutter/foundation.dart';
import '../models/family_member.dart';
import '../services/api_service.dart';

class FamilyProvider extends ChangeNotifier {
  final ApiService _api;

  List<FamilyMember> members = [];
  bool loading = false;
  String? error;

  FamilyProvider({required ApiService api}) : _api = api;

  Future<void> fetchMembers() async {
    loading = true;
    error = null;
    notifyListeners();
    try {
      members = await _api.getFamilyMembers();
    } catch (e) {
      error = e.toString();
    } finally {
      loading = false;
      notifyListeners();
    }
  }

  Future<FamilyMember?> createMember(String name) async {
    try {
      final m = await _api.createFamilyMember(name);
      members = [m, ...members];
      notifyListeners();
      return m;
    } catch (e) {
      error = e.toString();
      notifyListeners();
      return null;
    }
  }

  Future<bool> deleteMember(int id) async {
    try {
      await _api.deleteFamilyMember(id);
      members = members.where((m) => m.id != id).toList();
      notifyListeners();
      return true;
    } catch (e) {
      error = e.toString();
      notifyListeners();
      return false;
    }
  }

  Future<String?> enrollFace(
    int id,
    Uint8List photoBytes,
    String filename,
  ) async {
    try {
      final updated = await _api.enrollFace(id, photoBytes, filename);
      _replaceMember(updated);
      return null; // success
    } catch (e) {
      return e.toString(); // return error message
    }
  }

  Future<String?> unenrollFace(int id) async {
    try {
      final updated = await _api.unenrollFace(id);
      _replaceMember(updated);
      return null;
    } catch (e) {
      return e.toString();
    }
  }

  void _replaceMember(FamilyMember updated) {
    members = [
      for (final m in members)
        if (m.id == updated.id) updated else m,
    ];
    notifyListeners();
  }

  void clearError() {
    error = null;
    notifyListeners();
  }
}
