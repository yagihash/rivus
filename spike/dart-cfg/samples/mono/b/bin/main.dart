import 'package:a/a.dart';

void run(Map<String, String> request, Map<String, dynamic> body) {
  final q = readQuery(request);
  final user = parseUser(body);
  print('$q ${user.name}');
}
