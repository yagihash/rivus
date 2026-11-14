// The shape of code a code generator leaves: a class with a part file.
part 'user.g.dart';

class User {
  User({required this.name});

  final String name;

  factory User.fromJson(Map<String, dynamic> json) => _$UserFromJson(json);

  Map<String, dynamic> toJson() => _$UserToJson(this);
}
