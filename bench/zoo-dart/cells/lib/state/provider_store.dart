// @cell(F/state/provider-store)
// @shape: a state store (as riverpod and provider have it): one widget writes a provider's state from its event handler, and another widget reads it in build (the decoy has a second provider)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/state/provider-store/vuln', vuln);
  zooCase('F/state/provider-store/fixed', fixed);
  zooCase('F/state/provider-store/decoy', decoy);
}

// the store: a provider is the key of a state that lives in the Ref (the container of the app)
class Provider<T> {
  Provider(this.init);
  final T Function() init;
}

class Ref {
  final Map<Object, Object?> _states = {};
  T read<T>(Provider<T> p) => (_states[p] ??= p.init()) as T;
  void write<T>(Provider<T> p, T value) {
    _states[p] = value;
  }
}

// the vulnerable one: the search box writes the query, the result list reads it
final queryProvider = Provider<String>(() => '');

class SearchBox {
  void onChanged(Ref ref, String text) {
    ref.write(queryProvider, text);
  }
}

class ResultList {
  void build(Ref ref) {
    sinkCmd(ref.read(queryProvider)); // @sink(state-provider-store-vuln)
  }
}

void vuln() { // @vuln
  final ref = Ref();
  SearchBox().onChanged(ref, src('state-provider-store-vuln')); // @source(state-provider-store-vuln)
  ResultList().build(ref);
}

final queryFixedProvider = Provider<String>(() => '');

class SearchBoxFixed {
  void onChanged(Ref ref, String text) {
    ref.write(queryFixedProvider, cleanCmd(text));
  }
}

class ResultListFixed {
  void build(Ref ref) {
    sinkCmd(ref.read(queryFixedProvider)); // @sink(none)
  }
}

void fixed() { // @fixed
  final ref = Ref();
  SearchBoxFixed().onChanged(ref, src('state-provider-store-fixed'));
  ResultListFixed().build(ref);
}

// the decoy: two providers; the tainted one is read by a different sink, the other one is what the quiet sink reads
final queryDecoyProvider = Provider<String>(() => '');
final titleDecoyProvider = Provider<String>(() => '');

class SearchBoxDecoy {
  void onChanged(Ref ref, String text) {
    ref.write(queryDecoyProvider, text);
    ref.write(titleDecoyProvider, 'constant');
  }
}

class ResultListDecoy {
  void build(Ref ref) {
    sinkCmd(ref.read(titleDecoyProvider)); // @sink(none) @fp @nosink(state-provider-store-decoy) @fp @ref(KEYED-MAP)
    sinkSql(ref.read(queryDecoyProvider)); // @sink(state-provider-store-decoy)
  }
}

void decoy() { // @decoy
  final ref = Ref();
  SearchBoxDecoy().onChanged(ref, src('state-provider-store-decoy')); // @source(state-provider-store-decoy)
  ResultListDecoy().build(ref);
}
