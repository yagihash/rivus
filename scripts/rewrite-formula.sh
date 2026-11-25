#!/usr/bin/env bash
# tap の Formula/rivus.rb(標準入力)の `url` と `sha256` の行を書き換えて、標準出力に出す（tagpr.yml の bump-tap が使う）。
# 使い方: scripts/rewrite-formula.sh <url> <sha256> < Formula/rivus.rb > rivus.rb
# 行頭が 2 つの空白の `url "..."` と `sha256 "..."`（formula の本体の宣言）だけを書き換える。どちらかの行がなければ、失敗にする。
set -euo pipefail

url="${1:?usage: rewrite-formula.sh <url> <sha256>}"
sha="${2:?usage: rewrite-formula.sh <url> <sha256>}"
case "${url}${sha}" in
  *[\|\&\"\\]*)
    echo "the url or the sha256 has a character that the rewrite does not handle" >&2
    exit 1
    ;;
esac

out="$(sed -E "s|^  url \".*\"|  url \"${url}\"|; s|^  sha256 \".*\"|  sha256 \"${sha}\"|")"
if ! grep -qxF "  url \"${url}\"" <<<"${out}" || ! grep -qxF "  sha256 \"${sha}\"" <<<"${out}"; then
  echo "the formula has no url or sha256 line to rewrite" >&2
  exit 1
fi
printf '%s\n' "${out}"
