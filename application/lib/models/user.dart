class User {
  final int id;
  final String name;
  final String email;
  final String? familyMemberName;
  final String? photoUrl;
  final String? fcmToken;
  final DateTime createdAt;

  User({
    required this.id,
    required this.name,
    required this.email,
    this.familyMemberName,
    this.photoUrl,
    this.fcmToken,
    required this.createdAt,
  });

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      id: json['id'] as int,
      name: json['name'] as String,
      email: json['email'] as String,
      familyMemberName:
          (json['family_member_name'] ??
                  json['family_member'] ??
                  json['member_name'])
              as String?,
      photoUrl: json['photo_url'] as String?,
      fcmToken: json['fcm_token'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
