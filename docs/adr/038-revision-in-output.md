# ADR-038: 出力にリビジョンを記録する（`target`）

- 状態: 一部覆った
- 関連: ARCHITECTURE 4.5
- 状態の詳細: `dirty` を廃止、git を実行しない; 訂正・追記あり(書き方を保つ)。追記あり。覆した先・注意: 冒頭に「`dirty` は廃止」
- 要旨: `target: {revision}`。追記で `.git/config` の実行を避ける

**背景:** ID（`s1:`＋structural）にはリビジョンが入っていない。リビジョンが変わっても ID が安定することが目的なので、これは正しい。ただし `rivus resolve` はリビジョンが変わった後を前提にするのに、出力が「どのコードの結果か」も、ブックマークが「どのコードから取ったか」も記録していなかった。

**決定:**
1. **ID は変えない。** リビジョンは ID に入れない
2. **出力に `target` を入れる:** `analyze` / `sites` / `query` / `resolve` と、保存したインデックスに `target: {revision, dirty}` を付ける。`revision` は git の HEAD、`dirty` は追跡ファイルに変更があること。git の作業ツリーの外では省略する（エラーにしない）
3. **ブックマークに `revision` を持たせる:** ブックマーク単体の `revision`、または `{"target": ..., "bookmarks": [...]}` の形でまとめて渡す（個別の `revision` が優先）。出力の `target` をそのまま写せばよい
4. **`resolve` の出力:** 各結果に `from_revision` と `revision_change` を付ける。`revision_change` は `same`（同じコミットで、どちらの作業ツリーも変更なし）、`different`、`unknown`（どちらかが不明、または作業ツリーに変更あり）
5. スキーマバージョンを上げる: `rivus.result/1`、`rivus.query/1`、`rivus.sites/1`、`rivus.resolve/1`（フィールドの追加のみで、既存のフィールドは変えていない）

**限界:** `dirty` は変更の中身を区別しない（追跡されていないファイルは見ない）。dirty な作業ツリー同士は、同じコミットでも `unknown` になる。古いリビジョンを自分で展開して解く `resolve --from <rev>` は、まだ作らない（ブックマークのスニペットとフィンガープリントで足りるうちは不要）。

### 追記: `dirty` をやめ、git を実行しない（yagihash/rivus #6）

- **決定:** `target` は `{revision}` だけにする。リビジョンは `.git/HEAD`、refs、`packed-refs`（リンクされたワークツリーは `commondir`）を直接読んで求め、git は実行しない。`dirty` は出さない。
- **理由:** 解析対象の `.git/config` は信頼できないコードの一部で、`git status` は `core.fsmonitor`、`filter.*.clean`、`include.path`、サブモジュール、promisor など、設定が指すコマンドを実行する。設定を列挙して打ち消す方式は、経路が増えるたびに抜けた（#6 が再オープンされた）。
- **影響:** `revision_change` の `same` は「同じコミット」の意味になり、作業ツリーに未コミットの変更があるかどうかは見ない。`unknown` は、どちらかの target がないとき。reftable など、読めない形式のリポジトリは `target` なし。

### 追記: `when_arg_type` の数え方を `args` に揃え、`recv: true` を足した（yagihash/rivus #56）

- **決定:** `when_arg_type.arg` は、`args` / `params` / `out` と同じく、**レシーバを除いて**数える。レシーバの型を条件にするときは `when_arg_type: {recv: true, types: [...]}`（`arg` は読まない。`recv: true` と 0 以外の `arg` はエラー）。レシーバのない呼び出しは、レシーバへの条件を満たさない。
- **理由:** 同じ sink の中で数え方が 2 通りあると、メソッドの sink に書いた条件が、1 つずれた引数を見て、何も報告せずに効かなくなる（見逃しになる）。組み込みの `when_arg_type` は関数にしか使っていないので、既存のルールは変わらない。
- **限界（既存）:** 型は、引数の局所変数の型と、そこに直接入る `convert` の元の型（最大 4 段）だけを見る。変数に一度入れてから渡した値（`var o Out = Web{}; o.Put(x)`）は、元の型をたどれず、条件が成り立たない。

