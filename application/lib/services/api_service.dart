import 'dart:convert';
import 'dart:typed_data';
import 'package:http/http.dart' as http;
import 'package:http_parser/http_parser.dart';
import '../models/event.dart';
import '../models/family_member.dart';

class ApiService {
  String baseUrl;
  String? token;

  ApiService({required this.baseUrl});

  Map<String, String> get _authHeaders =>
      token != null ? {'Authorization': 'Bearer $token'} : {};

  Future<bool> getHealth() async {
    try {
      final resp = await http
          .get(Uri.parse('$baseUrl/health'))
          .timeout(const Duration(seconds: 5));
      return resp.statusCode == 200;
    } catch (_) {
      return false;
    }
  }

  // ── Auth ─────────────────────────────────────────────────────────────────

  Future<Map<String, dynamic>> signIn(String name, String password) async {
    final resp = await http.post(
      Uri.parse('$baseUrl/auth/signin'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'name': name, 'password': password}),
    );
    final body = jsonDecode(resp.body) as Map<String, dynamic>;
    if (resp.statusCode != 200) {
      throw Exception(body['error'] ?? 'Sign in failed');
    }
    return body;
  }

  Future<Map<String, dynamic>> signUp(
    String name,
    String email,
    String password,
  ) async {
    final resp = await http.post(
      Uri.parse('$baseUrl/auth/signup'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'name': name, 'email': email, 'password': password}),
    );
    final body = jsonDecode(resp.body) as Map<String, dynamic>;
    if (resp.statusCode != 201) {
      throw Exception(body['error'] ?? 'Sign up failed');
    }
    return body;
  }

  // ── Door ─────────────────────────────────────────────────────────────────

  Future<String> unlockDoor() async {
    final resp = await http.post(
      Uri.parse('$baseUrl/door/unlock'),
      headers: _authHeaders,
    );
    final body = jsonDecode(resp.body);
    return body['status'] as String;
  }

  Future<String> lockDoor() async {
    final resp = await http.post(
      Uri.parse('$baseUrl/door/lock'),
      headers: _authHeaders,
    );
    final body = jsonDecode(resp.body);
    return body['status'] as String;
  }

  Future<Map<String, dynamic>> getDoorState() async {
    final resp = await http.get(
      Uri.parse('$baseUrl/door/state'),
      headers: _authHeaders,
    );
    if (resp.statusCode != 200) {
      throw Exception('Failed to fetch door state');
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }

  // ── Events ───────────────────────────────────────────────────────────────

  Future<List<Event>> getEvents() async {
    final resp = await http.get(
      Uri.parse('$baseUrl/events'),
      headers: _authHeaders,
    );
    if (resp.statusCode != 200) {
      throw Exception('Failed to fetch events: ${resp.statusCode}');
    }
    final body = jsonDecode(resp.body);
    final list = body['events'] as List<dynamic>? ?? [];
    return list.map((e) => Event.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<Event> getEvent(int id) async {
    final resp = await http.get(
      Uri.parse('$baseUrl/events/$id'),
      headers: _authHeaders,
    );
    if (resp.statusCode != 200) {
      throw Exception('Event not found');
    }
    final body = jsonDecode(resp.body);
    return Event.fromJson(body['event'] as Map<String, dynamic>);
  }

  // ── Family members ───────────────────────────────────────────────────────

  Future<List<FamilyMember>> getFamilyMembers() async {
    final resp = await http.get(
      Uri.parse('$baseUrl/family'),
      headers: _authHeaders,
    );
    if (resp.statusCode != 200) {
      throw Exception('Failed to fetch family members: ${resp.statusCode}');
    }
    final body = jsonDecode(resp.body);
    final list = body['members'] as List<dynamic>? ?? [];
    return list
        .map((e) => FamilyMember.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<FamilyMember> createFamilyMember(String name) async {
    final resp = await http.post(
      Uri.parse('$baseUrl/family'),
      headers: {'Content-Type': 'application/json', ..._authHeaders},
      body: jsonEncode({'name': name}),
    );
    if (resp.statusCode == 409) {
      throw Exception('A member named "$name" already exists');
    }
    if (resp.statusCode != 201) {
      final body = jsonDecode(resp.body);
      throw Exception(body['error'] ?? 'Failed to create member');
    }
    final body = jsonDecode(resp.body);
    return FamilyMember.fromJson(body['member'] as Map<String, dynamic>);
  }

  Future<void> deleteFamilyMember(int id) async {
    final resp = await http.delete(
      Uri.parse('$baseUrl/family/$id'),
      headers: _authHeaders,
    );
    if (resp.statusCode != 200) {
      throw Exception('Failed to delete member');
    }
  }

  Future<FamilyMember> enrollFace(
    int id,
    Uint8List photoBytes,
    String filename,
  ) async {
    final request = http.MultipartRequest(
      'POST',
      Uri.parse('$baseUrl/family/$id/enroll'),
    );
    request.headers.addAll(_authHeaders);
    request.files.add(
      http.MultipartFile.fromBytes(
        'photo',
        photoBytes,
        filename: filename,
        contentType: MediaType('image', 'jpeg'),
      ),
    );
    final streamed = await request.send();
    final resp = await http.Response.fromStream(streamed);
    if (resp.statusCode == 422) {
      final body = jsonDecode(resp.body);
      throw Exception(body['error'] ?? 'Face enrollment failed');
    }
    if (resp.statusCode != 200) {
      final body = jsonDecode(resp.body);
      throw Exception(body['error'] ?? 'Failed to enroll face');
    }
    final body = jsonDecode(resp.body);
    return FamilyMember.fromJson(body['member'] as Map<String, dynamic>);
  }

  Future<FamilyMember> unenrollFace(int id) async {
    final resp = await http.delete(
      Uri.parse('$baseUrl/family/$id/enroll'),
      headers: _authHeaders,
    );
    if (resp.statusCode != 200) {
      throw Exception('Failed to unenroll face');
    }
    final body = jsonDecode(resp.body);
    return FamilyMember.fromJson(body['member'] as Map<String, dynamic>);
  }

  // ── Profile photo ─────────────────────────────────────────────────────────

  Future<String> uploadProfilePhoto(
    String userName,
    Uint8List photoBytes,
    String filename,
  ) async {
    final request = http.MultipartRequest(
      'POST',
      Uri.parse('$baseUrl/profile/photo?name=$userName'),
    );
    request.headers.addAll(_authHeaders);
    request.files.add(
      http.MultipartFile.fromBytes('photo', photoBytes, filename: filename),
    );
    final streamed = await request.send();
    final resp = await http.Response.fromStream(streamed);
    if (resp.statusCode != 200) {
      final body = jsonDecode(resp.body);
      throw Exception(body['error'] ?? 'Upload failed');
    }
    final body = jsonDecode(resp.body);
    return body['photo_url'] as String;
  }

  Future<void> updateFCMToken(String token) async {
    final resp = await http.post(
      Uri.parse('$baseUrl/users/fcm-token'),
      headers: {'Content-Type': 'application/json', ..._authHeaders},
      body: jsonEncode({'token': token}),
    );
    if (resp.statusCode != 200) {
      throw Exception('Failed to update FCM token');
    }
  }
}
