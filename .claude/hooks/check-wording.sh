#!/bin/bash
# Claude Code の PostToolUse hook（.claude/settings.json から呼ばれる）。
# Claude が .md ファイルを編集した直後に、そのファイルの用語と言い回しを regomark でチェックする。
# 違反があれば、内容を Claude に返す（exit 2）。regomark がなければ、インストールはせず、利用者に頼む。
# ルールは .regomark/policy/wording.rego。CI では実行しない。
set -u

file=$(jq -r '.tool_input.file_path // empty' 2>/dev/null)
case "$file" in
  *.md) ;;
  *) exit 0 ;;
esac

# 編集されたファイルのあるリポジトリ（worktree を含む）のルートで実行する
root=$(git -C "$(dirname "$file")" rev-parse --show-toplevel 2>/dev/null) || exit 0
[ -f "$root/.regomark.yaml" ] || exit 0
cd "$root" || exit 0
rel=${file#"$root"/}

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

out=$(regomark check --format text 2>&1)
status=$?
case "$status" in
  0) exit 0 ;;
  1)
    mine=$(printf '%s\n' "$out" | grep -F -- "$rel:" || true)
    [ -z "$mine" ] && exit 0
    {
      echo "$rel に、使わない用語・言い回しがあります。言い換えてください（ルール: .regomark/policy/wording.rego。行番号はブロックの先頭）:"
      printf '%s\n' "$mine"
    } >&2
    exit 2
    ;;
  *)
    # 設定やルールの誤りなど、チェックそのものができなかった
    printf 'regomark check を実行できませんでした（exit %s）:\n%s\n' "$status" "$out" >&2
    exit 2
    ;;
esac
