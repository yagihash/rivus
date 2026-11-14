// Builds the CFG of every function of a Dart file and writes a summary, and
// optionally DOT files. Throwaway spike code.
import 'dart:convert';
import 'dart:io';

import 'package:dart_cfg_spike/analysis.dart';

Future<void> main(List<String> args) async {
  if (args.isEmpty) {
    stderr.writeln('usage: cfg.dart <file.dart> [--dot <dir>] [--json]');
    exit(2);
  }
  final file = File(args.first).absolute.path;
  String? dotDir;
  final i = args.indexOf('--dot');
  if (i >= 0) dotDir = args[i + 1];
  final json = args.contains('--json');

  final cfgs = await cfgsOf(file);
  final out = <Map<String, Object?>>[];
  for (final MapEntry(key: name, value: cfg) in cfgs.entries) {
    final reach = cfg.reachable;
    final dead = cfg.blocks.where((b) => !reach.contains(b)).length;
    final stuck = cfg.reachable.difference(cfg.reachExit).length;
    out.add({
      'function': name,
      'blocks': cfg.blocks.length,
      'edges': cfg.edges.length,
      'edgeKinds': {for (final e in cfg.edgeCounts.entries) e.key.name: e.value},
      'unreachableBlocks': dead,
      'reachableBlocksThatCannotReachExit': stuck,
    });
    if (dotDir != null) {
      Directory(dotDir).createSync(recursive: true);
      File('$dotDir/${name.replaceAll('.', '_')}.dot').writeAsStringSync(cfg.toDot());
    }
  }
  if (json) {
    print(const JsonEncoder.withIndent('  ').convert(out));
  } else {
    for (final f in out) {
      print('${f['function']}: ${f['blocks']} blocks, ${f['edges']} edges ${f['edgeKinds']}'
          '${(f['unreachableBlocks'] as int) > 0 ? ' [${f['unreachableBlocks']} unreachable]' : ''}'
          '${(f['reachableBlocksThatCannotReachExit'] as int) > 0 ? ' [${f['reachableBlocksThatCannotReachExit']} cannot reach exit]' : ''}');
    }
  }
}
