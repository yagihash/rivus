#!/bin/sh
# usage: fetch.sh <outdir> [vuln|fixed]   (default: vuln)
# Fetches the target at a fixed SHA, shallow. Nothing is built or run. The output directory must be new or empty.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
case "${2:-vuln}" in
  vuln)  sha=7b0b426a76fd77b89817e0eafcccaed8d15b8cf4 ;;
  fixed) sha=411d76798d73446fff4a0244f0475f1ea8bf42dc ;;
  *) echo "usage: $0 <outdir> [vuln|fixed]" >&2; exit 2 ;;
esac
exec "$here/../../fetch-sha.sh" https://github.com/casdoor/casdoor "$sha" "$1"
