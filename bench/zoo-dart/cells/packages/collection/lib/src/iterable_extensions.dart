// A skeleton of package:collection, made from its public signatures (collection 1.19). The harness does not run it:
// exec/pubspec.yaml overrides `collection` with the real package. The bodies are empty on purpose: what the functions do
// with the function they are given is the built-in rows of package:collection.
extension IterableExtension<T> on Iterable<T> {
  T? firstWhereOrNull(bool Function(T element) test) => null;
  T? lastWhereOrNull(bool Function(T element) test) => null;
  T? singleWhereOrNull(bool Function(T element) test) => null;
  Iterable<T> whereNot(bool Function(T element) test) => this;
  bool none(bool Function(T) test) => false;
}
