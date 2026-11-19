#!/bin/sh
# The regression of the Dart extractor on real code: extracts the project of
# this directory (which imports 55 popular packages) with its dependencies, and
# checks that no construct is left out ("unsupported") and that the program is a
# valid IR (no function defined twice). Usage: run.sh [DART_COMMAND] ; needs `rivus` on the PATH.
#   dart pub get && ./run.sh "mise exec dart@3.13.6 -- dart"
set -e
DART=${1:-dart}
HERE=$(cd "$(dirname "$0")" && pwd)
OUT=${TMPDIR:-/tmp}/dart-corpus.jsonl
(cd "$HERE/../../extractor/dart" && $DART run bin/rivus_dart.dart "$HERE" > "$OUT")
N=$(grep -c '"op":"unsupported"' "$OUT" || true)
echo "unsupported: $N"
if [ "$N" != 0 ]; then
	grep -o '"reason":"[^"]*"' "$OUT" | sort | uniq -c | sort -rn | head -20
fi
rivus sites -ir "$OUT" -no-builtin-rules > /dev/null
echo "valid IR: yes"
[ "$N" = 0 ]
