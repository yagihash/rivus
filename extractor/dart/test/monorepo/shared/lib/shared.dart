import 'package:collection/collection.dart';

/// Uses a pub dependency, so that a call from the app goes through two packages.
String sharedLast(List<String> xs) => xs.lastOrNull ?? '';
