### 6.12 一括採用(規則の単位の決定から、行を生成する。設計案)

**状態: 段階 1 と段階 2 は実装済み(ADR-113)。** 段階 1: `adopt_rules:` の書式とローダー、`origin: generated` と `decision:`、`*.adopt.gen.yaml` の読み込みと照合(`model.CheckAdoption`)、規則の id(`bench/model-author/rules.go`)、同じ位置を取る決めの検出(`ResolveClaims`)。段階 2(Go): 展開のコマンド(`make model ... ARGS="-expand"`。`bench/model-author/expand.go`)。位置ごとに規則の id と語を付け(`Item.Tags`。`-recall` の出力は変わらない)、`sink.arg-type` の走査、範囲、手の行と `rejected:` / `deferred:` の優先、`covers` の検査(増えたら終了コード 70。数字を書き戻すのは `-write-covers` だけ)、代表の確認、`-out` を実装した。実際の `*.adopt.gen.yaml` はまだ組み込みに入れない(`TestBuiltinRowsAreHitByTheZoo` の変更(2 節)は、別の承認のあと)。Dart と Rust の展開、source の型の単位の判断(4 節)の検査は、まだ。

背景: 層 1 の候補は、採用した行より桁で多い。測定した数: go-github v93 は sink の行 57 に対して、未決の層 1 の候補が 1,321。Dart の `github` は、行 22 に対して未決 402(source の候補 74。その約 3 分の 1 は、利用者が書いた内容を持つフィールドで、ADR-108 の基準に入る。`RepositorySlug` 型だけで 147)。手のモデルは、同じ形の API の一部だけを採用していて(owner / repo を取るサービスのメソッドは 56 が sink で、同じ形の兄弟が数百、行なし)、残りは「判断して外した」のでなく、**手作業の都合で行がないだけの見逃し**。1 件ずつ `rejected:` に書くのは、判断の中身(同じ理由)の繰り返しで、1 件ずつ行を足すのは量が多すぎる。この節は、**判断を規則の単位で 1 回だけ書き、コマンドが行に展開する**形の設計案。実装はしない。事実は、`bench/model-author/rules.go`、`recall.go`、`internal/model/model.go`、`rejected.go`、`cmd/rivus/dead_rows_test.go`、ADR-108 で確認した。「`rejected:` の型単位のまとめの不採用」は、決めた案として扱うが、コードにはまだない(`rejected.go` は 1 件ずつ。**要確認**)。

#### 1. 判断の書式と、展開

**置き場所**: 手のモデルのファイル(`<lang>-<package>.yaml`)の新しい節 `adopt_rules:`。`rejected:` と同じく、ローダーが読んで形を検証するだけで、**解析は読まない**(解析が読むのは、行だけ)。展開した行は、別のファイル `<lang>-<package>.gen.yaml` に書く(4.3 の決めどおり。手の行が同じ symbol にあれば、手の行が勝つ)。

```yaml
adopt_rules:
  - id: owner-repo-of-services          # ファイルの中で一意。生成した行から指す
    kind: sink                          # sink | source | sanitizer(calls は対象にしない)
    rule: sink.arg-word                 # 層 1 の規則の id(下の表)
    words: [owner, repo]                # 規則の語彙の中から、採る語。語彙にない語は、検証で拒否する
    category: resource_name             # 行のカテゴリ。規則が出すカテゴリと違うときは override_why が要る
    scope:
      package: github.com/google/go-github/v*/github   # 完全一致(Go は /v*。ADR-110)
      receiver_suffix: Service          # 型の名前の末尾が 1 つ。ほかは receiver_types: [..](完全一致の一覧)
    why: "owner と repo は、URL のパス repos/<owner>/<repo>/ に入る。別のリポジトリを指せる"
    covers: 56                          # 展開した件数。ずれたら、下の 3 の動き
    representatives: [(*github.com/google/go-github/v*/github.RepositoriesService).Get,
                      (*github.com/google/go-github/v*/github.IssuesService).Create]   # 2 節
```

**規則の id**。今の `rules.go` は、規則を文章(`Item.Rule`: "sink: argument that names a ...")で持ち、id がない。決めの安定のために、表の各規則に短い id を付ける(語彙の表 `sinkWords` などは、今のまま):

| id | 今の規則(rules.go) | 判断の単位 |
| --- | --- | --- |
| `sink.arg-word` | 引数の名前の語が、カテゴリの語の一覧にある(`categoryOf`) | `words` と `category` |
| `sink.name-of-resource` | `name` / `id` の引数で、関数の名前に資源の名詞がある(`resourceNouns`) | `category: resource_name` |
| `sink.file-name` | `name` の引数で、関数の名前にファイルの動詞がある(`fileVerbs`) | `path_traversal` |
| `sink.bytes-word` | `[]byte` の引数で、語が `bytesSinkWords` | `words`、`xss` など |
| `sink.struct-field` | 構造体の引数のフィールドの名前がカテゴリの語(`arg1.Field`) | `words`、`field` の位置も展開する |
| `source.field-word`、`source.result-word` | 本文や内容の語を持つフィールド、戻り値(`sourceWords`) | 型の単位の判断(4 節) |
| `sanitizer.name` | string → string で、名前に `escape` などがある | `words` |

`calls:`(関数を引数に取る関数)は対象にしない。形が 1 つずつ違い、本体を読むパッケージでは候補にもならない(MODELING 4c-1)。

**範囲(`scope`)は、決定的に選べるものだけ**: `package`(完全一致。Go は `/v*`)、`receiver_types`(完全一致の一覧)、`receiver_suffix` と `receiver_prefix`(型の名前に対して、各 1 つ。`rejected:` の型単位のまとめと同じ制限)、`functions`(関数の名前の完全一致の一覧)。**シンボルへの自由な glob と正規表現は書けない**。1 つの決めに複数の範囲を書いたときは、すべてを満たすもの(AND)。例外は、`rejected:` の 1 件(今の書式。展開から外れる)か、手の行(手の行が勝つ)。

**展開のアルゴリズム**(`make model` の新しい段。取得した公開 API だけを読む):

1. 層 1 の候補を、今と同じに作る(型だけを読む。シンボルの辞書順)。
2. 決めごとに、候補のうち、`kind`、`rule` の id、`scope` を満たすものを選ぶ。位置は、候補の位置のうち、`words` の語を持つものだけ(空なら、その候補は対象外)。
3. すでに手の行(sink なら同じ位置)がある候補、`rejected:` に(位置が合って)ある候補は外す。件数は、`covers` と別に `skipped_by_row` / `skipped_by_rejected` として出力に書く。
4. 残りを、手の行と同じ形の行にする(`symbol`、`args` または `field`、`category`、`origin: generated`、`decision: <id>`)。2 つの決めが同じ位置を取ったときは、先に書いた決めが勝ち、後の決めは警告(終了コード 70 と同じ扱いにするかは、**要確認**)。
5. `<lang>-<package>.gen.yaml` に、決めの順、その中で symbol の辞書順で書く。先頭に `generator: model-author@<バージョン>`、`input: <パッケージ>@<バージョン>`、`rules: <規則の表のハッシュ>`、`decisions: <adopt_rules の内容のハッシュ>`。

**決定性、べき等**: 出力は、(決めの内容、手の行、`rejected:`、パッケージのバージョン、規則の表)だけで決まる(時刻と乱数なし)。同じ入力で 2 回走らせると、同じバイト列(テストで確認)。手の行を足すと、その symbol の生成の行が消える(差分に出る)。

**バージョンが変わったとき**: 新しい API が決めに合うと、行が増え、`covers` を超える。**`covers` より増えたときは、終了コード 70(未決)で止まり**、増えた symbol の一覧を作業ファイルに書く。人が見て、`covers` を直す(外すものは `rejected:` に足す)。減ったときは警告だけ(API が消えた)。ファイルの差分(`.gen.yaml` の追加と削除)で、何が sink になったかが、PR で見える。`verified_versions` は、今までどおり手で(`-write-verified`)。

**ローダーと `rivus models`**: ローダーは `adopt_rules` を読み、`id` の一意、`kind`、`rule`(形だけ。規則の表は `bench/model-author` にあるので、id の存在はコマンドが確認)、`why`、`covers` の有無、不明なキーの拒否を検証する。`origin` の値に `generated` を足す(`model.go` の `Origin*` と `Provenance.validate` は、今は builtin、human、llm だけで、`generated` は拒否される)。`--deterministic-only` は llm の行だけを外す(`WithoutOrigin`)ので、generated の行は残る(規則と表が決定的で、決めは人が書くため。**要確認**)。`rivus models` は、決めの件数を出す程度でよい(行は、今のとおり)。`go:embed *.yaml` は `*.gen.yaml` も含み、`builtin.Load` は名前順に読む(4.3 の別ファイルと同じ)。

#### 2. 生成した行の確認

今の `TestBuiltinRowsAreHitByTheZoo`(`cmd/rivus/dead_rows_test.go`)は、組み込みの**全部の行**に、zoo か curated の呼び出しが当たることを求める(当たらなければ `.dead-allow`、実行できない API だけ)。行の `origin` は見ない。数百の生成した行に、全部セルを書くのは現実的でなく、`.dead-allow` に数百を並べるのは、`.dead-allow` の意味(実行できない API)を崩す。

- **代表のセル**: 決めごとに、代表の行を `representatives:` に書く(生成した行の中から)。選び方(決定的): その決めが出した行を「位置の組(`args` と `field`)」で分け、組ごとに symbol の辞書順で最初の 1 つ。3 組を超えるときは、行の数が多い組から 3 つ。owner / repo の例なら、2 組ほど。セルは、既存の骨組みとテンプレート(`-gen-skeleton`)で作り、vuln / fixed / decoy を付ける。**代表が zoo で `@miss` になる決めは、採用しない**(規則かキーの綴りが間違っている)。
- **残りの行は、差分テスト(`bench/model-diff`)**: 今の差分テストは、`internal/model/builtin/*.yaml` の全部の行を、実際の本体を読む解析と比べる(`*.gen.yaml` も同じ glob に入る。**要確認**: `bench/model-diff/model.go` の 253 行の glob と、行の symbol を型で作る箇所)。MISS が 0 であることを、決めごとの条件にする。EXTRA は、今は `# model-diff: extra` の注釈が要る。生成した行には、行ごとの注釈を付けられないので、決めに `extra_ok: "<理由>"` を持たせる案(**要確認**)。結果(日付、版、MISS、EXTRA、未確認の件数)は、決めに `diff_checked:` として書く(`verified_versions` と同じ作法。`-write-verified` が書く)。
- **`TestBuiltinRowsAreHitByTheZoo` の最小の変更**: (a) `builtinCallRows` で、`origin: generated` の行を別の一覧にして、今の検査(全部に zoo の当たり)の対象から外す。(b) 新しいテスト `TestAdoptDecisionsAreHitByTheZoo`: `adopt_rules` ごとに、`representatives` が 1 つ以上あり、その全部が生成した行であり、かつ zoo の当たりがあること、`diff_checked` があって MISS が 0 であることを検査する。実行できない API の代表は、手のファイルの `.dead-allow` に理由つきで書く(今の形のまま)。(c) 生成した行の `.dead-allow` は禁止(生成した行が `.dead-allow` にあれば失敗)。生成した行のうち zoo に当たった割合は、`RIVUS_DEAD_ROWS_PRINT=1` のときだけ出す。**これは、ゲートを「全部の行」から「決めごとに代表」へ緩める変更なので、ユーザーの承認が要る**(`Zoo-Loosened:` の対象の `marks.lock` とは別のゲート。**要確認**)。
- **zoo のゲート(6.2)**: 代表のセルは、新しいセルなので、今のゲートの規則(新しいセルの `@sink(none)`、`@miss`、`@fp` は承認なし。`@miss` と `@fp` は `@ref(#N)` が要る)のままでよい。生成した行は、マークを増やさないので、ゲートに影響しない。

#### 3. 偽陽性の増え方の測定

**今の動き(コードで確認)**: カテゴリを選ぶ仕組みは、今ない。`rivus analyze` のフラグ(`cmd/rivus/analyze.go` の 92〜129 行)に、カテゴリの選択はない。`category`(と、`categories`)は、finding と site に付く文字列で(`internal/taint/engine.go` の finding、`internal/sites/sites.go`)、**組み込みの全部の sink が、既定で有効**。`resource_name` も同じ。source にラベルがあり、sink は(ラベルを書かない限り)どのラベルにも反応する(`model.go` の `Sink` と `Source.Labels`。AGENT-GUIDE の 39〜44 行は、`jq` で `.category` を見て分ける作法)。見たいカテゴリの選択は、利用者が結果を絞ること(`jq`)で、解析の側ではない。**だから、sink が 57 から数百に増えると、`resource_name` の finding は、そのまま既定の出力に増える**。カテゴリを選べるようにする(例: `-categories`)ことは、この設計の外で、決めるのはユーザー(open question 1)。

**測定の案(今は走らせない。Go の解析だけ。新しい取得は、MODEL-QUEUE の中のパッケージだけ)**:

1. 合成の小さいサーバを `bench/go-synth` の形で作る(3.1 の AWS S3 の例と同じ)。go-github v93 を使い、`r.URL.Query().Get(..)` の値を、(a) 手の行がある 56 の sink、(b) 生成で増える兄弟のメソッド、(c) ID や数値の引数(sink にならないはず)に渡す。期待の finding を `expected/` に書き、`score.py` で採点する。目的: 増えた行が、何を出すか(真の検出か、無関係か)を、小さい形で見る。
2. 実在の対象は、`bench/` にある対象のうち、`go.sum` に go-github があるものだけ(今、`bench/` の中に go-github を使う対象は見つからない。`bench/zoo-go-pkg/go_github` だけ。実在の対象が要るときは、取得の前にユーザーに聞く。方針)。
3. 比べるもの: `bench/compare.sh <base> <head> --light --only <名前>`(base は、`*.gen.yaml` を足す前の main、head は、足した PR)。`details/<bench>.md` で、追加された finding を、カテゴリ別に数える(`jq` で `.findings[] | .category` を集計)。見る数: (i) finding の総数、(ii) 追加の finding のうち `resource_name` の割合、(iii) 追加のうち、source が手で採用した source か、新しい source か、(iv) 追加の finding の中で、人が確認して偽陽性と判断したものの割合(上限 30 件。決まった乱数でなく、辞書順の最初の 30 件)、(v) 実行時間とメモリ(`-pass-stats`。行が増えると、サマリが増える)。
4. 受け入れの条件(案): 差分テストが MISS 0 で、追加の finding の偽陽性の割合を、決めごとに出して、PR の本文に書く。**割合の閾値は、ユーザーが決める**(open question 2)。決めが偽陽性の多い形(例えば、すべてのサービスのメソッドの `path`)なら、その決めを外すか、`functions` で狭める。

#### 4. source の側(ADR-108)

書式は、`kind: source` で、同じ `adopt_rules:` に書ける(`rule: source.field-word`、`source.result-word`)。展開すると、手の行と同じ `sources:` の行(`kind: field` の `field:`、または `call_result` の `symbol:`、`labels: [user_input]`)になる。ただし、**ADR-108 の基準は「その値を誰が書いたか」で、名前の語(`body`、`content`)では決まらない**(ID、時刻、SHA、サービスが決める URL は、source にしない。4 項)。語の規則は候補を出すだけで、**判断は型の単位に 1 回、人が理由つきで書く**。

```yaml
adopt_rules:
  - id: user-written-text
    kind: source
    rule: source.field-word
    types:                               # 判断の単位は型(完全一致の一覧)。型ごとに、採る項目の名前を書く
      - type: package:github/src/common/model/issues.dart#Issue
        fields: [title, body]
      - type: package:github/src/common/model/issues.dart#IssueComment
        fields: [body]
    labels: [user_input]
    why: "ADR-108 の 3 項。Issue の題と本文は、サービスの利用者が書く"
    not_adopted:                         # 型の中の、候補に出るが採らない項目と理由(5 項: 迷う値は入れない)
      - {type: ...#Issue, fields: [url, htmlUrl], why: "サービスが組み立てる URL"}
```

- **人の判断が要る**: 型ごとに「その項目を、利用者が書くか」。これは機械で決められない(`RepositorySlug` の 147 件は、ほとんどがサービスが決める値か、名前で、source にならない)。コマンドは、`types:` に書かれていない型の候補を、今までどおり未決(70)にする。`not_adopted` は、`rejected:` の項目に展開して数える(型単位のまとめの不採用と同じ形)。
- **機械でできる検査**: 項目の型が string か、本文を運ぶ型であること(数値、日時、bool は拒否)。名前が、ADR-108 の 4 項の語(`id`、`sha`、`createdAt`、`url`)なら警告を出して、`why` に反証を書かせる(**要確認**: 語の一覧)。
- 既存の `dart-gcp` などの見直し(課題 #158)は、この設計で行えるが、別の作業。

#### 5. 生成のファイルは 1 つか、別々か

**推奨: 生成器ごとに別のファイル。** `<lang>-<package>.gen.yaml` は、4.3a の型から作る flows(段階 3)に使う。一括採用の行は `<lang>-<package>.adopt.gen.yaml` とする。理由:

- **差分の見やすさ**: 一括採用の差分は「何が新しく sink になったか」(判断の結果。レビューで見る)で、型からの flows の差分は「API の形の変化」(機械的)。1 つにすると、PR の差分の中で、見るものと見ないものが混ざる。
- **衝突しにくさ**: 生成器のバージョンが上がると、型の flows のファイルは全部が書き換わる。判断の結果のファイルは、規則の表を変えたときだけ。別のファイルなら、片方の再生成が、もう片方の差分に出ない。
- **再生成**: 先頭の `generator`、`input`、`decisions` のハッシュが、ファイルごとに別。1 つにすると、片方だけ再生成できない。
- **`origin:` の追跡**: 行の `origin: generated` は共通で、生成器の区別はファイル名の先頭と、行の `decision:` で付く。flows の行には `decision:` がない。`builtin.Load` は名前順に読み、2 つのファイルが同じ symbol に行を出すと、行は足し合わされる(MODELING 4a-1)。そこで、**2 つの生成の間の重なりは、テストで拒否する**(型の flows が出す symbol には、一括採用は行を出さない)。
- 代償は、ファイルが増えること。1 つのパッケージで最大 3 つ(手、flows、採用)。

#### 5a. 使う場所への sink の移し替え(Dart の `github` の `RepositorySlug` で確認)

問い: 「値を作る場所(`RepositorySlug.new`、`.full`)の sink を、使う場所(スラッグを取るサービスのメソッド。型の単位の採用で、位置は、スラッグの引数そのもの)へ移し、古い行を流れの行(引数 → 戻り値)に変えて、同じ流れを 2 回報告しない」を、この節の形で書けるか。答え: **古い行の変更は、今の形で書ける。新しい sink の採用は、書けない(規則が足りない)**。(1) 古い行: 手のファイルの `sinks:` の 2 行を消し、`propagators:` に `- symbol: package:github/src/common/model/repos.dart#RepositorySlug.new` / `flows: [{from: arg0, to: result}, {from: arg1, to: result}]`、`RepositorySlug.full` は `flows: [{from: arg0, to: result}]` と書く(`propagators` の書式は `dart-http.yaml` と同じ)。これは手の行の編集で、`adopt_rules` は要らない(手の行は、生成の行に勝つ)。(2) 新しい sink: `sink.arg-word` は、引数の**名前の語**で選ぶ(`words` は規則の語彙の中だけ)ので、`slug` という名前の引数、または `RepositorySlug` 型の引数は選べない。足りないものは、**引数の型で位置を選ぶ規則**。次のように書けるとよい(案)。

```yaml
adopt_rules:
  - id: slug-argument-of-services
    kind: sink
    rule: sink.arg-type               # 新しい規則。位置は、引数の型が types の中にあるすべての引数
    types: [package:github/src/common/model/repos.dart#RepositorySlug]
    category: resource_name
    scope:
      package: package:github/*       # Dart の package の書き方は要確認
      receiver_suffix: Service
    why: "スラッグの owner と name は、リクエストのパス /repos/<owner>/<name>/ に入る。別のリポジトリを指せる"
```

そのほか足りないもの: (a) 4 節の「判断の単位は型」と同じ作りだが、あちらは source の側(`types:` は、型の中の項目を選ぶ)。ここでは、**引数の型**を選ぶので、`types:` の意味が違う(別のキー名がよい。例: `arg_types:`)。(b) 重なりの処理: 手の `sinks:` にある `getBranch` などの行(位置が別の引数)と、新しい規則が選ぶ引数 0 は位置が別なので、3 節のとおり「同じ位置の手の行は外す」だけで足りる。(c) 古い行を流れの行に変える**順序**の保証: 古い sink の行を残したまま新しい規則を入れると、同じ流れが 2 回出る。ゲートは、「`adopt_rules` の `arg_types` の型を作る関数(戻り値がその型)に、手の `sinks:` の行が残っていれば警告」と書けるが、今の設計にはない(要確認)。(d) フィールドへの代入(`slug.owner = v`)と `fromJson` は、新しい行がなくても、スラッグの引数の sink で流れが見つかるはず(値は、スラッグ(オブジェクト)に載って運ばれ、使う場所で出る)。**これは、解析が、フィールドへの代入と `fromJson` でオブジェクトに値を運べることに頼る**。zoo では、今はこの 2 つの形に不確かさ(trace)つきの見逃しが出るので(`F/dart/github-slug-fromjson`、`F/dart/github-slug-assign`)、行を直しても、解析の側で見つからないことが残る可能性がある(課題 `model-dart-github-repositoryslug-miss`)。

#### 6. 実装の順と、残る確認

**最初の小さい段(段階 1b)**: Go だけ。(1) 規則の表に id を付ける(`rules.go`。挙動は変えない。`-recall` の結果が変わらないことを確認)。(2) `adopt_rules:` のローダーの検証と、`origin: generated` と `decision:` の追加(`internal/model`)。(3) `bench/model-author` に、展開と `covers` の検査と `.adopt.gen.yaml` の書き出しを足す。(4) 試す対象は、go-github の `owner-repo-of-services` の 1 件(sink の側)だけ。56 の行が出ることと、手の行の symbol が重ならないことを、差分(手の行の `args` と比べる)で確認する。(5) 差分テストを通す。代表のセルと、2 節のテストの変更は、(4)(5)のあと(ユーザーの承認が要る)。source の側、Dart、Rust は、そのあと。

**今あるもので足りる部分**: 候補の一覧(`rules.go`、`Scan`)、手の行と `rejected:` との照合(`match.go`)、`-recall`(展開が手の行をどれだけ再現するかの検査に、そのまま使える。展開した結果が、手の 56 行の `args` と一致するかを見る)、差分テスト、`-gen-skeleton`、`readDeadAllow`、`builtin.Load`(別ファイルの読み込み)。**新しく要る**: 規則の id、`adopt_rules` の型とローダー、展開、`covers` の検査、`generated` の origin、テストの変更。

**相談役への確認(5 件)**:

1. `resource_name` を含めて、カテゴリを選べる仕組み(利用者が `-categories` などで選ぶ)を作るか。今は、全部が既定で有効で、sink が増えると finding も増える。
2. 偽陽性の割合の閾値(3 節の 4)。何%まで許すか、または、決めごとの判断を人の確認だけにするか。
3. `TestBuiltinRowsAreHitByTheZoo` を「全部の行」から「決めごとの代表」へ緩めること(2 節)。ユーザーの承認が要る変更か。
4. 同じ位置を 2 つの決めが取ったとき、警告か、失敗(70)か。EXTRA の理由を、決めに 1 つで足りるとするか。
5. `rejected:` の型単位の一括却下(`covers` つき)は、一括採用の `not_adopted` と同じ実装にしてよいか(コードは、まだ 1 件ずつ)。

