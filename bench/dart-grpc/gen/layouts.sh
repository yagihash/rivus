#!/bin/sh
# Builds two layouts of the same generated code under OUT (not run in CI; needs the pub cache, `dart pub get --offline`):
#   OUT/own/            the generated code is in the app package (code of the project)
#   OUT/mono/app        the app; the generated code is in OUT/mono/protos (package bulk_protos, a path dependency):
#   OUT/mono/protos       it is read with -allow-deps-under OUT/mono
# usage: gen/layouts.sh MESSAGES OUT
set -e
N=$1
OUT=$2
HERE=$(cd "$(dirname "$0")/.." && pwd)
mkdir -p "$OUT/own" "$OUT/mono/app" "$OUT/mono/protos/lib"
cp -r "$HERE/pubspec.yaml" "$HERE/pubspec.lock" "$HERE/bin" "$HERE/lib" "$OUT/own/"
(cd "$HERE" && dart run gen/gen.dart --messages "$N" --app-dir "$OUT/own" --pb-dir "$OUT/own/lib/src/generated" \
	--pb-import package:grpc_bench/src/generated/bulk.pbgrpc.dart)
cp -r "$HERE/pubspec.yaml" "$HERE/bin" "$HERE/lib" "$OUT/mono/app/"
cat > "$OUT/mono/protos/pubspec.yaml" <<'EOP'
name: bulk_protos
publish_to: none
environment:
  sdk: ^3.10.0
dependencies:
  fixnum: ^1.1.1
  grpc: 5.2.0
  protobuf: ^6.1.0
EOP
printf '  bulk_protos:\n    path: ../protos\n' >> "$OUT/mono/app/pubspec.yaml"
(cd "$HERE" && dart run gen/gen.dart --messages "$N" --app-dir "$OUT/mono/app" --pb-dir "$OUT/mono/protos/lib" \
	--pb-import package:bulk_protos/bulk.pbgrpc.dart)
for d in "$OUT/own" "$OUT/mono/protos" "$OUT/mono/app"; do (cd "$d" && dart pub get --offline > /dev/null); done
