import 'package:fixnum/fixnum.dart';
import 'package:shared/shared.dart';

String run(List<String> xs) {
  final n = Int64.parseInt(sharedLast(xs));
  return n.toString();
}
