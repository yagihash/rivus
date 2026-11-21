#!/bin/sh
# usage: fetch.sh <outdir> [vuln|fixed]   (default: vuln)
# Fetches the target at a fixed SHA, shallow. Nothing is built or run. The output directory must be new or empty.
# The vulnerable version is the same commit as GHSA-9c9w-9pq7-f35h's (both flaws are in it).
set -eu
here=$(cd "$(dirname "$0")" && pwd)
case "${2:-vuln}" in
  vuln)  sha=27d13a0584731371033926064d750e5f4f620e7a ;;
  fixed) sha=2a452bda89ffdb85f929fa78290bce1f456881dc ;;
  *) echo "usage: $0 <outdir> [vuln|fixed]" >&2; exit 2 ;;
esac
exec "$here/../../fetch-sha.sh" https://github.com/gophish/gophish "$sha" "$1"
