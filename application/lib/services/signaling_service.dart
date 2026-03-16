import 'dart:async';
import 'dart:convert';
import 'package:web_socket_channel/web_socket_channel.dart';
import '../models/signaling_message.dart';

class SignalingService {
  WebSocketChannel? _channel;
  Timer? _reconnectTimer;
  String? _wsUrl;
  bool _intentionalClose = false;

  final _messageController = StreamController<SignalingMessage>.broadcast();
  final _connectionController = StreamController<bool>.broadcast();

  Stream<SignalingMessage> get messages => _messageController.stream;
  Stream<bool> get connectionState => _connectionController.stream;
  bool get isConnected => _channel != null;

  void connect(String wsUrl) {
    _wsUrl = wsUrl;
    _intentionalClose = false;
    _doConnect();
  }

  void _doConnect() {
    if (_wsUrl == null) return;

    try {
      _channel = WebSocketChannel.connect(Uri.parse(_wsUrl!));
      _connectionController.add(true);

      _channel!.stream.listen(
        (data) {
          try {
            final json = jsonDecode(data as String) as Map<String, dynamic>;
            _messageController.add(SignalingMessage.fromJson(json));
          } catch (e) {
            // ignore malformed messages
          }
        },
        onDone: () {
          _connectionController.add(false);
          _channel = null;
          if (!_intentionalClose) {
            _scheduleReconnect();
          }
        },
        onError: (_) {
          _connectionController.add(false);
          _channel = null;
          if (!_intentionalClose) {
            _scheduleReconnect();
          }
        },
      );
    } catch (_) {
      _connectionController.add(false);
      _scheduleReconnect();
    }
  }

  void _scheduleReconnect() {
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(const Duration(seconds: 3), _doConnect);
  }

  void send(Map<String, dynamic> message) {
    _channel?.sink.add(jsonEncode(message));
  }

  void disconnect() {
    _intentionalClose = true;
    _reconnectTimer?.cancel();
    _channel?.sink.close();
    _channel = null;
    _connectionController.add(false);
  }

  void dispose() {
    disconnect();
    _messageController.close();
    _connectionController.close();
  }
}
