#!/bin/sh
# usage: fetch-sha.sh <repo-url> <full-sha> <outdir>
# Fetches ONE commit, shallow, into a new empty directory. Nothing of the target is built, tested or run:
# the files are read by `rivus analyze` (for Go, `go list` and the download of the modules are done by it).
# No absolute path is written here: the output directory is an argument.
set -eu
[ $# -eq 3 ] || { echo "usage: $0 <repo-url> <full-sha> <outdir>" >&2; exit 2; }
url=$1; sha=$2; out=$3
case "$sha" in *[!0-9a-f]*|"") echo "the SHA must be a full lowercase hex SHA" >&2; exit 2 ;; esac
[ ${#sha} -eq 40 ] || { echo "the SHA must have 40 digits" >&2; exit 2; }
if [ -e "$out" ] && [ -n "$(ls -A "$out" 2>/dev/null)" ]; then echo "$out is not empty" >&2; exit 2; fi
mkdir -p "$out"
git -C "$out" init -q
git -C "$out" remote add origin "$url"
git -C "$out" fetch -q --depth 1 origin "$sha"
git -C "$out" checkout -q FETCH_HEAD
test "$(git -C "$out" rev-parse HEAD)" = "$sha"
echo "$sha" > "$out/.sha"
echo "fetched $url at $sha into $out"
