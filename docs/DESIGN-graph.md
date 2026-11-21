# 設計: コールグラフの検索(`rivus graph`)と、保存するファイル

> **状態: 実装済み**(確認: 2026-11-22、9c26a39)
> - 実装済み: `callers`、`callees`、`path`、`impls`、`dispatchers`、`info` と各フィルタ。
> - 未実装: taint 層の DB への移行と `findings_at_sites`。
> - 注意: 本文の `reaches` / `paths` は、実際には `path`。`index -taint` は DB でなく JSON に出す。この設計に対応する ADR は、まだ無い。
> - 使い方と出力の正本は [GRAPH.md](GRAPH.md)。この文書は設計の理由(経緯)。

設計時の状態の記述(今の状態は上の枠が正本): **議論中**(決まったところから書く。議論が終わったら、DECISIONS.md に ADR としてまとめる)。

## 目的

AI エージェントが、コード上の 1 点を指して、「そこへ届く経路」「そこから届く経路」を検索できるようにする。taint 解析は要らず、grep で見つけた箇所の経路を確かめる使い方も満たす。解析結果を保存して、問い合わせのたびに、ロードからやり直さない。

## 決めたこと

### 1. 検索の単位と指定
- グラフの辺は、**呼び出し点(call site)**から呼び先の候補への辺。経路は、行と桁つきで出せる。
- 指定は 3 通りを受ける: **シンボル**、**`file:行:桁`**(その位置の呼び出し点)、**`file:行`**(行にある全部の呼び出し点。複数あれば、エラーにせず、候補を返して選ばせる)。IR は `Line` / `Col` / `Offset` を持つ。
- 呼び先が複数の呼び出し点(インターフェース、関数値)は、曖昧さではなく情報として、候補の一覧と出どころ(静的 / インターフェース / 関数値)を返す。

### 2. 経路の返し方(3 つの問いに、3 つの形)
- `callers` / `callees`: 深さを指定した、近くの隣接(既定は 1 段、上限つき)。
- `reaches`: 届くかどうかと、最短の経路 1 本。
- `paths`: 上限 k 本(既定 5)。最短から、重複の少ないものを選ぶ。
- 切ったときは、**必ず、数と上限を言う**(黙って切らない)。再帰や共通のユーティリティは、畳む。

### 2b. 動的な呼び出し(インターフェース、関数値)の見せ方
- 問題になるのは、順向きの深い展開(`callees` を深く)と、ハブ(`io.Writer.Write`、`error.Error` のような、あらゆる所から呼ばれ、あらゆる実装に飛ぶ呼び出し)を経由した、意味のない経路。起点と終点を指定する `reaches` / `paths` と、具体的な実装からの逆向きの `callers` は、終点(または起点)につながる辺だけをたどるので、爆発しない。
- 順向きの展開は、**動的な呼び出しで止めて、フロンティアとして見せる**(候補の数、静的な型、出どころ)。候補が少ない(既定 3 以下。実データで決める)ものは、そのまま展開する。**止めたものは、必ず出力で言う**(止める = 隠す、ではない)。
- `reaches` / `paths` は、まず確定した辺だけの経路を探し、なければ、動的な辺を含む経路を、「動的を含む」と分けて返す。ハブを経由する経路は、さらに分けて返す。
- 入口を足す: `graph impls <インターフェースのメソッド>`(実装の一覧)、`graph dispatchers <実装のメソッド>`(その実装を候補として呼ぶ動的な呼び出し点)。具体的な実装から始める使い方を促す。
- **辺のフィルタ**: `--via <symbol>`、`--avoid <symbol>`、`--only-impl <型>`。エージェントが、コードを読んで知った情報(「この Writer の実体は bytes.Buffer」)で絞れる。**絞り込んだことは、必ず出力に書く**(`filters`)。「この条件のもとで経路なし」と言い、条件を外すと出るかもしれないことも書く。
- 使い方の知識は 3 層: ① ツールの既定(安全。見逃しを黙って作らない) ② 出力の `hints` と `--help`(その場で伝わる) ③ スキル(詳しい戦略。あとで作る)。ガードは、スキルに頼らず、ツールに置く。

### 2c. 入口
- 最初は **CLI だけ**。MCP などの常駐サーバは、あとで検討する(ARCHITECTURE の「`query` 層は差し替え可能」に沿う)。

### 2d. コマンドの整理
- DB は 2 つの層でできる: **構造の層**(関数、呼び出し点、辺、サイト。速い。数十秒)と、**taint の層**(findings、不確実性。遅い。数十分)。
- **`rivus index` 1 つで作る**。既定は構造の層だけ。`rivus index -taint` で、taint の層も足す。構造だけで作ったときは、出力に「taint の層はない。`-taint` で追加できる」と書く(黙って、findings がないと誤解させない)。`graph build` は作らない。`graph` は、読む専用。JSON の index の互換は気にしない。
- **サイト ID を、指定の 4 つ目**として受ける(`graph callers --site <ID>`)。サイトの列挙は解析なしで速いので、サイトの表は構造の層に入れる。逆に、`graph` の出力のノードと辺に、そこに載っているサイト ID を付ける。taint の層があるときは、経路に重なる finding を付ける(橋)。
- **言葉を分ける**: 呼び出しの関係は `calls`、データの流れは `flows`。`graph` は `calls` だけを言い、すべての経路に `"relation": "calls"` を付ける。taint の層があれば `"findings_at_sites": [...]`(経路の呼び出し点にあるサイトを source / sink とする finding。名前は決定済み。`also_flows` ではない)、なければ `"flows": "not_checked"`(確認していないことを明記する)。`reaches` という名前は使わず、`graph path` に一本化する(届くかは出力の `reachable`、最短 1 本は `--limit 1`)。`--help` の冒頭に、1 行で、違いを書く。
- コマンド: `index [-taint]`、`graph callers` / `callees` / `path` / `impls` / `dispatchers` / `info`。ほかは、今のまま(`sites`、`analyze`、`query`、`resolve`、`extract`、`models`)。

### 2e. 出力の形(`rivus.graph/1`)
- 問いの種類で形を変える。`callers` / `callees` / `impls` / `dispatchers` は、**ノードと辺を分ける**(広がりの結果。共通の関数が多い)。`path` は、**経路を、ステップの列**で返す(順番が大事。そのまま読める)。
- `path` は、最初は、重複を許して、読みやすさを優先する(経路は k 本、既定 5)。トークンが問題になったら、ノードの表に分ける(スキーマ版を上げる)。
- 各ステップに必ず入れる: 関数のシンボル、位置(`file:line:col`)、呼び出しの種類(`static` / `interface` / `value`)と候補の数(動的のとき)、その呼び出し点にあるサイト ID(あれば)。経路全体に、`certainty`(`certain`: 全部静的、`uncertain`: 動的を含む)と `relation: "calls"`。
- 共通で出す: `snapshot`(`source_hash`、作成日時、rivus の版、`-C` で照合したか)、`truncated`(切った数。0 でも書く)、`frontier`(動的で止めた呼び出し点)、`filters`(絞り込み)、`hints`(次の一手)。

### 3. 保存するファイル
- **SQLite の 1 ファイル**(純 Go の実装。cgo なし)。**使い捨てのキャッシュ**で、元に戻せる派生物。
- **対象(`-C`)ごとに 1 つ**。多言語は、1 つの中に、言語の列で区別する。
- **明示したパスからだけ読む**(`-graph path`)。解析対象のディレクトリの中の DB を、自動では読まない(細工された DB を同梱できるため)。読み取りは、読み取り専用で開く。既定の書き込み先は、ユーザーのキャッシュ(解析対象のディレクトリには作らない)。
- **世代管理はしない**(`resolve` は DB に依存しない: ブックマークが、指紋とスニペットと revision を持つ)。
- **互換はマイグレーションしない**。スキーマ版(`PRAGMA user_version`)、rivus の版、ルールのハッシュが合わなければ、エラーで作り直しを案内する。ユーザーのデータ(ブックマークなど)は DB に入れない。
- 表は、グラフの部分(`funcs`、`callsites`、`edges`)と、あとで足す taint の部分に分け、`meta` に部分ごとの版・ハッシュ・作成日時を持つ。ずれたら警告する。JSON の index は、移行するまで、そのまま。
- 土台として、ファイルごとの内容ハッシュ(`files` 表)と、関数ごとの本体のハッシュ(`funcs` 表)を、最初から持つ。

### 4. 使い回しと、見逃しを避ける原則
- 今回は、**まるごとの再利用(段階 1)だけ**。変更があれば、作り直す。
- **原則: 疑わしいときは、必ず作り直す。** 偽のヒット(変わったのに古い結果を使う)は見逃しになるので、絶対に避ける。偽のミス(作り直す)は、時間がかかるだけで、許容する。
- 鍵には、結果に影響しうるものを、全部入れる: 読んだソース(依存、ビルドタグ、`-tests` で読まれるものを含む)の内容、依存(`go.sum`、lock、`replace` と path 依存の中身)、ルール、フラグ、rivus の版、環境(言語の版、`GOOS`/`GOARCH`、ツールチェイン、extractor の版)、解析が読んだプロジェクトの外のファイル。
- 鍵は、**実際に読んだファイルの一覧**から作る(推測しない)。読み込み時に、`-C` があれば、今のソースから鍵を作り直して照合し、違えば、古いと警告して、作り直す(または拒否)。`-C` なしで DB だけを渡されたときは、出力に、いつ・どの `source_hash` で作ったかを必ず付け、「今のソースとの一致は確認していない」と書く。
- 鍵の各要素について、「変えたらヒットしない」ことのテストを書く。
- **段階 2(抽出の結果の再利用)と段階 3(関数の要約の再利用)は、結果が「全部やり直したとき」と完全に同一であることを、変更のパターンを変えて(Gitea を含めて)確かめられるまで、入れない。**疑わしいときは、広く無効にする。

## 段階 1 の実装(2026-11-21)で決まったこと

`rivus index`(構造の層)、SQLite への保存、`rivus graph info` を作った。段階 2 以降(`callers` / `callees` / `path` / `impls` / `dispatchers`、フロンティア、フィルタ、サイト ID の連携)は、まだ。

- **コード**: `internal/graphdb`(スキーマ、書き込み、読み取り、鍵の部品)、`cmd/rivus/graph.go`(`index` の構造の層、`graph info`)、`cmd/rivus/graphkey.go`(鍵の組み立て)。SQLite は `modernc.org/sqlite`(純 Go)。
- **スキーマ(`PRAGMA user_version` = 1)**: `meta`(key/value: `schema_version`、`rivus_version`、`created_at`、`source_hash`、`dir`、`languages`、`flags`、`key_config`、`manifest`、`roots`、`revision`、`layers`(層ごとの名前・版・作成日時の配列。今は `structure` だけ))、`files`(`path`、`sha256`、`size`、`kind`。読んだファイルは `sha256` を持ち、位置にだけ出るファイルは持たない)、`funcs`(`fid`、`symbol`、`file_id`/`line`/`col`/`offset`、`entry`、`init`、`dep`、`exported`、`has_body`、`lang`、`body_hash`。本体のない呼び先も、`has_body = 0` の行として入る)、`callsites`(`func_id`、位置、`kind` = `static` / `interface` / `value` / `builtin`、`callee`、`ncand`、`is_go`)、`edges`(`callsite_id`、`from_func`、`to_func`、`kind`、`ncand`)、`sites`(`rivus sites` と同じ ID と位置。呼び出し点なら `callsite_id`)。索引: シンボル、位置(`file_id`, `line`, `col`)、関数 ID、辺の両端(`from_func`、`to_func`、`callsite_id`)。taint の層の表はない。
- **辺の候補は `sg` のもの**: 静的(`StaticID`)、インターフェース(`Callee.Candidates`)、関数値(VTA の候補と `internal/valuecalls`)。呼び出し点の種類(`kind`)は、呼び出しの種類(IR の `Callee.Kind`)で決め、辺の `kind` はそれと同じ。インターフェースの呼び出しで候補が 0 のものも、呼び出し点としては入る(`ncand = 0`)。組み込み(`len`、`append` など)も呼び出し点(`kind = builtin`)として入る。
- **`-taint` の扱い**: `rivus index` の既定は、構造の層だけを SQLite に書く。`-taint`(または `-o`)は、これまでどおり JSON の index を書く(`query -index` と `resolve -index` は、そのまま JSON を読む)。同じ実行で、構造の層も SQLite に書く(グラフを作った直後なので、費用がほぼない)。taint の層は、JSON にあり、SQLite にはない(出力に書く)。`-o` を `-taint` なしで渡したときは、`-taint` を意味する(これまでの使い方を、黙って変えない)。
- **保存先**: `-graph <file>`。なければ `os.UserCacheDir()/rivus/<-C の実パスの SHA-256 の先頭 8 バイト>/graph.db`(環境変数 `RIVUS_CACHE_DIR` で、キャッシュのルートを変えられる。テストが使う)。一時ファイルに書いて、移動する(読む側が、書きかけを見ない)。世代管理なし。読むのは `-graph` か `-C` から導いたキャッシュだけで、`mode=ro`、`query_only`、`trusted_schema=0` で開き、ビューとトリガーを持つファイルは拒否する。スキーマ版が違えば、`rivus index` で作り直すよう案内するエラー。
- **鍵(`source_hash`)**: SHA-256 の元は、(a) 設定(`rivus` の版 = 実行ファイルのハッシュ + 任意の `versionStamp`、`-tests` / `-no-deps` / `-k` / `-numeric-taint` / `-max-implementations` / `-lang`(解決後)/ `-allow-deps-under` / `-no-builtin-rules` / `-no-project-rules` / `-deterministic-only` / パッケージ、組み込みルールの内容のハッシュ、ルールファイルの一覧、`go env -json` の全項目(`GOGCCFLAGS` だけは毎回違う一時ディレクトリを含むので除く。`GOMOD` / `GOWORK` は `-C` からの相対にする)、Dart と Rust の extractor の実行ファイルのハッシュ・`--version`・SDK の版)、(b) ファイルとその内容のハッシュ、(c) ツリーのファイル名の一覧のハッシュ(マニフェスト。新しいファイルを見逃さない)。
  - **Go のファイル**は `go/packages` の一覧(`GoFiles`、`CompiledGoFiles`、`OtherFiles`、`EmbedFiles`、ビルド制約で除かれた `IgnoredFiles`。依存と標準ライブラリを含む)と、各モジュールの `go.mod`、`go.sum` / `go.work` / `go.work.sum` / `vendor/modules.txt`(あれば)。マニフェストの対象は、`-C` と、メインのモジュールと `replace` 先のディレクトリ(モジュールキャッシュは除く)。**読んでいないファイル(`-tests` なしの `_test.go` など)の内容の変更は、変更にしない**(読んでいないので、結果に影響しない)。ただし、ファイルの追加と削除は、マニフェストで変更になる。
  - **Dart と Rust**: IR のストリームは、読んだファイルの一覧を持たない(IR のスキーマは変えていない)。そこで、保守的に、プロジェクトのツリーの全ファイルの内容(`.git`、`.dart_tool`、`.rivus`、`node_modules`、ルート直下の `build` と `target` を除く。`lib/.generated` のような、隠しディレクトリの中の、読まれるソースは含む)、ロックファイル(`pubspec.lock`、`.dart_tool/package_config.json`、`Cargo.lock`、`rust-toolchain*`、`.cargo/config*`)、IR が名前を挙げるツリーの外のファイル(パッケージキャッシュの依存。**`PruneDeps` の前**の一覧)、extractor と SDK の版、で鍵を作る。`-ir` のときは、ストリームのファイルの内容が鍵(ソースは読まない)。
  - 読んでいる最中にファイルが書き換わったら(読み込み開始より新しい更新時刻)、鍵を作らず、エラーにする。
  - 照合(`graph info -C`)は、記録した一覧のファイルを今のディスクで再ハッシュし(ロック、ルール、ツリーは、一覧を作り直す)、設定を作り直し、違いを `stale_reasons` に、1 行ずつ出す。**出力の `fresh: true` は、「同じフラグで、今のソースから作り直しても同じ鍵になる」こと**。
- **足りないこと(保守的にしていないもの)**: Dart の extractor 自体のソース(`pub global activate --source path` で入れたもの)の内容は、ハッシュしていない(`--version` と shim の実行ファイルだけ)。Rust のプロジェクトが、`-C` より上のワークスペースのルートの `Cargo.lock` を使うときは、読まない(`-C` をルートにする)。Dart / Rust の `-C` の外の、パス依存(`path:`)の中の、IR に関数が出ないファイル(宣言のない定数だけのファイルなど)は、見ない。
- **測定(2026-11-21。他のエージェントが動いていて、時間は揺れる)**: x/tools v0.52(関数 9,549、呼び出し点 57,989、辺 48,328、サイト 274): `rivus index` 全体 4.2 秒(鍵 0.28 秒、書き込み 0.24 秒)、DB 10 MB、開く 3 ms、`file:line:col` の検索 0.2 ms、`graph info -C` 0.28 秒。Gitea(関数 63,591、呼び出し点 482,450、辺 584,929、サイト 696、ファイル 12,640): 全体 90 秒(ロードと IR が 77 秒。構築の費用は、鍵 0.9 秒 + 書き込み 2.2 秒 + 本体のハッシュ)、DB 91 MB、開く 3 ms、位置の検索 0.2 ms、`graph info -C` 0.77 秒。

- **既知の揺れ**: 同じソースから作っても、関数の件数が数件違うことがある(x/tools で 9,542〜9,549)。違うのは標準ライブラリの関数(`(*sync.Mutex).Lock`、`slices.Repeat` など)の本体の有無で、`Deps` つきの Go のロード(`ssautil.AllFunctions`)の、もとからある揺れ。`rivus extract -all`(依存なし)は安定している。DB の鍵には影響しない(鍵は入力から作る)が、段階 2 で「件数が同じか」を比べるときは、これを踏まえる。

## 段階 2 の実装(2026-11-21)で決まったこと

`rivus graph callers | callees | path` を作った(DB だけを読む。ソースの再読み込みも taint もしない)。`impls` / `dispatchers` / `--via` / `--avoid` / `--only-impl` と、taint の層との橋(`also_flows`)は、段階 3 以降。

- **コード**: `internal/graphdb/query.go`(隣接の取得: `OutEdges` / `InEdges`、位置・シンボル・サイトの検索。索引 `edges_from` / `edges_to` / `callsites_func` / `callsites_pos` を使う)、`internal/graphq`(`graphq.go` の型と定数、`resolve.go` の指定の判別、`traverse.go` の callers / callees、`path.go` の path)、`cmd/rivus/graphq.go`(フラグ、`-C` の照合、JSON の出力)。スキーマ(`user_version` = 1)は変えていない(段階 1 の DB が、そのまま読める)。
- **探索はメモリに全部読まない**: 関数 1 つの隣接を、SQL で 1 回ずつ取る(1 つの問いの中でキャッシュ)。深さ(最大 6)、辺の数(`--limit`、既定 50)、path の訪問数(40 万)と列挙の歩数(200 万)に上限がある。**辺を通すかの述語(`EdgePred`)を、探索が受け取る**(`--via` / `--avoid` / `--only-impl` は、これを作って渡すだけ)。静的だけ、動的の候補数の上限は、SQL に押し込む(ハブを読んで捨てない)。
- **指定の意味(迷って採用した解釈)**: 位置(呼び出し点 C)は、「C を含む関数」と「C が呼ぶ関数」の 2 つを指しうる。`callers C` は **C を含む関数の呼び元**(C が呼ぶ関数の呼び元ではない。それはシンボルで言える。位置を渡すのは、grep で見つけた行の、上流を知りたいとき)。`callees C` は **C が呼ぶ関数**(C の呼び先の候補。C を含む関数が呼ぶもの全部ではない。それは、含む関数のシンボルで言える)。`path --from C` は C を通って始まる(最初のステップは C を含む関数で、`at` が C)。`path --to C` は C を含む関数で終わる(最後のステップの `at` が C)。深さは、`callees C` では、C の呼び先が深さ 1。
- **位置の判別**: `file:line:col` は、その列から始まる呼び出し点(DB が持つのは呼び出しの始まりだけで、終わりはない)。そこに始まる呼び出しがなければ、その行で、それより前に始まる最後の呼び出し点(`resolved` に言う)。`file:line` は、その行の呼び出し点(builtin を除く)。**同じ関数の同じ位置に呼び出し点が複数あるとき(Rust の `?` の脱糖など)は、1 つの呼び出し点として扱う**(callees は、全部の呼び先の和)。位置の違う呼び出し点が複数あれば、`ambiguous` に返す(エラーにしない)。呼び出し点のない行は、エラーで、直前に始まる関数を示す(関数の終わりも DB にないので、確定はしない)。ファイルは、DB の名前、絶対パス(解析対象のディレクトリからの相対にして照合。シンボリックリンクも解く)、末尾(1 つに決まるなら)。シンボルは、関数の ID(`fid`)でも `symbol` でも引く(メソッドとそのラッパーは、同じ `symbol` を持つので、複数の関数になる。全部を起点にする)。パッケージのパスを省いた末尾が 1 つに決まるなら、それを使う(`resolved` に言う)。見つからないときは、末尾が同じ名前を候補に出す。
- **builtin**: 既定で、結果から外す(`kind = builtin` の呼び出し点には、辺がない)。`callees` は、展開した関数の builtin の数を `omitted.builtin` に足す(`callers` では、位置の指定が builtin の呼び出し点を含んでいたときの数)。builtin しかない行を指定したら、エラー(`--include-builtin` を案内)。`--include-builtin` で、`builtin:len` のノードと、`kind: builtin` の辺として戻す。
- **動的な辺**: `callers` は、すべての種類の辺を通る。動的な辺には、`kind` と `candidates` が付く。`callees` は、動的な呼び出しのうち、**候補が 3 以下(`ExpandCandidates`)のものを展開し、4 以上のものと、候補が 0 のもの(実装が見つからず、どこへ行くか言えない)で止めて、`frontier` に入れる**(位置、`kind`、候補の数、静的な型・メソッドの名前 = `callee`、サイト ID。最大 50 件を載せ、総数は `frontier_total`)。位置で指定した呼び出し点の候補は、数にかかわらず全部出す(`--limit` の範囲で)。
- **path**: 起点と終点の両側から、小さい方を 1 層ずつ広げて、最短の長さを求める。次に、終点からの距離(最短 + 2 まで)で刈りながら、短い順に、関数の列を k 本(と、残りがあるか言うための 1 本)数える。同じ 2 つの関数の間に呼び出し点が複数あるときは、1 つだけ使う(静的を優先、次に、ファイルの位置の順)ので、**同じ関数の列は重複しない**。まず静的な辺だけ。なければ、動的な辺(候補が 20 以下(`HubCandidates`))を含めて探し、`certainty: uncertain`。(この段は、「段階 2.5 と 3a で決まったこと」の 3 段の探索に置き換わった。`hub_skipped` の一覧はなくなった。)見つからなかったときは、ヒントで、実装を指定して始める道を案内する。`reachable: false` のとき、`exhaustive: true` は、探索が上限で切れていないこと(「ない」が確か。ただし、使った辺の範囲で。ハブは使っていない)。切れたら `warnings`。k 本で切ったら、`truncated: {count: 1, at_least: true, limit, unit: "paths"}`。
- **出力(`rivus.graph/1`)**: 共通: `schema`、`relation: "calls"`、`snapshot`(`source_hash`、`created_at`、`rivus_version`、`verified`、`fresh`、`stale_reasons`、`note`)、`query`(`op`、`target` または `from` / `to`、`depth`、`limit`、`resolved`)、`truncated`(`count`、`limit`、`unit`、`at_least`、`unexpanded`、`beyond_depth`。0 でも書く)、`omitted`(`builtin` は 0 でも書く)、`filters: []`、`hints`、`warnings`(古いとき、深さを切ったとき、探索が切れたとき)。`callers` / `callees`: `start`、`call_site`(位置の指定のとき)、`nodes`(`id` = 関数の ID、`symbol` は ID と違うときだけ、`at` = 宣言の位置、`depth`、`no_body`)、`edges`(`from`、`to`、`at`、`kind`、`candidates`(動的のとき)、`sites`)、`ambiguous`。`callees` は `frontier` と `frontier_total`。`path`: `reachable`、`certainty`、`exhaustive`、`paths`(各 `certainty`、`hops`、`steps`)、`hub_skipped`、`hub_skipped_total`。ステップは `func`、`at`(その関数の中の、次の関数へ進む呼び出し点)、`kind`、`candidates`、`sites`(最後のステップに `at` はない。呼び出し点が終点のときを除く)。出力は、既定で 1 行の JSON(`--pretty` で整形)。
- **`-C` の照合**: `graph info` と同じ(`verifySnapshot`)。古くてもエラーにせず、答えを返し、`warnings` に言う(答えは古いソースのもの。「黙って使わない」を満たす)。照合のとき、与えなかった解析のフラグ(`-tests`、`-no-deps`、`-k`、`-lang` など)とパッケージのパターンは、DB に記録されたものを使う(`index` に渡した通りに鍵を作り直すため。与えれば、そちらを使うので、違えば古いと出る)。`-C` なしは、`verified: false` と `note`。
- **動的な呼び出し点の候補の数(Gitea。展開する上限を 3 にした根拠)**: インターフェース 21,470 点 / 関数値 6,612 点の、候補の数の分布(インターフェース / 関数値): 0 個 3,694 / 937、1 個 9,987 / 4,330、2 個 2,717 / 557、3 個 1,043 / 151、4〜5 個 717 / 174、6〜10 個 1,328 / 259、11〜20 個 792 / 91、21〜50 個 522 / 65、51〜100 個 260 / 42、101 個以上 410 / 6。候補のある点(23,451)のうち、3 以下は 18,785(80%)、20 以下は 22,146(94%)、21 以上(ハブ)は 1,305(5.6%)。**展開の上限 3 と、path のハブの上限 20 は、この分布による**(3 までで、候補のある呼び出しの 8 割が、止まらずに展開される。20 を超えるものは、全体の 6% で、その先が、ほぼ「あらゆる実装」)。x/tools(動的 4,391 点)も同じ形: 候補ありの 3,519 点のうち、3 以下は 2,579(73%)、21 以上は 223(6.3%)、候補 0 は 872。
- **測定(2026-11-21。他のエージェントが動いている中。プロセスの起動と DB を開く時間(約 3 ms)を含む)**: Gitea(関数 63,591、辺 584,929): `callers log.Error`(1,840 の呼び元、ハブ)21 ms、出力 8.6 KB(約 2,100 トークン、50 辺で切って、`truncated.count` が 1,790 以上)。`callers GetUserByID --depth 2` 17 ms、13.8 KB。`callers (errorWrapper).Error --depth 2` 18 ms、20 KB。`callees runWeb --depth 3` 17 ms、`callees Routes --depth 3` 16 ms、11.6 KB(約 2,900 トークン)、`--limit 200` で 40 KB、`--depth 6 --limit 1000` で 52 ms、200 KB(約 50,000 トークン。上限を上げたときの費用)。`path runWeb → GetUserByID`(静的になし、動的あり。5 本 + さらにあり)180 ms、8 KB。`path runWeb → (*Session).Where` 253 ms、6.4 KB。`path runWeb → libc.Int32FromInt32`(なし。ハブ 15 点を使わず)312 ms、2.4 KB。`path GetUserByID → runWeb`(なし)15 ms。x/tools(関数 9,549): `callers (Cursor).Node`(ハブ)15 ms、14.7 KB。`callees packages.Load --depth 3` 9 ms、12 KB。`path` 7〜40 ms。
- **既知の限界**: (1) 呼び出し点の終わりの位置は DB にないので、`file:line:col` の「その位置を含む呼び出し点」は、始まりの一致(なければ、その行の直前に始まる呼び出し点)で近似する。(2) 関数の終わりも持たないので、呼び出し点のない行は、含む関数を言えない(直前に始まる関数を、候補として出すだけ)。(3) path の列挙は、最短 + 2 の長さまで。それより長い経路は、k 本に満たなくても、探さない。(4) `callers` の深さ 2 以降は、`--limit` で、幅が先に尽きることがある(`unexpanded` に言う)。(5) path の `hub_skipped` は、探索で見た範囲の、ハブの一部で、経路に関係しないものを含む。

## 段階 2.5 と 3a で決まったこと(2026-11-21)

構造の層(DB に書く辺、スキーマ)は変えていない(`user_version` = 1 のまま。短い指定の索引は、足さずに済んだ)。

- **path は 3 段で探す**: 静的 → 動的(候補 20 以下)→ ハブ込み(全部の辺)。次の段は、前の段で経路がなかったときだけ。出力に `searched`(探した段。`["static"]`、`["static","dynamic"]`、`["static","dynamic","hub"]`)を足し、`exhaustive` は、その範囲についてのもの。3 段目でも `reachable: false` で `exhaustive: true` なら、DB の全部の辺について「経路なし」。上限(訪問 40 万、歩数 200 万)は変えていない。
- **`certainty: "hub"`**: 3 段目で見つかった経路。ハブのステップに `hub: true` と `candidates`。並べる順は、ハブのステップの少ない順、次に短い順(費用 = ハブ 1 つ ≫ 1 歩。後ろ向きの Dijkstra で、最良の費用から、歩数 +2 までを数える)。**ハブの数が最少のものだけ**を並べる(ハブが多い経路は、短くても並べない。ヒントで言う)。ヒントは、実装からの問い直し、`--only-impl`、`--avoid` を案内する。
- **`hub_skipped` の置き換え**: 一覧(`hub_skipped`)はやめた。2 段目で見つかったときは、ハブを使う経路は探しておらず、それを `searched` で言い、出会ったハブの呼び出し点の**総数**を `hub_skipped_total` に残す(G-1: 以前は、関数ごとに 3 件までしか数えず、少なかった。今は、呼び出し点を 1 度ずつ、全部数える)。1 段目で見つかったときは、動的な辺を見ていないので、書かない。`--hubs` で、ハブ込み(3 段目)から始められる(その場合は、ハブなしの経路が最初に出る)。
- **終点**: path の `Step` に `no_body`。終点(呼び出し点を持たない最後のステップ)に `defined_at`(宣言の位置。本体と位置がなければ省き、`no_body: true`)。呼び出し点が終点のときは `at` のまま。途中のステップには付けない。
- **短い指定**: 完全一致がなければ、全関数の `fid` / `symbol` を走査して、末尾一致を**識別子の境界**で見る(直前の文字が、文字・数字・`_` でないこと。`ByID` は `GetUserByID` に当たらない)。Go のメソッド `(*p.T).M` は `p.T.M` の形にして(括弧と `*` を除く)、Rust の `<T as Tr>::m` は `T::m` の形でも照合する。**全体で 1 つの symbol に決まるときだけ採用**(`resolved` に言う)。複数なら、エラーにせず `ambiguous`(`file:line` と同じ形。関数の候補は `id`、`at`、`no_body`、`dep`)と `ambiguous_total` を返す。並べる順は、自分のコード(`dep = 0`)→ 本体あり → 名前。上限 20 件(`MaxChoices`)。20 を超えるときは、`pkg.Name` のヒント。SQL の `LIKE` は使わない。全件走査の費用は、Gitea(関数 63,591)で、`graph callers Close`(319 件)が 53 ms(プロセスの起動を含む)なので、最後の名前の列も索引も足さない。
- **`flows: "not_checked"`**: すべての `graph` の出力に付く(`relation: "calls"` と並べて)。taint の層を DB に移すまでは、常にこの値。
- **辺のフィルタ**(`callers` / `callees` / `path` のすべて。繰り返し可): `--avoid <シンボル>`(その関数との間の辺を通さない)、`--via <シンボル>`(その関数を経由するものだけ)、`--only-impl <型>`(`interface` の辺は、行き先のメソッドの受け手の型が一致するものだけ。型は、メソッド名より前の部分に、識別子の境界で現れること。静的な辺と関数値の辺は絞らない)。シンボルは、短い指定と同じ規則で、**複数に決まるときはエラー**(絞り込みを、黙って選ばない)。どれも、`filters` に 1 行ずつ書く。
  - `path --via`: 起点 → via 1 → via 2 → … → 終点 を、区間ごとに探して、つなぐ(与えた順)。段(静的 / 動的 / ハブ)は、全区間で同じ段で探す。
  - `callers` / `callees` の `--via`: (関数, 通した via の集合)を状態にした幅優先。via を全部通った状態に着いたノードと辺だけを出す。通っていない状態の展開は、`MaxVisited` で切って、`warnings` に言う。`callees` は、フロンティアの先は通らない(ヒントで言う)。
  - `path` で `reachable: false` かつフィルタありのとき、フィルタなしで 1 回探し直し、`without_filters: {reachable, certainty, hops, searched}` と、ヒントで言う。費用は 1 回分。
  - `--only-impl` に、どの関数の symbol にも現れない型を渡したら、`warnings`(すべての interface の呼び出しが切れる)。
- **`path --from-entry --to X`**: 起点は `funcs.entry`(Go の `main.main` と各パッケージの `init`、Dart の `main`、Rust の `entry_fn`)。ただし、変数の初期化(`init`)だけのときは、プログラムの起点とは言わず、読んだコードの exported な関数(`dep = 0`)を足す(`sg.Program.Reachable` と同じ規則)。出力の `entry`: `definition`、`source`(`entry` / `exported`)、`count`、`langs`、`examples`。経路の最初のステップが、届いた entry。届かないときは、「フレームワークに登録されるハンドラーなどは、関数の値の呼び出しに候補として載っていなければ、届かない。死んだコードとは限らない」とヒントで言う。起点がなければエラー。`--from` と一緒には使えない。
- **測定(Gitea。関数 63,591。他のプロセスが動いていて揺れる。起動と DB を開く時間を含む)**:
  - 「経路なし」`path runWeb → libc.Int32FromInt32`: 段階 2 の 312 ms(ハブを使わない)が、3 段目(全部の辺)を足して 830 ms(`exhaustive: true`、`searched` に hub)。
  - `path runWeb → GetUserByID`(2 段目で見つかる): 292 ms(ハブ 29 点に出会って、使わなかった)。`--hubs` で 3 段目から: 191 ms。`--from-entry`(1,685 の起点)→ `GetUserByID`: 220 ms、→ `libc.Int32FromInt32`: 131 ms。`path runWeb → (*Session).Where`: 339 ms。`--via (*Session).Where` を付けて、経路なし(2 区間 × 3 段): 991 ms。`--avoid` と `--only-impl` つき: 383 ms。
  - 短い指定: `graph callers Close`(319 件、20 件を載せる)53 ms、`callers GetUserByID` 53 ms。`callers GetUserByID --depth 2 --via runWeb` 123 ms、`callees runWeb --depth 3 --avoid log.Error` 97 ms。
- **まだ決まっていないこと / 限界**: (1) 3 段目は、ハブが最少の経路だけを並べる。ハブが 1 つ多い経路は、並べない。(2) ハブのある探索(3 段目)は、ハブの先が「あらゆる実装」なので、見つかった経路が、実際に通るとは限らない(ヒントで言う。`impls` / `dispatchers` は、次の段階)。(3) `--via` が複数のときの、`callers` / `callees` の意味は、「全部を通る(順は問わない)」で、`path` は「与えた順」。(4) `--only-impl` は、型の名前を symbol の文字列で見る。同名の型が複数の package にあれば、区別しない(`pkg.Type` と書けば、区別できる)。(5) 経路が via の区間をつなぐとき、同じ関数を 2 回通る経路は、後ろに回す(なければ、そのまま出す)。

## 段階 3b で決まったこと(2026-11-21)

`graph impls` / `graph dispatchers`。構造の層(DB に書く辺)とスキーマ(`user_version` = 1)は変えていない。

- **読み方**: `edges`(`kind = interface`)を、呼び出し点の静的な型(`callsites.callee` = インターフェースのメソッドのシンボル)ごとにまとめる。`callee` に索引はないので、`callee = ?` は `callsites` の走査(Gitea 482,450 点)。`impls` は、`callsites` と `edges` を結んで読み(`callsite_id` の索引を使う)、`dispatchers` は `edges_to` の索引で、その関数に届く動的な辺を読む。
- **`impls` の指定**: 呼び出し点が持つ名前に完全一致、なければ、その末尾(識別子の境界。`forms` と `matchEnd` を、メソッド名の集合に使う)。**インターフェースのメソッド全体で 1 つに決まるときだけ採用**し、複数なら `ambiguous`(`MethodChoice`: `id`、`call_sites`。呼び出し点の多い順、最大 20)。名前が呼び出し点になく、関数としては解決できるときは、エラーにせず、空の結果と `reason` と `hints`(本体のある関数なら `dispatchers` を案内)。関数としても解決できなければ、`SpecError`(近い名前つき)。
- **`impls` の出力**: **「DB の中の、その呼び出し点の候補の和」であって、型としての実装の全部ではない**。`basis` に必ず書く(Go のメソッドなら VTA が呼び出し点ごとに絞ったこと、それ以外は、言語の extractor が呼び出し点ごとに出した候補であること。どの呼び出し点からも届かない型は載らない)。`call_sites`、`zero_candidate_sites`(候補が 0 の呼び出し点。Gitea の全体では、interface 3,694 + 関数値 937 = 4,631。`(error).Error` では 628 点のうち 170)、`impls_total`、`impls`(自分のコード → 呼び出し点の多い順 → ID)、`truncated`(unit `impls`)、`by_package`(切ったときだけ。全体の上位 10 と `by_package_other`。Go は symbol の import path、ほかはファイルのディレクトリ)。呼び出し点が 1 つもないときは `reason`。呼び出し点はあるが全部候補 0 のときも `reason`。
- **`dispatchers`**: 指定は `callers` の規則(シンボル、短い形、`ambiguous`)。呼び出し点を指す指定(`file:line`)はエラー。その関数に届く `kind` が `interface` / `value` の辺だけを、呼び出し点ごとに 1 つ(同じ呼び出し点の重複は除く)。`at`、`in`、`kind`、`candidates`、`callee`(通るインターフェースのメソッド = 静的な型。関数値のときは、その値の名前)、`sites`。**候補の少ない順**、次に位置の順(実装を名指しに近いものを先に)。`truncated`(unit `call sites`)、`by_callee`(上位 10 と `by_callee_other`)。同じ symbol の関数が複数あるとき(型の埋め込みで生じるラッパー。symbol が、埋め込まれたメソッドと同じ)は、ID が symbol と同じ**本物のメソッドだけ**を見て、ラッパーがあることを `hints` で言う(ラッパーを候補に持つ呼び出し点は、そのメソッドの呼び出し点ではないので、混ぜない)。
- **フィルタ**: `--avoid`(その関数を含む辺、その関数から呼ばれる辺を除く)と `--only-impl` は、辺の述語として使う(`filters` に書く)。`--via` は、意味がないのでエラー。`impls` の `call_sites` / `zero_candidate_sites` は、DB の全体の数(フィルタの影響を受けない)。実装ごとの `call_sites` は、フィルタを通ったもの。
- **測定(2026-11-21。起動と DB を開く時間を含む。ほかのプロセスが動いていて揺れる)**: Gitea(関数 63,591): `impls (error).Error`(595 実装、628 点、既定の 50 で切る)190 ms(初回 550 ms)、出力 7.3 KB。`--limit 5` で 2.5 KB。`impls (io.Writer).Write`(894 点、219 点が候補 0)80 ms、9.4 KB。`dispatchers (*bytes.Buffer).Write`(328 点)23 ms、13.4 KB(50 点を載せる)。`impls Write`(複数 → `ambiguous`)80〜180 ms。x/tools: 7〜37 ms、1.6〜2.4 KB。
- **限界**: (1) `impls` は、候補の和。候補が 0 の呼び出し点の先は、載らない。(2) `callee` は、呼び出し点が持つ名前の文字列。Go で、同じインターフェースを別の形で書いた名前(型の別名、埋め込み)は、別のメソッドとして数える。(3) `dispatchers` は、関数値の呼び出し点も含み、その `callee` は、インターフェースのメソッドではなく、値の名前(Rust と Dart では空のことがある)。(4) `--only-impl` と `--avoid` は、実装(行き先)の symbol で見る。

## レビューの指摘への対応(2026-11-21)

アーキテクチャ助言者のレビュー(G-4〜G-8)と、足したもの。構造の層(DB に書く辺)とスキーマ(`user_version` = 1)は変えていない。どの指摘も、直す前に落ちるテストを書いた(`internal/graphq/review_test.go`)。

- **G-4(`rank` が呼ばれていなかった)**: 2 つの関数の間に辺が複数あるとき、`best` が静的な辺しか優先せず、あとは SQL の順の先頭を残していた。候補 5 の動的な呼び出しと、候補 300 のハブが並ぶと、ハブが先に来れば、ハブが残り、3 段目で `hub: true` と誤った `candidates` が付き、`enumerateHub` の費用も `hubCost` になっていた。`rank` の小さい辺(静的、少ない候補の動的、ハブ)を残し、同じ `rank` は先頭。Gitea では、`--from-entry` の 400 点の無作為な終点(ハブ経由 66 点)と、「少ない候補の動的 + ハブ」が並ぶ 57 組の間で、修正の前後の出力は同じだった(SQL の順に助けられていた)。誤りの効く形は、テストのフィクスチャで確かめた。
- **G-5(`--via` の区間)**: 区間ごとに `want` 本を列挙し、`y.root == x.end()` でつないでいたので、`--via` の関数が複数の ID を持つとき(メソッドとラッパー)、前の区間の k 本が全部 ID A で終わり、次の区間が全部 ID B から始まると、つながる経路があっても空になり、`cut` のまま次の段へ進んでいた。**直し**: (1) 区間の起点を、前の区間の終点の集合に絞る。(2) それで届かない(探索は尽くした)ときは、via の点の ID のうち先へ届くものだけに絞って、最初からやり直す(1 つの点につき 1 回)。(3) 列挙が切れて見つからなかった段では、次の段へ進まない(`exhaustive: false`、`warnings` で言う)。静的な経路があるのに、`uncertain` / `hub` を返さない。
- **G-6(`--only-impl` の意味)**: 呼び出し点ごとに決める。**候補に、指定した型のメソッドが 1 つでもある呼び出し点だけ**を、その型のメソッドに絞る。候補に指定の型がない呼び出し点は、絞らない(「この Writer の実体は bytes.Buffer」が、経路の途中の `Stringer` や `error` を切らない)。`filters` に、出会った interface の呼び出し点のうち、絞った数と、絞らなかった数を書く(探索が進むにつれて数え、最後の数を言う)。型がどの symbol にも現れなければ、`warnings`(絞られる呼び出し点がない)。
- **G-7(型の判定)**: `receiverNames` が、メソッド名の前の部分のどこかに、識別子の境界で一致すればよかった。今は、**型の部分**だけを見る: Rust の `<T as Tr>::m` は `T`(trait ではない)、`Type::m` は `Type`。Go は `(*pkg/path.Type).M` の `pkg/path.Type`(パッケージ名だけの一致は当てない)。型引数(`Vec<Foo>` の `Foo`)は当てない。指定は、型の名前(最後の `.` / `::` / `#` の後ろ)に一致するか、経路を付けた形が、`/` `#` `::` の境界で終わること。
- **G-8(`without_filters` のヒント)**: フィルタつきの探索が上限で切れた(`exhaustive: false`)ときは、「フィルタが原因とは言えない」と言う。フィルタなしの探索が切れたときの「経路なし」も、「どちらも切れた」と言う(`without_filters.exhaustive`)。
- **`listed` / `beyond`**: `path` が並べた範囲を、ヒントでなく、フィールドで言う。`listed: {stage, hubs, shortest_hops, max_hops}`(`hubs` は hub の段だけ: 並べた経路のハブの数、最少)と、`beyond: "not_searched"`(より長い経路、ハブの多い経路は、探していない。`truncated.count` は `--limit` で切った数だけで、0 でも「全部」ではない)。`--via` のときは、区間ごとの最短の和。
- **`--avoid-call <file:line:col>`**: 呼び出し点 1 つの辺を、すべて通さない(`callers` / `callees` / `path` / `impls` / `dispatchers`、繰り返し可、`filters` に書く)。位置の規則は段階 2.5 と同じ(`Resolve`)。`--avoid`(関数を丸ごと切る)との違いは、呼び出しを含む関数と、その関数のほかの呼び出しが残ること。ハブの経路は「本物らしい順」ではない(候補 578 の `error.Error` 1 つより、候補 21 の呼び出し 2 つのほうが本物かもしれない)ので、エージェントが、1 本ずつ確かめて、疑わしい呼び出し点を切っていくために使う。
- **フィルタが 2 つ以上のときの `without_filters`**: 1 つずつ外した探索を足した(`culprits`: 外すと届くもの。ほかは残す)。費用は、フィルタの数だけ探索が増える。
- **見送ったもの**: なし(`culprits` は費用が小さいので入れた)。

- **到達の限界を、出力で言う(`--from-entry`)**: `path` の `exhaustive` は、DB の呼び出しの辺(呼び出しと、インターフェース・関数値の候補)についてのもの。`exhaustive_of` にそう書く。関数値の保存(`m.Get("/x", repo.Activity)`)、リフレクション、フレームワークからの呼び出しは、辺にない。**`--from-entry` で `reachable: false` のときは、`exhaustive` を `false` にして**、`limits` で上の 3 つを言い、`target_facts`(終点ごと: `static_call_sites`、`dynamic_call_sites`〔`dispatchers` 相当〕、`exported`、`has_body`、`dep`、`value_references: null`〔辺の表がまだない〕)を並べる。`edges_exhausted` は、辺の探索が上限で切れなかったこと(従来の `exhaustive`)。`--from` つきの「経路なし」は変えていない(同じ限界はあるが、`exhaustive_of` で範囲を言う)。`entry.definition` は、起点は `sg.Program.Reachable` と同じ規則で、辺は違う(関数値として名前を挙げた関数を辿らない)と書く。DB の辺と `user_version` は変えていない。参照の表と、参照を辿る 4 段目は、出力の形を見せてから別に作る。

## 参照(`refs` 表)と、`path` の 4 段目(2026-11-21。ユーザーが承認済み: C-3)

背景: `graph path --from-entry` は、Gitea の Web ハンドラ(`m.Get("/x", repo.Activity)` のように、関数値として登録される)に届かず、`reachable: false` と言っていた(限界は `limits` と `target_facts` で言っていた)。一方、taint 側の `sg.Program.Reachable` は、値として名前を挙げた関数(`in.Args` の `KFunc`)も辿る。同じ問いに 2 つの答えがあったので、**graph の規則を、`Reachable` に合わせた**。

- **スキーマ(`PRAGMA user_version` = 2)**: 表 `refs` を足した。`owner_func`(参照を持っている関数)、`file_id` / `line` / `col` / `offset`(参照が書かれた位置)、`target_func`(参照される関数。本体がなくても行になる)、`via_callee`(その参照を引数に取っている呼び出しの呼び先のシンボル。`(*web.Route).Get` など。参照が、代入や返り値のときは空)、`kind`(`arg`: 呼び出しの引数、`closure`: そこで作ったクロージャ〔値のキャプチャあり〕、`other`: 代入、返り値、構造体への格納など)。索引: `refs_target(target_func)`、`refs_owner(owner_func)`。`meta.layers` の `structure` の版も 2。**DB は使い捨てなので、マイグレーションしない**: 版 1 のファイルは、`graph` のどのコマンドでも、「版 1 で、この rivus は版 2 を読む。`rivus index` で作り直す」というエラーになる(テストあり)。`graph info` の `counts` に `refs`。
- **元の規則は `Reachable` と同じ**: 命令の `Args` の、`KFunc`(`make_closure` で、捕まえる値のないもの = 関数の名前そのもの)。呼び出しの引数なら `arg`、ほかの命令なら `other`。位置は、**その命令の位置**(呼び出しなら、Go は呼び出しの `(` の位置。`m.Get(` を書いた行。引数が複数行にわたっても、登録の文の最初の行)。スニペット(ソースの 1 行)は、DB に入れない(位置だけ。ユーザーの決定)。
- **`make_closure`(クロージャの生成)は、`Reachable` に辿られていない(コードで確かめた)**: 捕まえる値のない関数(`Args` が空の `make_closure`)は `KFunc` の値になり、辿られる。**捕まえる値のあるクロージャ**(関数リテラルが外の変数を使う、メソッド値 `x.Method`)は `KResult` で、`Reachable` は引数としても辿らない(`in.Static`〔その場で呼ぶクロージャ〕と、関数値の呼び出しの候補〔`valuecalls`〕を通るときだけ届く)。したがって、`m.Group("/x", func() {...})` や `t.Run("a", func(t *testing.T) {...})` のように、外の変数を使う関数リテラルを引数に渡すと、その本体は、`Reachable` に入らないことがある。x/tools(関数 9,541)では、`Reachable` が 5,758、graph が(クロージャの参照を含めると)6,212 で、**差の 454 関数は、すべて、この種のクロージャの参照を辿ったもの**(クロージャの参照を外すと、集合は `Reachable` と完全に一致する)。graph の側では、`refs` に `kind = closure` として足した(`sg` の `Reachable` は、この仕事では触らない。報告する)。**クロージャの本体は、作った関数の一部ではない**: Go の IR では、クロージャは別の関数(ID は `f$1` など)で、作った関数との間に辺はない(その場で呼べば静的な辺、`defer func(){...}()` と `go func(){...}()` も同じ)。そうでなければ、参照(`closure`)だけが、作った関数からクロージャの本体への辺になる。
- **`path` の 4 段目**: 静的 → 動的 → ハブ → **参照**。4 段目は、3 段目までで見つからないときだけ(`--refs` で 4 段目から。`--no-refs` で使わない。両方はエラー)。辺は、全部の呼び出しの辺と、参照。参照のステップは、呼び出しではないので、`kind: "ref"`、`ref_kind`、`via_callee`(あれば。`m.Get` に渡した、が分かる)、`note`(「この関数が、次の関数を値として持つ(登録、保存、受け渡し)。呼び出しではない」。英語)。**ステップの位置 `at` は、参照が書かれた位置(`file:line:col`。登録の行)**。経路の `certainty` は、参照を含めば `"reference"`(`certain` < `uncertain` < `hub` < `reference`)、**`relation` は経路ごと**(参照を含む経路は `"calls+refs"`、含まない経路は `"calls"`。出力の最上位の `relation` は、参照を含む経路があれば `"calls+refs"`、なければ `"calls"`。「経路なし」のときは `"calls"` のまま。探した範囲は `searched` と `exhaustive_of` が言う)、経路の `note` が「届くかもしれない、の広い近似(登録しただけで、呼ばれるかは分からない)。ref のステップは呼び出しではない」と言う。`searched` に `"ref"`、`listed.stage` が `"ref"`(`listed.refs` は並べた経路の参照の数 = 最少、`listed.hubs` も言う)、`beyond: "not_searched"`。**並べる順: 参照が少ない → ハブが少ない → 短い**(費用 = 参照 1 つ ≫ ハブ 1 つ ≫ 1 歩。3 段目の後ろ向きの Dijkstra に、参照の重みを足した。参照の数が最少の経路だけを並べる)。2 つの関数の間に、呼び出しと参照の両方があるときは、呼び出しを残す(`rank`: 静的、動的、ハブ、参照)。k 本で切り、`truncated` と `listed` / `beyond` は、この段でも出す。
- **`--from-entry` と `exhaustive`(判断)**: `reachable: true` のとき、最初のステップが、届いた entry(参照を使った経路でも)。`reachable: false` のとき、**`exhaustive` は常に `false`**(G-14 で、C-3 の「4 段目まで探して辺が尽きたら `true`」を取り消した)。理由: 反例 =「呼び元の辺も参照もない、自分のコードの関数」が、実行時には呼ばれる。依存の型(interface)のメソッドを実装していて、依存の側から呼ばれる形(`(*extractorMissingError).Unwrap` を `errors.Is` / `errors.As` が、`(*listFlag).Set` / `String` を `flag` パッケージが、`flag.Value` として呼ぶ)。読んでいないコードからの呼び出しは辺にない(C-4 の `overrides` の穴)ので、「辺にない」は「呼ばれない」と言えない。**探索が辺を尽くしたことは `edges_exhausted: true` が言う**(`exhaustive_of` の範囲の説明は残す: 呼び出しと参照の辺。`sg.Program.Reachable` と同じ規則)。`--from X --to Y`(起点を名指しした問い)の `exhaustive` は、今のまま(その問いの範囲で確か)。リフレクションと、フレームワークからの呼び出しは、`limits` に残す。`target_facts` に、`has_receiver`(メソッドである。シンボルから読む)を足し、メソッドのときは、ヒントで「呼び元の辺のないメソッドは、インターフェース経由で、読んでいないコード(依存、フレームワーク)から呼ばれうる」と言う。`--no-refs` のときも `limits` は、関数値の保存が辺でないことを言う。`target_facts` の `value_references` は、参照の数(null でなく、0 も言う)。`value_reference_owners` に、参照を持つ関数の上位 3 と、その数。`entry.definition` は、辺が `Reachable` と同じ規則になったことを言う。
- **`callers` / `callees`**: 参照を、既定では辿らない。辿らなかった分は、`omitted.refs`(展開して見せた関数の、持っている参照〔callees〕、参照されている数〔callers〕の合計。0 でも書く)と、`hints` で言う。`--refs` で、`kind: "ref"` の辺(`ref_kind`、`via_callee` つき。`candidates` と `sites` はない)として足し、`filters` に「refs added」と書く。ハブの関数値の呼び出しの候補(`kind: "value"` の辺)とは、別の辺のまま(混ざらない。テストあり)。`impls` / `dispatchers` は、参照を使わない(`--refs` はエラー)。
- **`Reachable` との整合(測定)**: 下の「測定」。Go のサンプルは、`Reachable` と完全に同じ集合(41 関数)。x/tools は、クロージャの参照を除けば完全に同じ(5,758)。参照を辿らなければ、`Reachable` にあって graph にない関数が 1,105(19%)あった。
- **限界**: (1) 位置は、命令の位置(Go の呼び出しは `(`)。引数の位置ではない(`KFunc` の値に、使った位置がない)。(2) `via_callee` は、呼び出しの呼び先のシンボル(インターフェースのメソッドなら、そのメソッドの名前)。引数のうちどれか、は言わない。(3) 参照は広い近似: 値として持ったことが、呼ばれることを意味しない。ハブのように「あらゆる実装」ではないが、`m.Get` のようなルータのハンドラは、実際に呼ばれることが多い。(4) 参照の先の関数が呼ばれたかは、辺にない。参照を含む経路は、「届くかもしれない」。(5) クロージャの参照は、本体を作った関数が持つ(その関数の中で `defer func(){...}()` のようにその場で呼ぶときは、静的な辺と参照の両方がある。経路は、辺を先に選ぶ)。
- **測定(Go のサンプル、x/tools)**: 上の整合の数。Gitea(DB のサイズ、`rivus index` の時間、`routers/web/repo` のハンドラ)は、ほかのプロセスのメモリ不足(スワップ満杯)で、この変更の時点では測れなかった。測ったら、ここに足す。

## これから決めること
- `hints` の文面(段階 2 は、英語の短い文。使ってみて直す)と、`path` の出力を、トークンが問題になったときにノードの表に分けるか(今は、各ステップに関数の ID を書く。1 辺が約 190 バイト)
- サイト ID の連携(今は、辺とステップに `sites` を付けるだけ)
- git の revision をどう添えるか(`dirty` をやめたので、作業ツリーの変更は、`source_hash` で表す。今は `meta.revision` に HEAD を入れるだけ)
- taint の層(findings、不確実性)を、いつ DB に移すか。移すときは、`layers` に層を足し、層ごとの版・作成日時・ハッシュで、構造の層とのずれを検出する。移したら、**`findings_at_sites`**(経路の呼び出し点にあるサイトを source / sink とする finding)を出す(**決定済みの名前**。`also_flows` にはしない。ユーザーが決めた)。taint の層がないときは、今のまま `flows: "not_checked"`。この橋と、層の DB への移行は、まだ作っていない
- ハブ(`log.Error` のような、1,800 の呼び元)の `callers` を、どう見せるか(今は、`--limit` で切って、数を言う。呼び元を関数でまとめる、パッケージでまとめる、など)
- `callsites` の builtin は、呼び出し点の約 4 割(Gitea: 全 482,450 のうち 203,578)。今は、DB に入れたまま、検索で外している(`kind` の索引はない)。DB から落とすかは、サイズ(91 MB)が問題になったら

## graph の細かい指摘の直し(G-9 から G-16。アーキテクチャ助言者のレビュー 2026-11-21)

- **G-9**: `--avoid-call` の `filters` に、解決した呼び出し点(`file:line:col`)、呼び先、種類、候補の数、切った呼び出し点の数を書く。指定の桁から始まる呼び出しがなく、前の呼び出しに寄せたときは、寄せたと書く。
- **G-10**: 同じ関数・同じ位置の呼び出し点(Rust の `?` の脱糖など)は、全部切る(`filters` に数)。
- **G-11**: 位置のない(`line` が 0 の)呼び出し点・参照のステップは、`at` を出さず(`file:0:0` と書かない)、`at_unknown: true` を出す。`defined_at` も同じ。`--avoid-call` で切れないので、`--avoid <関数>` を使う。
- **G-12**: `without_filters.culprits` が空(2 つ以上のフィルタが、同じ経路を一緒に塞いでいる)のときは、「1 つ外しただけでは届かない。2 つ以上が塞いでいる」と言う。フィルタが 1 つのときは、「そのフィルタが塞いでいる」。
- **G-14**: 上の `--from-entry` と `exhaustive` の項。
- **G-15**: Go の `make_closure`(`ssa.MakeClosure`)は、go/ssa が位置を持たない。`internal/frontend/goir` が、関数リテラルの位置(`func` の位置)を、その命令の位置にする。Dart の `make_closure` はリテラルの位置、Rust の `make_closure` は、関数をそのまま値にした(`FnDef`)ときだけで、オペランドの位置を持つ。IR のスキーマは変えない。`refs` の `closure` に、位置が入る(DB の作り直しが要る)。
- **G-16**: 参照の位置は、命令(文)の位置で、引数や複合リテラルの要素の位置ではない(複数行の呼び出しでは、文の最初の行。ルートの表 `[]Route{{"/a", A}, {"/b", B}}` では、全部が同じ位置)。参照のステップに `at_precision: "statement"` を付け、ヒントで「どの引数かは、ソースを読んで」と言う。**案(今回はしない)**: IR の `Instr` に、引数ごとの位置(`ArgPos []Pos`)を持たせ、3 つの extractor が付ける。IR のスキーマと、3 つの extractor の変更なので、大きい。小さい案は、関数を値にする `make_closure`(Go の `val` の `*ssa.Function` の分岐)に、その関数名の位置を付けること。
