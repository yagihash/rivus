import 'package:dart_cfg_spike/analysis.dart';
import 'package:dart_cfg_spike/cfg.dart';
import 'package:test/test.dart';

void main() {
  late Map<String, Cfg> g;

  setUpAll(() async {
    g = await cfgsOf('samples/cfg/constructs.dart');
  });

  int kind(Cfg c, EdgeKind k) => c.edgeCounts[k] ?? 0;
  int copies(Cfg c) => c.blocks.where((b) => b.items.contains('finally (copy)')).length;

  test('every function resolves and has no dead block and no trapped block', () {
    for (final MapEntry(key: name, value: c) in g.entries) {
      expect(c.blocks.where((b) => !c.reachable.contains(b)), isEmpty, reason: '$name has dead blocks');
      expect(c.reachable.difference(c.reachExit), isEmpty, reason: '$name has blocks that cannot reach the exit');
    }
  });

  group('async', () {
    test('each await is an edge to a resume block', () {
      expect(kind(g['awaitInSequence']!, EdgeKind.awaitResume), 2);
      expect(kind(g['awaitInCondition']!, EdgeKind.awaitResume), 1);
      expect(kind(g['awaitInLoop']!, EdgeKind.awaitResume), 1);
    });

    test('await in a condition happens before the branch', () {
      final c = g['awaitInCondition']!;
      final branch = c.blocks.firstWhere((b) => b.items.contains('branch'));
      expect(branch.label, startsWith('resume'));
    });

    test('await for suspends at each step of the loop, and break leaves it', () {
      final c = g['awaitFor']!;
      expect(c.edges.where((e) => e.kind == EdgeKind.awaitResume && e.note == null).length, 1);
      expect(kind(c, EdgeKind.brk), 1);
      expect(kind(c, EdgeKind.back), 1);
    });

    test('an exception at an await goes to the catch clauses', () {
      final c = g['tryAroundAwait']!;
      final dispatch = c.blocks.firstWhere((b) => b.label == 'catch.dispatch');
      final fromAwait = c.edges.where((e) => e.to == dispatch && e.kind == EdgeKind.exception);
      expect(fromAwait, isNotEmpty);
      expect(kind(c, EdgeKind.awaitResume), 1);
    });
  });

  group('try / catch / finally', () {
    test('the finally block is copied once per way out', () {
      // normal end of the try, return inside the try, return in the catch,
      // and an exception that no catch clause handles.
      expect(copies(g['finallyManyExits']!), greaterThanOrEqualTo(4));
    });

    test('a return inside the try goes through the finally first', () {
      final c = g['finallyManyExits']!;
      for (final b in c.blocks) {
        for (final e in b.out.where((e) => e.kind == EdgeKind.ret)) {
          expect(e.to, c.exit);
          // The block that returns has already run a copy of the finally block.
          expect(b.items, contains('return'));
        }
      }
      final withFinally = c.blocks.where((b) => b.items.contains('finally (copy)') && b.out.any((e) => e.kind == EdgeKind.ret));
      expect(withFinally.length, greaterThanOrEqualTo(2));
    });

    test('rethrow in a catch body runs the finally and leaves', () {
      final c = g['tryAroundAwait']!;
      expect(c.edges.where((e) => e.note == 'rethrow'), isNotEmpty);
      expect(copies(c), greaterThanOrEqualTo(2));
    });

    test('code after the last call of a try body is not covered by its catch clauses', () {
      // The copy of the finally block follows the last call of the try body.
      // If the exception edge belonged to the block, it would cover the
      // finally code too: calls that may throw end their block.
      final c = g['finallyManyExits']!;
      final finallyCalls = c.blocks.where((b) => b.items.contains("sink('finally')"));
      expect(finallyCalls, isNotEmpty);
      for (final b in finallyCalls) {
        for (final e in b.out.where((e) => e.kind == EdgeKind.exception)) {
          expect(e.to.label, isNot('catch.dispatch'), reason: 'finally code is outside the try');
        }
      }
    });

    test('a try inside a try keeps its own finally', () {
      final c = g['nestedTry']!;
      expect(copies(c), greaterThanOrEqualTo(2));
      expect(kind(c, EdgeKind.catchMatch), 1);
    });
  });

  group('loops and labels', () {
    test('break outer leaves the outer loop; continue outer starts its next step', () {
      final c = g['labeledJumps']!;
      final brk = c.edges.firstWhere((e) => e.kind == EdgeKind.brk);
      final cont = c.edges.firstWhere((e) => e.kind == EdgeKind.cont);
      expect(brk.to.label, 'for.after');
      // The block after the outer loop holds the statement after it.
      expect(brk.to.items.any((i) => i.contains("'after'")), isTrue);
      expect(cont.to.label, 'for.head');
      // The head of the outer loop is the one that is entered first.
      final heads = c.blocks.where((b) => b.label == 'for.head').toList();
      expect(cont.to, heads.first);
    });

    test('while, do and break', () {
      final c = g['whileAndDo']!;
      expect(kind(c, EdgeKind.brk), 1);
      expect(kind(c, EdgeKind.back), 2);
    });
  });

  group('switch with patterns', () {
    test('a case that does not match falls to the next one; guards branch', () {
      final c = g['area']!;
      expect(kind(c, EdgeKind.caseMatch), 3);
      expect(kind(c, EdgeKind.caseFail), 4); // 3 cases and the final "no case matched"
      expect(c.blocks.where((b) => b.label == 'when'), hasLength(1));
    });

    test('a switch expression yields a value on every path, with a throw if nothing matches', () {
      final c = g['describe']!;
      expect(kind(c, EdgeKind.caseMatch), 4);
      expect(c.edges.where((e) => e.note == 'no case matched'), isNotEmpty);
    });
  });

  group('expressions', () {
    test('&& and || are control flow', () {
      final c = g['shortCircuit']!;
      expect(c.blocks.where((b) => b.label == '&&.rhs'), hasLength(1));
      expect(c.blocks.where((b) => b.label == '||.rhs'), hasLength(1));
      expect(kind(c, EdgeKind.ifTrue), greaterThanOrEqualTo(4));
    });

    test('a conditional expression is a diamond', () {
      final c = g['shortCircuit']!;
      expect(c.blocks.where((b) => b.label == '?.then'), hasLength(1));
      expect(c.blocks.where((b) => b.label == '?.else'), hasLength(1));
    });
  });
}
