// The skeleton of the parts of Flutter that the zoo's cells need, in a few lines each (the framework is not
// in the zoo, docs/DESIGN-zoo.md 3.4): what the user gives to an app (the text of a field, the arguments of
// a route, the message of a platform channel) and the state of a widget. It is real code, not an
// declaration: the harness runs it and rivus analyses it. It has the shape of the framework's API, not its
// behaviour.
import 'dart:async';

/// The text that the user typed into a field: the state that `TextField` and the app share.
class TextEditingController {
  String _text = '';
  String get text => _text;
  set text(String value) {
    _text = value;
  }
}

/// A route and what the caller gave to it.
class RouteSettings {
  const RouteSettings({this.name, this.arguments});
  final String? name;
  final Object? arguments;
}

typedef RouteBuilder = void Function(RouteSettings settings);

/// Navigator: `pushNamed(name, arguments: ...)` builds the route of the name with the settings.
class Navigator {
  final Map<String, RouteBuilder> routes = {};
  void pushNamed(String name, {Object? arguments}) {
    routes[name]!(RouteSettings(name: name, arguments: arguments));
  }
}

/// A call from the platform (the other side of a MethodChannel).
class MethodCall {
  const MethodCall(this.method, [this.arguments]);
  final String method;
  final Object? arguments;
}

/// What an app implements to answer the platform (the handler of a MethodChannel).
abstract class MethodCallHandler {
  Future<Object?> onMethodCall(MethodCall call);
}

class MethodChannel {
  MethodChannel(this.name);
  final String name;
  MethodCallHandler? _handler;
  void setMethodCallHandler(MethodCallHandler handler) {
    _handler = handler;
  }

  /// The platform calls the app.
  Future<Object?> invokeFromPlatform(MethodCall call) => _handler!.onMethodCall(call);
}

/// The state of a widget: `setState` runs its closure and (in the framework) builds again.
abstract class State {
  void initState() {}
  void dispose() {}
  void build();
  void setState(void Function() fn) {
    fn();
  }
}
