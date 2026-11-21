// @cell(F/dart/flutter-override-arg)
// @na(go: no such form; the framework calls the method that an app overrides)
// @shape: the source is the parameter of a method that an app overrides (the handler of a MethodChannel); the platform's message is given with typed(), which is not a source of the rules
import 'package:zoo_flutter/zoo_flutter.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/flutter-override-arg/vuln', vuln);
  zooCase('F/dart/flutter-override-arg/fixed', fixed);
  zooCase('F/dart/flutter-override-arg/decoy', decoy);
}

class OpenHandler extends MethodCallHandler {
  @override // @source(dart-flutter-override-arg-vuln)
  Future<Object?> onMethodCall(MethodCall call) async {
    sinkCmd(call.arguments as String); // @sink(dart-flutter-override-arg-vuln)
    return null;
  }
}

Future<void> vuln() async { // @vuln
  final channel = MethodChannel('open')..setMethodCallHandler(OpenHandler());
  await channel.invokeFromPlatform(MethodCall('open', typed('dart-flutter-override-arg-vuln')));
}

class OpenHandlerFixed extends MethodCallHandler {
  @override
  Future<Object?> onMethodCall(MethodCall call) async {
    sinkCmd(cleanCmd(call.arguments as String)); // @sink(none)
    return null;
  }
}

Future<void> fixed() async { // @fixed
  final channel = MethodChannel('open')..setMethodCallHandler(OpenHandlerFixed());
  await channel.invokeFromPlatform(MethodCall('open', typed('dart-flutter-override-arg-fixed')));
}

class OpenHandlerDecoy extends MethodCallHandler {
  @override // @source(dart-flutter-override-arg-decoy)
  Future<Object?> onMethodCall(MethodCall call) async {
    sinkCmd('constant'); // @sink(none) @nosink(dart-flutter-override-arg-decoy)
    sinkSql(call.arguments as String); // @sink(dart-flutter-override-arg-decoy)
    return null;
  }
}

Future<void> decoy() async { // @decoy
  final channel = MethodChannel('open')..setMethodCallHandler(OpenHandlerDecoy());
  await channel.invokeFromPlatform(MethodCall('open', typed('dart-flutter-override-arg-decoy')));
}
