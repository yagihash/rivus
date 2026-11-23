#!/bin/bash
# Claude Code の PostToolUse hook（.claude/settings.json から呼ばれる）。
# Claude が .md ファイルを書いた直後に、そのファイルの用語と言い回しを regomark でチェックする。
# 対象は、Claude が書いた .md のすべて: リポジトリの中の文書（.regomark.yaml の documents に無いものも）、
# リポジトリの外の文書（セッションの間の相談、引き継ぎ、メモなど）。
# 違反があれば、内容を Claude に返す（exit 2）。regomark がなければ、インストールはせず、利用者に頼む。
# ルールは、そのファイルのあるリポジトリの .regomark.yaml（あれば）、なければプロジェクト（$CLAUDE_PROJECT_DIR）のもの。
# CI では実行しない。
set -u

file=$(jq -r '.tool_input.file_path // empty' 2>/dev/null)
case "$file" in
  *.md) ;;
  *) exit 0 ;;
esac
[ -f "$file" ] || exit 0

# ルールの置き場所: 編集されたファイルのあるリポジトリ（worktree を含む）。そこに設定がなければ、プロジェクト。
src=$(git -C "$(dirname "$file")" rev-parse --show-toplevel 2>/dev/null) || src=""
if [ -z "$src" ] || [ ! -f "$src/.regomark.yaml" ]; then
  src=${CLAUDE_PROJECT_DIR:-}
fi
[ -n "$src" ] && [ -f "$src/.regomark.yaml" ] || exit 0

policy=$(sed -n 's/^policy:[[:space:]]*//p' "$src/.regomark.yaml" | head -n 1)
entry=$(sed -n 's/^entrypoint:[[:space:]]*//p' "$src/.regomark.yaml" | head -n 1)
policy=${policy:-.regomark/policy}
entry=${entry:-wording}
[ -d "$src/$policy" ] || exit 0

if ! command -v regomark >/dev/null 2>&1; then
  msg='regomark が見つからないので、文書の用語チェックをスキップしました。インストール: go install github.com/babarot/regomark@latest'
  jq -n --arg m "$msg" '{
    systemMessage: $m,
    hookSpecificOutput: {
      hookEventName: "PostToolUse",
      additionalContext: ($m + "（自動でインストールしないこと。利用者にインストールを依頼する。）")
    }
  }'
  exit 0
fi

# regomark は、git のリポジトリのルートからの相対パスだけを読む。だから、一時のリポジトリに、
# そのファイルとルールを写して、1 ファイルだけをチェックする（リポジトリの外のファイルも、同じ形で見られる）。
tmp=$(mktemp -d "${TMPDIR:-/tmp}/rivus-wording.XXXXXX") || exit 0
trap '/bin/rm -rf "$tmp"' EXIT
git init -q "$tmp" 2>/dev/null || { echo "check-wording: git init に失敗したので、$file をチェックできませんでした" >&2; exit 2; }
mkdir -p "$tmp/policy"
cp -R "$src/$policy/." "$tmp/policy/"
cp "$file" "$tmp/doc.md"
printf 'documents:\n  - "doc.md"\npolicy: policy\nentrypoint: %s\n' "$entry" > "$tmp/.regomark.yaml"

out=$(cd "$tmp" && regomark check --format text 2>&1)
status=$?
case "$status" in
  0) exit 0 ;;
  1)
    {
      echo "$file に、使わない用語・言い回しがあります。言い換えてください（ルール: $src/$policy/wording.rego。行番号はブロックの先頭）:"
      printf '%s\n' "$out" | while IFS= read -r line; do printf '%s\n' "${line/#doc.md:/$file:}"; done
    } >&2
    exit 2
    ;;
  *)
    # 設定やルールの誤りなど、チェックそのものができなかった
    printf 'regomark check を実行できませんでした（exit %s）:\n%s\n' "$status" "$out" >&2
    exit 2
    ;;
esac
