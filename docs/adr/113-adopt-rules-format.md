# ADR-113: まとめての採用の書式 `adopt_rules:` と、生成した行の `origin: generated`(段階 1。2026-11-23)

- 状態: 有効
- 関連: DESIGN-package-models 6.12、MODELING 4c-2、ADR-108、ADR-110

**背景:** 層 1 の候補は、採用した行より桁で多い。1 件ずつ行を足すのも、1 件ずつ `rejected:` に書くのも、同じ判断の繰り返しになる。DESIGN-package-models 6.12 は、判断を規則の単位で 1 回だけ書き、コマンドが行に展開する形を設計した。この ADR は、その最初の段階(書式とローダー)で決めたことを記録する。展開のコマンドは含まない。

**決めたこと:**
1. 手のモデルのファイルに、`adopt_rules:` の節を置く。項目は `id`、`kind`(sink か source)、`rule`、`words`、`category`(sink のみ。source は `labels`)、`scope`、`why`(必須)、`covers`(必須)、`representatives`、`diff_checked`(`version` と `count`。確認するまで省略できる)、`extra_ok` と `extra_covers`(EXTRA の理由は 1 つ。その理由が覆う件数を記録し、増えたら警告する)。引数の型で位置を選ぶ規則 `sink.arg-type` は `arg_types:`(型の完全な名前)を取り、パッケージより狭い範囲を要る。
2. `scope` は、決定的に選べるものだけ: `package`(完全一致。Go は `/v*`)、`receiver_types` と `functions`(完全一致の一覧)、`receiver_suffix` か `receiver_prefix`(1 つ)。シンボルへの glob と正規表現は書けない。
3. ローダーは `rejected:` と同じく、形を厳しく検査するだけで、解析は `adopt_rules:` を読まない。エラーには、項目の番号と id と行を出す。規則の id は `bench/model-author/rules.go` の表で安定させ(テストで固定)、ローダーは `internal/model` の同じ一覧(`AdoptRuleIDs`)で確認する。語が規則の語彙にあるかは、表を持つコマンドが確認する。
4. 行の `origin` に `generated` を足す。`decision: <id>` は generated の行だけが持ち、generated の行は必ず持つ。生成した行は `<lang>-<package>.adopt.gen.yaml`(`*.yaml` の埋め込みに入る)に置く。同じパッケージの手のファイルと一緒に読むとき(`builtin.Load` の中の `model.CheckAdoption`)、`decision` が手のファイルの `adopt_rules:` にあり、行の種類(sink か source)が決めと合うことを確認する。generated の行が別のファイルにある、生成のファイルに sink と source 以外か `adopt_rules:` がある、も拒否する。`--deterministic-only` は llm の行だけを外すので、generated の行は残る。
5. 2 つの決めが同じ位置を取ったときは、カテゴリが同じなら 1 つの行にまとめて警告(両方の決めの名前を出す)、違えば終了コード 70 で止めて、人が決める。検出は純関数(`ResolveClaims`)で、展開は後の段階。

**生成した行と zoo の検査(この ADR では変えない):** 今の `TestBuiltinRowsAreHitByTheZoo` は、組み込みの全部の行に zoo の当たりを求める。生成した行を、決めごとの代表のセルと差分テストで確認することにして、この検査から外す方針は、ユーザーが原則として承認した。ただし、**外すのは、テストの変更が別の承認つきの変更として入ったあと**で、それまでテストはそのまま。生成した行は `.dead-allow` に書かない方針も、そのときのテストで決める。

**しないこと(この ADR では):** 展開のコマンド、`.adopt.gen.yaml` の書き出し、source の型単位の書式(6.12 の 4 節)、`TestBuiltinRowsAreHitByTheZoo` の変更。
