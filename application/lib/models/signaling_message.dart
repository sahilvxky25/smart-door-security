class SignalingMessage {
  final String type;
  final String? eventType; // present when type == "alert"
  final String? title; // present when type == "alert"
  final String? body; // present when type == "alert"
  final String? imageUrl;
  final String? timestamp;
  final Map<String, dynamic>? sdp;
  final Map<String, dynamic>? candidate;

  SignalingMessage({
    required this.type,
    this.eventType,
    this.title,
    this.body,
    this.imageUrl,
    this.timestamp,
    this.sdp,
    this.candidate,
  });

  factory SignalingMessage.fromJson(Map<String, dynamic> json) {
    return SignalingMessage(
      type: json['type'] as String,
      eventType: json['event_type'] as String?,
      title: json['title'] as String?,
      body: json['body'] as String?,
      imageUrl: json['image_url'] as String?,
      timestamp: json['timestamp'] as String?,
      sdp: json['sdp'] as Map<String, dynamic>?,
      candidate: json['candidate'] as Map<String, dynamic>?,
    );
  }

  Map<String, dynamic> toJson() {
    final map = <String, dynamic>{'type': type};
    if (sdp != null) map['sdp'] = sdp;
    if (candidate != null) map['candidate'] = candidate;
    return map;
  }
}
