import 'lib.dart';

// Newer and less common syntax: a flow through each of them must be found.

extension Wrap on String {
  String wrap() => '<$this>';
  String get twice => this + this;
}

extension Safe on String {
  String fixed() => 'constant';
}

class Holder {
  Holder(this.value);
  Holder.named(this.value);
  static Holder make(String v) => Holder(v);
  final String value;
}

void extensionMethod() {
  final s = source(); // @source(syn-ext)
  sink(s.wrap()); // @sink(syn-ext)
}

void extensionGetter() {
  final s = source(); // @source(syn-extget)
  sink(s.twice); // @sink(syn-extget)
}

void extensionOverride() {
  final s = source(); // @source(syn-override)
  sink(Wrap(s).wrap()); // @sink(syn-override)
}

void extensionThatDropsIt() {
  final s = source();
  sink(s.fixed()); // @sink(none)
}

void constructorTearOff() {
  final make = Holder.new;
  final h = make(source()); // @source(syn-ctor)
  sink(h.value); // @sink(syn-ctor)
}

void namedConstructorTearOff() {
  final make = Holder.named;
  final h = make(source()); // @source(syn-named)
  sink(h.value); // @sink(syn-named)
}

void dotShorthandStatic() {
  Holder h = .make(source()); // @source(syn-dot)
  sink(h.value); // @sink(syn-dot)
}

void dotShorthandConstructor() {
  Holder h = .new(source()); // @source(syn-dotnew)
  sink(h.value); // @sink(syn-dotnew)
}

void collectionForClassic() {
  final xs = [for (var i = 0; i < 2; i++) source()]; // @source(syn-for)
  sink(xs[0]); // @sink(syn-for)
}

void collectionForPattern() {
  final xs = [for (var (a, b) in [(source(), 1)]) a]; // @source(syn-forpat)
  sink(xs[0]); // @sink(syn-forpat)
}

void collectionForIdentifier() {
  String s;
  final out = [for (s in [source()]) s]; // @source(syn-forid)
  sink(out[0]); // @sink(syn-forid)
}

void collectionNested(bool a, bool b) {
  final s = source(); // @source(syn-nest)
  final xs = [
    if (a) if (b) s,
    for (var i = 0; i < 2; i++) if (a) 'x',
  ];
  sink(xs[0]); // @sink(syn-nest)
}

void collectionNullAware() {
  final s = source(); // @source(syn-nullaware)
  final xs = [?s];
  sink(xs[0]); // @sink(syn-nullaware)
}
