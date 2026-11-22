# 依存を含む Dart のプロジェクトの解析コスト(合成の gRPC サーバでの測定)

状態: 測定の記録と案(実装はしていない)。2026-11-22、origin/main の `7e7143f` を基に、`bench/dart-grpc` で測った。

## 1. 目的と対象

依存を含む Dart のプロジェクトで、解析が終わらないことがある。別の対象の測定(dart-pad の `dart_services`)は、28,072 関数、10 GB、9 分超で時間切れだった(`analyzer` 系の最大の SCC 12,130 関数が主因らしい)。Dart の gRPC サーバは代表的な利用の形なので、最初の対象にする。小さい合成のサーバ(`bench/dart-grpc`、README 参照)で、次を調べた。

- 依存込み(既定)で、現実的な時間で解析できるか。どの依存が重いか。
- `grpc` / `protobuf` / `http2` の本体を読まなくても(`-no-deps`)、サービスのメソッドから sink までの経路が出るか。
- 生成された protobuf のコードの規模と置き方で、コストがどう変わるか。

サーバは実行していない(解析だけ)。新しい第三者のパッケージは取得していない(`dart pub get --offline` だけ。`bench/dart-corpus` も同じ)。rivus は `7e7143f` のビルド、Dart の extractor は同じ worktree のもの。時間とメモリは `/usr/bin/time -l`(wall、最大 RSS = 子プロセスの extractor を含む、`peak memory footprint` = rivus 本体)。`-timeout 540s` を付けたが、時間切れになった実行はなかった。測定は 1 回ずつ(ばらつきは測っていない)。

合成サーバ: 3 サービス、7 メソッド(unary 5、サーバストリーム 1、クライアントストリーム 1)。vuln 3(`Files.Read` = パス、`Jobs.Run` = コマンド、`Reports.Fetch` = SSRF)と fixed 4(定数のパス、表の値、キーの参照だけ)。メッセージとサービスの基底は、protoc_plugin の出力の形で手で書いた。

## 2. (a) 依存込み(既定)の解析

`rivus analyze -lang dart -C bench/dart-grpc -pass-stats`

| 項目 | 値 |
| --- | --- |
| wall | 10.0 秒(解析の `duration_ms` は 153。残りは extractor と IR の読み込み。内訳はこの実行では分けていない) |
| 最大 RSS / footprint | 1.15 GB / 146 MB |
| 関数(解析した数) | 2,396(IR にあるのは 5,113。残りは到達できないので落ちる) |
| `levels` / `largest_scc` | 32 / 195 |
| `summary_elems` | 5,441 |
| `dep_spent_ms` / `own_spent_ms` | 526 / 7 |
| 時間切れ / 収束 | なし / 収束(4 パス) |
| finding | 3(vuln の 3 件)。fixed の誤検出 0 |

### どの依存が重いか(`packages_top`、上位 20 件)

| パッケージ | 解析した関数 | `reanalyses` | `summary_elems` | `spent_ms` | IR の関数(依存を全部読んだ数) |
| --- | ---: | ---: | ---: | ---: | ---: |
| protobuf | 521 | 761 | 1,225 | 64 | 857 |
| http2 | 407 | 582 | 2,038 | 145 | 473 |
| collection | 277 | 574 | 227 | 84 | 648 |
| grpc | 259 | 378 | 687 | 91 | 773 |
| (自分のコード grpc_bench) | 307 | 307 | 284 | 7 | 307 |
| async | 138 | 261 | 138 | 43 | 485 |
| typed_data | 96 | 215 | 15 | 15 | 139 |
| source_span | 110 | 154 | 402 | 15 | 150 |
| crypto | 72 | 99 | 69 | 29 | 115 |
| path | 85 | 98 | 159 | 7 | 193 |
| http_parser | 22 | 70 | 156 | 25 | 81 |
| fixnum | 42 | 43 | 13 | 2 | 147 |
| term_glyph / clock / googleapis_auth / http | 30 / 10 / 6 / 2 | 30 / 10 / 6 / 2 | 4 / 3 / 10 / 4 | 0 | 189 / 50 / 174 / 128 |

この対象では、重さは http2(`spent_ms` 145、`summary_elems` 2,038)、grpc、collection、protobuf の順で、全部で 0.5 秒。重い依存は無い。`packages_top` は上位 20 件で、ここでは `spent_ms` が 0 のものまで入っていて足りている。大きい対象(後述の corpus)では 20 件で切れるので、全件を出すには、`stats.packages_top` の件数の上限を、フラグ(例: `-pass-stats-all`)で外す変更が要る(今は上限 20 で固定。変更は案だけ)。→ その後、`-pass-stats-top N`(0 で全件)と `-pass-stats-sort` を足した(ADR-061 の追記)。

IR の関数の数は `rivus_dart` の出力の `kind: func` を、`package:<名前>` ごとに数えた。ここまでは extractor が依存を全部出し、コアが到達できるものだけ解析に残す(`extract.dart` の読み取りの順序)。

### 巨大な依存: `bench/dart-corpus` の IR(解析ではなく関数の数だけ)

54 の人気パッケージ(88 パッケージ)の IR は 51,336 関数、77 の区分(`dart:` と `file:` を含む。上位 20 を示す)。

| パッケージ | 関数 | パッケージ | 関数 |
| --- | ---: | --- | ---: |
| analyzer | 25,018 | file | 623 |
| _fe_analyzer_shared | 4,478 | petitparser | 577 |
| archive | 2,320 | built_collection | 566 |
| pointycastle | 1,975 | async | 486 |
| dart_style | 1,469 | http2 | 473 |
| rxdart | 999 | charcode | 423 |
| postgres | 997 | dio | 385 |
| protobuf | 857 | grpc | 773 |
| sqlite3 | 835 | xml | 728 |
| intl | 659 | collection | 649 |

`analyzer` と `_fe_analyzer_shared` で 29,496 関数、全体の 57%。`dart_services` の 28,072 関数の大半も、同じ 2 つが占めるはずだが、これはその対象の IR を数えていないので推測。

## 3. (b)(c) `-no-deps` と依存込みの差

同じ合成サーバ(7 メソッド)。

| 項目 | 依存込み | `-no-deps` |
| --- | --- | --- |
| wall / 最大 RSS / footprint | 10.0 秒 / 1.15 GB / 146 MB | 6.0 秒 / 1.14 GB / 21 MB |
| 関数 / `largest_scc` / `levels` | 2,396 / 195 / 32 | 294 / 1 / 8 |
| finding | 3(vuln 3 件) | 3(同じ 3 件) |
| fixed の誤検出 | 0 | 0 |
| 不確実性の注記の合計 | 273 | 365 |
| `dynamic_dispatch`(実装が分からない) | 7 | 151 |
| `dynamic_dispatch`(インターフェースの呼び出し) | 202 | 58 |
| `unmodeled_library_call` | 9 | 103 |
| `unresolved_callee` | 48 | 48 |
| `numeric_stop`(int / bool のフィールド) | 3 | 3 |
| `assumed_call` / `depth_limit` | 2 / 1 | 1 / 1 |
| `dependency_body_unread` | 1 | 0 |
| finding の経路についた注記(3 件とも) | 各 4 | 各 2 |
| 確信(confidence) | 3 件とも `requires_review` | 同じ |

- 7 メソッドのどれも、入口(`Service` を継承したクラスのメソッド)として一致する(`rivus sites` で確認。`_Pre` は除外される)。`Files.Write`(クライアントストリーム)も、確認のため一時的に `File(r.path)` に変えると、依存込みと `-no-deps` の両方で finding が出た(確認後に戻した)。
- vuln の経路は、`-no-deps` でも出る。リクエストのフィールドの読み出し(`request.path` = 生成された getter が `$_getSZ(0)` を呼ぶ)は、本体を読まなくても汚染を保つ。`$_getSZ` はモデルも本体も無い呼び出しで、既定(レシーバと引数が結果へ流れる、`unmodeled_library_call`)が働くため。経路の注記は `dynamic_dispatch`(「実装が分からない」)。
- 差(依存の本体を読んで得ている情報)は、finding には出ない。出るのは注記の種類と数(`unmodeled_library_call` +94、実装が分からない呼び出し +144)と、確信の根拠だけ。この対象では、`grpc` / `protobuf` / `http2` を、本体を読まずモデルで扱う、ことは、finding の面では支障がない。

### 3.1 見逃す形: メッセージへの書き込みのあとの読み出し

リクエストを新しいメッセージに入れて(setter)、そのフィールドを sink に渡す形を試した。`own8`(メッセージ 16 個、`gen/gen.dart` の出力を改めたもの)で、次を見た。

| 形 | 依存込み | `-no-deps` |
| --- | --- | --- |
| A `m.text = request.name; sink(m.text)` | 見逃す | 見逃す |
| C `Msg()..name = request.name; sink(m.name)`(cascade) | 見逃す | 見逃す |
| D `m.tags.add(request.name); sink(m.tags.first)`(repeated) | 見逃す | 見逃す |
| E `m.child = request.child; sink(m.child.name)`(入れ子) | 見逃す | 見逃す |
| B `Msg12(text: request.name)`(コンストラクタ) | 見逃す | 見逃す |
| 同じ形を、普通のクラスの getter と setter(`Box`)で | 出る(F) | 出る |

- 依存込みでも見逃す。注記は `dynamic_dispatch` と `unresolved_callee`(setter と getter の同名による余分な候補)だけで、見逃しの原因を指す注記はない。つまり、依存の本体を読んでも、`$_setString` から先でフィールドが書かれる流れが失われる(原因は、`_fieldSet` の中の汎用のコンテナの経路と思われるが、読みきっていない)。
- `-no-deps` の側は、`$_setString` が本体もモデルもない呼び出しで、既定では引数がレシーバへ書かれない(結果へだけ流れる)ため。

つまり「リクエストのフィールドを読む」は両方で保たれるが、「メッセージのフィールドに書いて、読む」は、依存込みでも保たれない。

## 4. 生成された protobuf のコードの規模と置き方(追加の測定)

`gen/gen.dart` で、メッセージ N 個(フィールド 9〜23、入れ子、repeated、map、oneof、enum を含む。1 個あたり 約 270 行、関数 約 85)と、N/4 メソッドのサービスを作った(出力は N=300 で 2.6 MB。`dart analyze` で型の整合は確認済み。コミットしない)。`vuln` のメソッド 2 つ(`Op0` = コマンド、`Op1` = 入れ子のメッセージのフィールドからパス)を含む。

### 4.1 規模(生成コードが自分のコード = 同じパッケージ)

| N | 依存 | wall | 最大 RSS / footprint | 解析した関数 | `largest_scc` | `summary_elems` | `own_spent_ms` | finding |
| --- | --- | ---: | --- | ---: | ---: | ---: | ---: | ---: |
| 0(合成の 7 メソッド) | 込み | 10.0 秒 | 1.15 GB / 146 MB | 2,396 | 195 | 5,441 | 7 | 3 |
| 50 | 込み | 11.5 秒 | 1.14 GB / 198 MB | 6,627 | 195 | 10,130 | 132 | 5 |
| 50 | `-no-deps` | 7.3 秒 | 1.13 GB / 95 MB | 4,462 | 1 | 3,337 | 66 | 5 |
| 300 | 込み | 16.0 秒 | 1.70 GB / 587 MB | 27,692 | 195 | 33,510 | 478 | 5 |
| 300 | `-no-deps` | 11.3 秒 | 1.51 GB / 494 MB | 25,277 | 1 | 19,400 | 389 | 5 |

- メッセージ 1 個あたり 約 84 関数。解析の時間(`duration_ms` + 自分のコードの `own_spent_ms`)は、N に対しておおよそ線形(1 メッセージ 1〜1.5 ms)。**`largest_scc` は増えない**(195 は依存の分のまま)。生成コードは SCC を作らない。
- この規模(300 メッセージ、27k 関数)でも、解析のフェーズは 1 秒未満、wall の大半は extractor(N=300 の自分のコード配置で、extractor だけで 14.5 秒、最大 RSS 1.7 GB。別測定)。メモリは footprint が N=300 で 587 MB(1 メッセージ 約 1.1 MB)。
- つまり、この合成では「生成された protobuf のコードの量」だけでは、終わらない解析にならない。終わらないほどの重さは再現できなかった。実際の対象の重さの原因が、生成コードの量でなく別のもの(コードの構造)であるかは、この測定では分からない。

### 4.2 置き方(N=300、メッセージ 25,206 関数)と `-C` の指し方

| 置き方 / 指し方 | wall | 最大 RSS / footprint | 解析した関数 | `largest_scc` / `levels` | `summary_elems` | 自分のコードの関数 | finding |
| --- | ---: | --- | ---: | --- | ---: | ---: | ---: |
| (1) 自分のコード、`-C own` | 16.0 秒 | 1.70 GB / 587 MB | 27,692 | 195 / 32 | 33,510 | 25,589 | 5 |
| (2) path の依存、`-C app -allow-deps-under mono` | 13.9 秒 | 1.57 GB / 514 MB | 7,298 | 195 / 32 | 11,458 | 383 | 5 |
| (2) に `-no-deps` | 6.7 秒 | 1.12 GB / 24 MB | 370 | 1 / 8 | 130 | 368 | 3(Bulk の 2 件が消える) |
| (2) で `-allow-deps-under` なし(N=50) | 10.2 秒 | 1.14 GB / 143 MB | 2,409 | 195 / 32 | 5,441 | 319 | 3(同上) |
| (3) `-C mono`(ルート全体)`-allow-deps-under mono` | 19.6 秒 | 2.23 GB / 528 MB | 25,277 | 1 / 8 | 19,925 | 25,275 | 5 |

効いている理由(測って確認したこと):

- **到達できる関数だけが対象になる: 当たり。** (2) は、extractor は path の依存の 25,206 関数を全部出すが、コアは到達できる 4,815 関数だけを解析に残す(`packages_top` の `bulk_protos`)。(1) は、自分のコードの 25,589 関数を全部解析する(自分のコードは到達の絞り込みがない)。解析した関数は 27,692 対 7,298、`summary_elems` は 33,510 対 11,458、解析のフェーズは 451 ms 対 233 ms。
- **自分のコードの関数がすべて入口の候補になる: 外れ。** 入口(source)の数は、(1) も (2) も 84 で同じ(`rivus sites`)。差を作っているのは、入口の数ではなく、自分のコードのすべての関数を解析する点。
- **使われないパッケージが対象から外れる: この測定では当てはまらない**(全部のパッケージが、サービスから使われる)。測れなかった。
- wall の差は小さい(16.0 秒 対 13.9 秒): extractor が、どちらでも全部を読んで出すため。到達で絞るのは、コアの側だけ。extractor の側(読む量、メモリ 1.5〜1.7 GB)は変わらない。
- `-C app` で、path の依存が許可の外のとき(`-allow-deps-under` なし)は、その依存を読まない。Bulk の vuln 2 件が消える(finding 5 → 3)。報告は 6 節。
- (3) `-C mono`(ルート全体)は、`grpc` / `protobuf` / `http2` が読まれていない(解析した関数 25,277 は `-no-deps` の (1) と同じ数、`largest_scc` 1)。ルートに `.dart_tool/package_config.json` が無いためと思われるが、理由は調べていない。この指し方でも finding は 5 件、時間は短いが、依存を読んでいないので、「依存込み」の解析ではない。**短時間で終わる理由の一つになりうる**(依存の本体を読まないため)が、実際の対象でこれが起きたかは分からない。

## 5. (c) 生成された protobuf のコードを安く扱う案

読み出しは、すでに本体なしで保たれる(3 節)。足りないのは、(i) 書き込み → 読み出しの往復と、(ii) フィールドごとの区別。

測定: `-rules` で、次の 3 行だけのモデルを足して、3.1 の形を試した(`own8`、依存込みと `-no-deps` の両方で同じ結果)。

```yaml
propagators:
  - symbol: package:protobuf/src/protobuf/internal.dart#GeneratedMessage.$_setString
    flows: [{from: arg1, to: recv}]
  - symbol: package:protobuf/src/protobuf/internal.dart#GeneratedMessage.$_setField
    flows: [{from: arg1, to: recv}]
  - symbol: package:protobuf/src/protobuf/pb_list.dart#PbList.add
    flows: [{from: arg0, to: recv}]
```

| 形 | 結果 |
| --- | --- |
| A、C、E(setter のあとの読み出し) | 出る(往復が戻る) |
| D(`tags.add` のあと `tags.first`) | まだ出ない(`$_getList` が呼ぶたびに別の値を返すためと思われる。確認していない) |
| H1: 同じメッセージの、別のフィールド(`h.status.name`) | **誤検出になる**(fixed のはずが vuln) |
| fixed: メッセージに汚染と定数を別のフィールドに入れ、定数のほうを sink に | **誤検出になる** |

- **モデルだけ(`to: recv`)では、フィールドごとに保てない**。`flows` は値の全体にしか名指しできない(`MODELING.md` 4 節のとおり、`to: arg0.Field` は今は書けない)。往復は戻るが、メッセージ全体が汚染され、fixed が 2 件 vuln になる(精度が落ちる)。
- 普通のクラスの getter / setter は、今のコアでもフィールドごとに保つ(`Two` というクラス: `a` を汚染、`b` を定数。`b` を sink に渡した行は出ず、`a` の行が出た)。つまり、コアの仕組みは足りていて、生成されたアクセサだけが、インデックスで指定する形(`$_getSZ(0)`)のため、フィールド名に結びつかない。

案(軽い順):

1. **extractor で、`GeneratedMessage` の派生クラスのアクセサを、フィールドの読み書きに落とす。** `get x => $_getSZ(i)` / `$_getIZ` / `$_getBF` / `$_getN` / `$_getI64` は、そのクラスのフィールド `f<i>` の読み出し、`set x(v) => $_setString(i, v)` などは `f<i>` への書き込みとして、IR に出す(本体は、依存の `protobuf` を読まなくても、プロジェクトの `*.pb.dart` の中にあるので、extractor が見える)。コアの既存のフィールド別の扱い(上の `Two`)がそのまま使え、フィールドごとの区別と往復の両方が付く。`$_getList` / `$_getMap` / `$_ensure` は、コンテナ(または入れ子のメッセージ)のフィールド `f<i>` の読み出しにして、`PbList.add`(`arg0 → recv`)、`PbMap.[]=`(`arg1 → recv`)、`PbList.first` / `[]`(`recv → result`)は組み込みのモデル(`flows`)で足す。必要: extractor とコアの変更(小さい)。`flows` / `holds` だけでは足りない。
2. **モデルだけの縮小版**(精度を落としてよいなら): 上の 3 行 + `$_setInt64` などを足す。往復は戻るが、メッセージ全体が汚染される。上の測定のとおり誤検出が出るので、勧めない。
3. `BuilderInfo` の構築、`create` / `createEmptyInstance` / `getDefault`、`fromBuffer` / `mergeFromBuffer` の本体は、読まなくてよい(`dart-grpc.yaml` の隣に、`fromBuffer`: `arg0 → result` と `writeToBuffer`: `recv → result` のモデルを置く案)。今回の測定では、これらを読んでも finding は変わらない。コストは小さいので、案 1 のあと、必要が出たら。

フィールドごとに保てない場合の不確実性の注記: 案 2 を採るなら、往復の流れにつく注記として、`field_imprecision` に近い注記(「この型のフィールドは区別していない」)を出すのが妥当(注記の名前と出し方は、この文書では決めない)。

## 6. (d) 利用者への案内

モノレポで、アプリのパッケージを `-C` で指し、path の依存を `-allow-deps-under` で許可する使い方が、今の文書で分かるか。

- `rivus analyze -h` の `-allow-deps-under`: 「プロジェクトの外のディレクトリで、そのパッケージを依存として読んでよいもの。`package_config.json` は信用しない。プロジェクト、pub のキャッシュ、SDK だけが、頼まなくても読まれる(モノレポの兄弟のパッケージは、これが要る)」と書いてある。モノレポの兄弟のパッケージ、の例まで書いてあるので、分かる。
- `README.md` 212 行: 同じ内容が 1 行ある。`docs/ARCHITECTURE.md` にも起動の形の説明がある。ただし、「アプリのパッケージを `-C` で指し、兄弟を許可する」の手順の例はない。

読まれなかったとき、rivus は何と報告するか(合成のモノレポ `mono50/app`、`-allow-deps-under` なしで実測):

- 標準エラー: `rivus: extracting dart: ...` の 1 行だけ。読まれなかったことは出ない。
- 出力の JSON: `uncertainties` に `missing_source`(`attributable_to: target`)が 1 件。文は `a dependency outside the project, the pub cache and the SDKs is not read (what package_config.json points to is not trusted): --allow-deps-under <mono のディレクトリ> reads it`。**`-allow-deps-under` を案内する 1 文は、ここにある**(`extract.dart` の `loadError`、`_advisedDir` が親ディレクトリを示す)。
- `summary.unread` は `null`(この種類は `unread: true` が付かない)。`summary.program.by_kind` には、`missing_source` が数に入る。finding は、依存の側の経路が読めないので、一部が消える(この測定では 5 → 3)。
- 見落としやすい点: 標準エラーに何も出ない。出力の JSON の注記だけを読む人は少ない。

足す案(実装はしていない):

1. `cmd/rivus/analyze.go` の、`unread` を標準エラーに出す箇所(`rivus: %d package(s) or file(s) were read and not built ...`)の隣に、`missing_source` を数えて 1 行出す: `rivus: N dependency package(s) outside the project were not read; their code is missing from the result (finding may be lost). Allow them with -allow-deps-under <DIR> (the DIR is in the note of kind missing_source)`。
2. `summary.unread` に、同じ件数とディレクトリの一覧(`unread: true` を付ける、または別の欄)を足す。
3. `README.md` の 212 行の隣に、モノレポの例を 2 行: `rivus analyze -lang dart -C app -allow-deps-under .`(アプリのパッケージを指し、ルート以下を許可する)。

## 7. (d) 最初に作るモデルの案

| 対象 | 種類 | 理由(測定) |
| --- | --- | --- |
| `package:analyzer`、`package:_fe_analyzer_shared`(と、利用者のテイントに関係しない巨大なもの) | 本体を読まない指定(今は仕組みがない) | `bench/dart-corpus` で 29,496 関数、全体の 57%。別の対象で、最大の SCC の主因らしい(推測)。ADR-092 の 1〜3 の条件を満たす測定が要る(この合成サーバには出ないので、`dart_services` 型の対象で、読む場合と切る場合を比べる)。`analyzer` は、利用者のビジターを呼ぶ(`RecursiveAstVisitor` など)ので、`calls:` の行が要る(条件 3) |
| 生成された protobuf のアクセサ(`$_getSZ` ほか) | extractor の扱い(5 節の案 1)と、`PbList` / `PbMap` の `flows` | 本体なしでも往復とフィールドごとの区別が付く。モデルだけでは、誤検出が出る |
| `protobuf` の `fromBuffer` / `writeToBuffer` / `mergeFromBuffer` | `flows` | 5 節の案 3。コストでなく、注記を減らすため |
| `grpc`、`http2` | 今は切らない | この合成サーバで、依存の分は全部で 0.5 秒。入口は、`Service` を継承したクラスのメソッドから出るので、本体を読まなくても出る。切るなら、`ServiceMethod` に渡したハンドラを `calls:` で書く必要(ライブラリが利用者のコードを呼ぶ) |
| `dart:*` | 今のまま | — |

### 「本体を読まない指定」の最小の仕組みの案

ADR-092 の決めたこと 5(利用者が本体を読ませるフラグ)と `MODELING.md` 4a 節のとおり、今は、全部読むか、全部読まないか(`-no-deps`)だけで、パッケージ単位の指定がない。最小の案:

1. ルールのファイルに、パッケージの一覧を書く欄(例: `unread_packages: [package:analyzer, package:_fe_analyzer_shared]`)。組み込みでも、利用者のルールでも書ける。
2. コアが、IR を読み込むとき、一覧のパッケージの関数を、本体のない関数(今の `-no-deps` の依存と同じ)として扱う。呼び出しは、モデルがなければ、既定(引数とレシーバ → 結果、`unmodeled_library_call`)。出力の `summary` に、切ったパッケージを示す(ADR-092 の 5)。
3. (任意)extractor に同じ一覧を渡して、そのパッケージを出力しない。上の測定では、時間の大半が extractor なので、コアだけの変更では、`dart_services` 型の「解析が終わらない」は直るが、extractor の時間とメモリ(1.5〜1.7 GB)は減らない。
4. 照合の検査(ADR-092 の 2): 切る場合と読む場合の finding と境界のエッジの一致を、フィクスチャで比べる。

## 8. 測れなかったこと

- 「終わらない解析」の再現: 合成では、規模を 300 メッセージ(27k 関数)まで上げても 16 秒で終わった。`dart_services` の 9 分超は再現していない。原因が生成コードかどうかは分からない。
- 巨大な SCC の中身(パッケージごとの関数の数): 今の出力にはなく、IR の関数の数(2 節、パッケージ全体の数)で代えた。SCC の内訳そのものは測っていない。
- 依存込みでの `analyzer` 系の解析時間(この合成には出ない。corpus は IR の関数の数だけ)。
- wall の中の、extractor と IR の読み込みの割合: N=300 の自分のコード配置で extractor だけを測った(14.5 秒 / 16.0 秒)。ほかの実行では分けていない。
- `-C` にルートを指した場合に、依存が読まれない理由(4.2 の (3))。
- 測定は 1 回ずつ。ばらつきは測っていない。
- 使われないパッケージの扱い(4.2 の理由 3 つ目)。

## 9. 再現

```sh
cd bench/dart-grpc && dart pub get --offline
bench/dart-grpc/gen/layouts.sh 300 /tmp/out          # own と mono の 2 つの配置
rivus analyze -lang dart -C /tmp/out/own -pass-stats
rivus analyze -lang dart -C /tmp/out/mono/app -allow-deps-under /tmp/out/mono -pass-stats
```

`RIVUS_DART_EXTRACTOR="dart run <worktree>/extractor/dart/bin/rivus_dart.dart"` を設定する。

## 10. Google Cloud の Dart のクライアントを足した測定(`bench/dart-gcp`)

状態: 測定の記録と案(実装はしていない)。目的は、依存を含む Dart のプロジェクトで解析が終わらないことがある(公開の測定は 1 節の `dart_services`)原因を、Google Cloud のクライアントのパッケージに探すこと。結論を先に書く: **この合成では、どの依存を足しても収束した**。Google Cloud のパッケージは、解析(コア)のコストを増やさず、最大の SCC も大きくしなかった。増えるのは extractor の時間とメモリで、これは読むファイルの数に比例する。

### 10.1 測定の条件

| 項目 | 値 |
| --- | --- |
| rivus | `c40fcca` のビルド(`go build -o <scratchpad>/rivus ./cmd/rivus`)。コアと extractor は同じ worktree |
| Dart SDK | 3.13.7 (stable)、macos_arm64 |
| 対象 | `bench/dart-gcp`(README 参照)。`pubspec.lock` は `bench/dart-gcp/pubspec.lock`(コミット済み) |
| コマンド | `rivus analyze -lang dart -C <dir> -pass-stats -pass-stats-top 0 -pass-stats-sort time -detail summary -timeout 540s`。1 回の実行の上限は 10 分(どの実行も上限に届かなかった) |
| 計測 | `/usr/bin/time -l`(wall、最大 RSS = 子プロセスの extractor を含む) |
| 取得したパッケージ | `dart pub get` で解決した、`googleapis` 15.0.0、`gcloud` 0.9.0、`google_cloud_storage` 0.6.4、`google_cloud_protobuf` 0.5.5、`google_cloud_rpc` 0.6.0、と、その推移的な依存(`_discoveryapis_commons` 1.0.8、`convert` 3.1.2、`ffi` 2.2.0、`retry` 3.1.2)。`googleapis_auth` 2.3.7 と `google_cloud` は pub のキャッシュにあった。取得したコードは実行していない(読むのは rivus の extractor と `dart analyze` だけ) |
| 時間の読み方 | **測定中、この機械は他の作業で負荷が高かった(load average 20 以上)。wall は 2 回目の測定で 1〜2 割ずれた(基準の `dart-grpc` は 17.3 秒と 19.2 秒。2 節の 10.0 秒は別の時の値)。比べるときは、wall の差より、関数の数、`summary_elems`、`duration_ms`(解析の本体)、最大 RSS を見る** |

### 10.2 1 つずつ足して測る(依存込み)

変種は、`bench/dart-gcp` から、サービスと `pubspec.yaml` を削って作った(scratchpad。リポジトリには、全部入りの `bench/dart-gcp` だけを置く)。行は上から足していく。`largest_scc_packages` の上位 3 は、その行で最大の SCC のものだけ書く。

| # | 足したもの | 解析した関数 | converged / passes | `largest_scc`(上位のパッケージ) | `levels` | `summary_elems` | 解析 `duration_ms` | wall | 最大 RSS | finding |
| --- | --- | ---: | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 0 | 基準(`dart-grpc` と同じ 3 サービス) | 2,409 | 収束 / 4 | 195(collection 87、typed_data 51、protobuf 34) | 32 | 5,471 | 201 | 17.3 秒 | 1.15 GB | 3 |
| 1 | `googleapis`(`storage/v1`、`pubsub/v1`、`bigquery/v2`)と `googleapis_auth` | 3,025 | 収束 / 4 | 202(collection 87、typed_data 51、protobuf 34) | 46 | 12,645 | 927 | 16.7 秒 | 1.15 GB | 3 |
| 2 | `gcloud`(Storage、Pub/Sub、Datastore) | 3,322 | 収束 / 4 | 176(collection 71、typed_data 51、protobuf 29) | 49 | 13,967 | 857 | 16.9 秒 | 1.13 GB | 3 |
| 3 | `google_cloud_storage`(と `google_cloud_protobuf`、`google_cloud_rpc`) | 3,638 | 収束 / 4 | **263(http2 106、async 38、grpc 38)** | 49 | 20,274 | 1,735 | 18.1 秒 | 1.14 GB | 3 |
| 4 | JSON の Map を `fromJson` / `toJson` に渡すサービス(`googleapis` の 4 つの型、`google_cloud_protobuf`) | 4,008 | 収束 / 4 | 263(同上) | 49 | 22,077 | 2,311 | 23.1 秒 | 1.16 GB | 3 |
| 5 | `googleapis` の 10 個の API をもう 1 回ずつ呼ぶ(`drive`、`sheets`、`gmail`、`compute`、`youtube` ほか) | 4,493 | 収束 / 4 | 263(同上) | 49 | 24,444 | 1,744 | 29.8 秒 | 1.54 GB | 3 |
| 6 | (上限の確認)`googleapis` の 305 個の API のクラスを、全部 1 回ずつ作る | 4,792 | 収束 / 4 | 263(同上) | 49 | 25,318 | 1,693 | 50.7 秒 | 3.83 GB | 3 |

- 5 の行までが `bench/dart-gcp` の内容。6 は、実際の利用者のコードではなく、規模の上限を見る確認(`googleapis` のすべてのライブラリを import し、`XxxApi(client)` を 1 つずつ作る)。変種は scratchpad にだけ置いた。
- `timed_out` は、どの実行でも出なかった(時間切れなし)。**収束しなくなる依存は無かった。大きく遅くなる依存も、解析のコアの側には無かった**(`duration_ms` は 0.2 秒 → 1.7 秒、6 でも同じ)。
- 行 3 で `largest_scc` が 195 → 263 になり、中身が変わる(collection / typed_data / protobuf → http2 / async / grpc)。足した `google_cloud_storage` が `http` / `convert` / `crypto` を通る流れで、`grpc` と `http2` の SCC に合流したため。ただし、263 の中は、`google_cloud_storage` 10 関数、`_discoveryapis_commons` 6、`http` 5 で、**Google Cloud の側の関数は 1 割未満(16 関数)**。残りは `grpc` の依存の関数で、基準の時点で別の SCC にあったものが、1 つに繋がった。生成された `googleapis` のコード(1 つの API が数百〜数千の関数)は、行 6 まで足しても SCC に入らない(上位 10 件にも出ない)。
- 全部入り(行 5)の依存込みと `-no-deps` の比較:

| 項目 | 依存込み | `-no-deps` |
| --- | --- | --- |
| 行 0 の wall / 解析した関数 / finding | 19.2 秒(2 回目)/ 2,409 / 3 | 9.0 秒 / 307 / 3 |
| 行 3 の wall / 解析した関数 / finding | 20.2 秒(2 回目)/ 3,638 / 3 | 12.1 秒 / 489 / 3 |
| 行 5 の wall / 解析した関数 | (上の表)29.8 秒 / 4,493 | 16.2 秒 / 525 |
| 行 3 の不確実性の注記(`unmodeled_library_call`) | 13 | 180 |

### 10.3 時間とメモリは、どこで使われるか

extractor だけ(`rivus_dart.dart`、読むだけ)の時間と、IR の大きさ。

| 変種 | IR の関数 | extractor の wall | 最大 RSS | 解析の `duration_ms` | 全体の wall |
| --- | ---: | ---: | ---: | ---: | ---: |
| 0 基準 | 5,126 | 11.3 秒 | 1.13 GB | 201 | 17.3 秒 |
| 3 | 10,709 | 13.1 秒 | 1.15 GB | 1,735 | 18.1 秒 |
| 6 上限 | 94,437 | 39.0 秒 | 3.70 GB | 1,693 | 50.7 秒 |

- **wall の 7〜8 割は extractor**(行 6 でも)。IR は、import からたどれるファイルを全部出す(行 6 は 94,437 関数のうち、解析に残るのは 4,792 = 5%)。コアは到達できる関数だけを解析するので、`googleapis` のうち 1 つも触れない API は、解析のコストにならない。ただし extractor は読む。
- **解析の `duration_ms` は、足した依存の「関数の数」ではなく、「到達する関数の形」で決まる**。`googleapis` 255 関数と `gcloud` 107 関数の `spent_ms` は、合わせて 28 ms(行 3)。時間は、`googleapis_auth`(137 関数、3,591 ms)、`convert`(62、1,579 ms)、`google_cloud_storage`(107、1,140 ms)、`google_cloud`(11、902 ms)、`http`(82、676 ms)、`async`(138、631 ms)が使っていて、`dep_spent_ms` の約 73%。`googleapis_auth` と `convert` は、ハンドラが認証のクライアントを作る(`clientViaApplicationDefaultCredentials`)ためと、`ChunkedConversionSink` の経路のため。

### 10.4 a(届く)/ b(届かない)の検出

組み込みのモデルには、Dart の Google Cloud のクライアントの sink が無い(`rivus/dart/*` に `googleapis` / `gcloud` / `google_cloud_*` の行は無い)。そのため、組み込みのままでは、**Google Cloud の呼び出しで finding は出ない**(行 0〜6 のとおり、3 件は基準の `Files.Read`、`Jobs.Run`、`Reports.Fetch` だけ)。

sink が書かれたとして届くかを見るため、scratchpad に、12 行の sink の試作(`args` と `parts`。リポジトリに入れていない)を作って `-rules` で足した(行 3 の変種)。

| 項目 | 依存込み | `-no-deps` |
| --- | --- | --- |
| wall / 最大 RSS | 15.8 秒 / 1.13 GB | 7.1 秒 / 1.14 GB |
| finding | 13(基準の 3 件 + a の 9 件 + `Bucket.read` のインターフェースと実装で重複 1 件) | 12(重複なし) |
| a: 届く 9 経路(`gcloud` 4、`googleapis` 3、`google_cloud_storage` 2) | 9 件とも出る | 9 件とも出る |
| b: 届かない 8 経路(`*Fixed`) | 0 件(誤検出なし) | 0 件 |

- 試作の sink は、`Bucket.read` / `Storage.bucketInfo` / `PubSub.lookupTopic` / `Filter.new` の `args: [2]`(`gcloud`)、`ObjectsResource.get` / `ProjectsTopicsResource.publish` / `JobsResource.query`(`googleapis`)、`Storage.downloadObject` / `Storage.uploadObjectFromString`(`google_cloud_storage`)。リクエストのフィールドから、位置引数に届く経路は、本体を読まなくても(`-no-deps`)出る。**名前つき引数**(`Query(kind: ...)`、`QueryRequest(query: ...)` の `parts`)は、`gcloud` の `Query` の `kind:` では試していない(`Filter.new` の位置引数で代えた)。
- 届かない経路(キーの参照だけ、メッセージの本文だけ)は、sink が引数の位置を決めているので、誤検出にならない。
- `Json` サービス(`fromJson` に Map を渡し、`toJson` で戻す)は、`insert` / `patch` の本体(`bucket` や `job` の中身)が sink になっていないので、この 12 行では何も出ない。JSON の Map 経由の経路の検出は、測っていない(sink を `parts` で書いた場合の確認が要る)。

### 10.5 最大の SCC の中を 1 段掘る

rivus の出力の `largest_scc_packages` は、パッケージごとの関数の数だけ(行 3 は 10.2 のとおり)。関数の中身は出ないので、IR(`rivus_dart` の出力)から、静的な呼び出しと、候補つきの動的な呼び出しで、自分で SCC を作って調べた(コアの SCC とは一致しない: 到達の仕方と候補の決め方が違い、最大が 66 関数。向きと傾向だけを見る)。

| 呼び出しの候補の上限 | 到達した関数 | 最大の SCC |
| --- | ---: | ---: |
| 制限なし(`ChunkedConversionSink` などを全部) | 1,899 | 66 |
| 候補 16 個まで | 1,698 | 27 |
| 候補 8 個まで | 1,466 | 23 |
| 候補 3 個まで | 1,251 | 9 |
| 候補 1 個だけ | 964 | 7 |
| 静的な呼び出しだけ | 595 | 1(SCC なし) |

- 基準(`dart-grpc`)も、同じ方法で、最大 27(collection 12、typed_data 11、protobuf 4。`setRange` / `fillRange` / `length=` といった、`List` の操作)。
- 行 3 の最大(66)の中身は、`async` 27、`convert` 18、`crypto` 7、`http_parser` 6、`grpc` 5、`google_cloud_storage` 3。関数の名前は、`close` 22、`add` 18、`addError` 7、`_close` 5、`addSlice` 3 など。**`Sink` / `EventSink` / `ChunkedConversionSink` の `add` と `close` が、互いを呼ぶ**(ダイジェストの計算 → チャンクの変換 → 次のシンク、の連鎖が、動的な呼び出しで循環に見えると思われる。1 つずつの確認はしていない)。
- 動的な呼び出しの候補が多いほど、SCC が大きい(上の表)。**候補を 8 個までに切るだけで、最大の SCC は 66 → 23 になる**。
- コアの統計(`stats.dispatch_candidates`、1 つの呼び出しに候補がいくつあるか)。足すほど増えるが、桁は変わらない。

| 変種 | 候補 1 | 2〜8 | 9〜16 | 17 以上 | 候補なし | 最大 |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| 0 基準 | 1,833 | 826 | 317 | 58 | 830 | 61(`DelegatingMap.toString`) |
| 3 | 2,371 | 1,232 | 409 | 78 | 1,177 | 155(同じ呼び出し。`Object.toString`) |
| 5 | 2,628 | 1,742 | 411 | 78 | 1,316 | 155 |

- IR の中で、Google Cloud の 6 パッケージ(`googleapis`、`gcloud`、`google_cloud_storage`、`google_cloud_protobuf`、`google_cloud_rpc`、`_discoveryapis_commons`)の呼び出しの形(行 3 の IR、全 20,972 呼び出しのうち)。`googleapis` は 5,278 呼び出しのうち、静的 2,940、動的 1,973(候補 0 個 867、1 個 595、2〜8 個 876)、関数値 365 で、**候補が 9 個以上の動的な呼び出しは `googleapis` に 0**。9 個以上の呼び出しが多いのは `protobuf` と `collection`、`async`、`grpc`、`http2` で、`gcloud` に 24 件、`google_cloud_storage` に 8 件、`google_cloud_protobuf` に 4 件だけ。
- Google Cloud の側で候補が 9 個以上の呼び出しの記号は、`List.length`(37。動的 = レシーバの型が分からない)、`JsonEncodable.toJson`(2 件、最大 49 候補。`google_cloud_protobuf` のインターフェース)、`Object.hashCode`(2)、`Object.toString`(1、155 候補)。**JSON の Map を動的に渡す呼び出し(`fromJson(Map)`)は静的に解決される**(`X.fromJson(json_ as Map)` はコンストラクタの呼び出し)ので、SCC を作らない。

### 10.6 `rivus index` と `analyze` の時間の違い

行 3 の変種(`bench/dart-gcp` の部分集合)で、同じ機械で、続けて 1 回ずつ測った(この時の load average は 25 前後で、2 節の測定より遅い。比は目安)。

| コマンド | wall | 最大 RSS | 出力 |
| --- | ---: | ---: | --- |
| `rivus analyze -detail summary` | 44.2 秒 | 0.70 GB | JSON(finding と統計) |
| `rivus index`(グラフだけ) | 55.9 秒 | 1.02 GB | グラフのファイル 3.3 MB(3,790 関数、9,043 呼び出し、18,784 エッジ) |
| `rivus index -taint -o <file>` | 88.6 秒 | 1.10 GB | グラフ 3.3 MB + JSON のインデックス 479 KB(finding 3、不確実性 550) |

- 負荷が低かった時の別の測定(同じ変種): `analyze` 18.1〜20.2 秒、`index -o <file>` 23.1〜23.5 秒(`-o` は解析まで走らせる)。どちらの時も、`index` は `analyze` より 1.2〜1.3 倍遅い。
- 違い: `analyze` は、extractor の出力を読んで解析するだけ。`index` は、同じ extractor と IR の読み込みに加えて、呼び出しグラフを作って書く(`rivus graph` が使う)。`-taint` は、その上で解析も走らせ、結果のインデックスも書く。つまり、**`index` は `analyze` より安くない**。インデックスは、1 回作って、`graph` / `query` で何度も引くためのもの。
- この対象では、`index`(グラフだけ)と `index -taint` の差は解析の分(`duration_ms` で 1.7 秒ほど。上の表は負荷で広がった)。グラフ側の時間も、大半は extractor。

### 10.7 モデルで切る提案(コードは書いていない)

測定の数字から、Google Cloud の Dart のクライアントについて、最小で効果が大きい順:

| 順 | 提案 | 根拠(数字) | 効果 |
| --- | --- | --- | --- |
| 1 | **(i) source / sink のモデル**(`dart-gcp.yaml` のような組み込み。`MODELING.md` の `args` と `parts`) | 組み込みに Google Cloud の sink が 0 行。試作の 12 行で、a の 9 経路が出て、b の 8 経路は 0 件(誤検出なし)。`-no-deps` でも同じ(位置引数の経路は、本体を読まなくても出る)。コストは変わらない(行を足すだけ) | **検出が 3 → 12 件になる(検出力)。コストの面では無害** |
| 2 | **(ii) 値の流れだけのモデル**(中を読まずに、引数から戻りへ)を、少数の、候補が多い呼び出しに | `JsonEncodable.toJson`(最大 49 候補)、`Object.toString`(155 候補)、`List.length` などの動的な呼び出しが、SCC を大きくする(10.5: 候補を 8 個までに切ると 66 → 23)。ただし、いまの SCC は、Google Cloud でなく、`async` / `convert` / `crypto` の `Sink` の連鎖で、`dart:*` のモデルの領域(`ChunkedConversionSink.add` / `close`)。Google Cloud のパッケージだけのモデルは、SCC に効かない | Google Cloud のパッケージのモデルでは、効果が小さい(`googleapis` と `gcloud` の `spent_ms` は合わせて 28 ms)。効くとすれば、`dart:convert` の `ChunkedConversionSink` / `Sink` のモデル(Google Cloud に限らない) |
| 3 | **(iii) `unread_packages`(読まないパッケージ)** | ADR-092 の条件 1(コストの測定の記録: 時間か GB の 9 割が、その依存)が**満たされない**。解析の `duration_ms` は 1.7 秒で、wall(18 秒)の 1 割弱。extractor が 7〜8 割。`googleapis_auth` / `convert` / `google_cloud_storage` を切っても、解析は最大 1.7 秒しか減らず、extractor の読む量は変わらない(コアだけの変更。7 節の案 3)。`-no-deps`(全部を切った場合)の上限で、wall は 20.2 → 12.1 秒(1 回の比較)、finding は同じ | **最小の効果**。条件 1 を満たさないので、いまは勧めない。要るなら、extractor が読まない(出力しない)指定で、`googleapis` の使われない API のライブラリを読まない(行 6 の 39 秒 / 3.7 GB を減らす)ほうが効く |

- **1 位が、最小で効果が大きい**(行を足すだけで、検出が 3 → 12)。2 位、3 位は、いまの数字では、Google Cloud のパッケージには要らない。
- 切る必要が出るのは、Google Cloud のパッケージではない。`dart_services` で 12,130 関数の SCC を作るのは、1 節のとおり `analyzer` 系で、Google Cloud ではない(この測定で、Google Cloud の依存は、SCC に入る関数が 1 割未満。生成された `googleapis` の関数は 0)。`dart_services` 型の終わらない解析を再現するには、`analyzer` を含む合成の対象が要る。
- (iii) の ADR-092 の条件(測定の記録、読んだ場合との照合、`calls:`、保守的な注記、`summary` に読まなかったパッケージの一覧)は、7 節の最小の仕組みの案のまま。`googleapis_auth` を切る場合は、認証のクライアントが利用者の `http.Client` を呼び返すかどうかを調べて、必要なら `calls:` の行を書く(調べていない)。
- (i) の注意: `gcloud` の `Bucket` / `Storage` / `PubSub` は抽象クラス(インターフェース)で、実装は `_BucketImpl` など。インターフェースの記号(`package:gcloud/storage.dart#Bucket.read`)で行を書くと、呼び出しの候補になる実装の側(`_BucketImpl.read`)でも一致し、finding が 2 件になる(重複)。どちらか 1 つの書き方に決める必要がある。名前つき引数(`kind:`)を指す書き方は、`MODELING.md` で確認できなかった。

### 10.8 見つけた rivus の不具合と注意

- `rivus index -o <file>` は、`-taint` を付けなくても、解析まで走らせる(`-o implies it` と `-h` に書いてある。意図どおり)。グラフだけの時間を測るには、`-o` を付けない。
- `.dart_tool/package_config.json` が無い(`dart pub get` をしていない)プロジェクトは、終了コード 0、finding 0 で終わり、`uncertainties` に `missing_source` が出るだけ(6 節と同じ。この測定でも、最初の 1 回で踏んだ)。
- `stats.timed_out` は、時間切れでないときは出力に無い(キーが存在しない)。収束の記録では、「無い = 時間切れなし」と読んだ。

### 10.9 再現

```sh
cd bench/dart-gcp && dart pub get          # pubspec.lock があるので、同じバージョンになる
RIVUS_DART=dart RIVUS_DART_EXTRACTOR="dart run <worktree>/extractor/dart/bin/rivus_dart.dart" \
  rivus analyze -lang dart -C bench/dart-gcp -pass-stats -pass-stats-top 0 -pass-stats-sort time -detail summary -timeout 540s
```

変種(行 0〜6)は、`bench/dart-gcp` の `lib/src/services/*.dart` から、必要なサービスだけを `bin/server.dart` に入れ、`pubspec.yaml` を削って作った。試作の sink の 12 行は、リポジトリに入れていない。
