#!/bin/sh
# usage: fetch.sh <outdir> [vuln|fixed]   (default: vuln)
# Fetches the target at a fixed SHA, shallow. Nothing is built or run. The output directory must be new or empty.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
case "${2:-vuln}" in
  vuln)  sha=27d13a0584731371033926064d750e5f4f620e7a ;;
  fixed) sha=e3352f481e94054ffe08494c9225d3878347b005 ;;
  *) echo "usage: $0 <outdir> [vuln|fixed]" >&2; exit 2 ;;
esac
exec "$here/../../fetch-sha.sh" https://github.com/gophish/gophish "$sha" "$1"
