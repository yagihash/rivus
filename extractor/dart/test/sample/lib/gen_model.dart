import 'lib.dart';

part 'gen_model.g.dart';

void usesGenerated() {
  final s = source(); // @source(rv-generated)
  generatedSink(s);
}
