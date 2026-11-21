// The app of the Dart zoo, for the entry points that a framework calls (ADR-073). The shape is a Flutter app: main hands
// the root widget to runApp, the framework, which does not call the app in the code that was read, so nothing that main
// reaches calls build, initState, dispose, or the closure of a button. The comment of each sink says
// what `reachability` its finding has (TestDartFrameworkCallbacks holds it).
import 'package:zoo_flutter/zoo_flutter.dart';
import 'package:zoo_mark/zoo_mark.dart';

void main() {
  sinkCmd(src('app-main')); // from_entry: main
  helper();
  runApp(const RootApp());
}

// reached from main through a call
void helper() {
  sinkCmd(src('app-helper')); // from_entry
}

class RootApp extends StatelessWidget {
  const RootApp();

  // the framework calls build (an override of StatelessWidget.build)
  @override
  Widget build(BuildContext context) {
    sinkCmd(src('app-build')); // from_framework_callback
    return TextButton(
      // the framework calls the closure on a press; it is made in build
      onPressed: () {
        sinkCmd(src('app-on-pressed')); // from_framework_callback
      },
    );
  }
}

// a state: the framework calls initState, build and dispose (a flow across two of them, through a field, is not found
// when nothing in the code that was read calls them in order: that is a different matter from where they start)
class PageState extends State {
  @override
  void initState() {
    sinkCmd(src('app-init-state')); // from_framework_callback
  }

  @override
  void build() {}

  @override
  void dispose() {
    sinkCmd(src('app-dispose')); // from_framework_callback
  }
}

// a method of the app that overrides a method of another class of the app: the base is in the code that was read, so
// nothing says that a framework calls it
class Base {
  void run() {}
}

class Child extends Base {
  @override
  void run() {
    sinkCmd(src('app-own-override')); // not_from_entry: nobody calls it
  }
}

// called by nothing, and not an override
void orphan() {
  sinkCmd(src('app-orphan')); // not_from_entry
}
