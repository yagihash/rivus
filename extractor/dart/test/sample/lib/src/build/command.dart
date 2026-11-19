import '../../lib.dart';

// #36: a directory named build deep in the path is code of the project.
void inBuildDirectory() {
  final s = source(); // @source(is-build-dir)
  sink(s); // @sink(is-build-dir)
}
