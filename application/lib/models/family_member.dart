class FamilyMember {
  final int id;
  final String name;
  final String photoUrl;
  final bool faceEnrolled;
  final DateTime createdAt;

  FamilyMember({
    required this.id,
    required this.name,
    required this.photoUrl,
    required this.faceEnrolled,
    required this.createdAt,
  });

  factory FamilyMember.fromJson(Map<String, dynamic> json) {
    return FamilyMember(
      id: json['id'] as int,
      name: json['name'] as String,
      photoUrl: (json['photo_url'] as String?) ?? '',
      faceEnrolled: (json['face_enrolled'] as bool?) ?? false,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
