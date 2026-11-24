# parity: 言語間の差の表(B-52、段階 1)

組み込みのモデルの整備の状況を、「割合 1 つ」でなく、**量・需要・正しさ・当たっているか**の 4 つの数に分け、Go・Dart・Rust の列に実数と割合で並べる。結果は [`TABLE.md`](TABLE.md)(コミットする表)、一致しなかった行の一覧は [`DEAD.md`](DEAD.md)。**コア(`internal/`)は変えない**: モデルは `internal/model/builtin` を読み、IR は小さな自前のデコーダで読む。

## 4 つの数

| | 何を数えるか | 分母 |
| --- | --- | --- |
| (A) 量 | 標準ライブラリの公開 API のうち、組み込みの行がある関数の数 | `go/types` で列挙した、データを運びうる関数(**手で作らない**)。Go だけ。Dart と Rust は形が決まってから |
| (B) 需要 | 実コードの、本体のないライブラリ関数への呼び出し(箇所)のうち、組み込みの行がある箇所の数 | IR の `call` 命令。形((i)〜(iii))ごと。汚染が届いた呼び出しは `-pass-stats` の `unmodeled_top` から(上位 20 だけ。下の「限界」) |
| (C) 正しさ | `pure` の行の実行の検査(`TestZooPureGo`)と、zoo の表(セル / ケース / 出る / `@miss`) | `pure` の行の数。Dart と Rust は「未」 |
| (D) 当たっているか | どの IR にも一致しなかった行の数(**死んだ行**) | 鍵が呼び出しの行 |

### 形((i)〜(iv))

既定の扱い(`internal/taint/calls.go` の `unmodeled`)は、**レシーバと引数のすべての中身が、結果のすべてに流れる**(bool は除く)。レシーバと引数への書き込みは含まない。コールバックは呼ばれない(CB-LIST)。だから「モデルがない」は見逃しではない。効くのは、既定が間違う形:

- (i) 値を返すだけ: 既定で運ばれる。モデルは不確実性を減らすだけ。
- (ii) 引数やレシーバに書く(`json.Unmarshal(data, &v)`、`buf.WriteString(s)`、Rust の `&mut` の出力引数、Dart のコレクションへの `add`): 既定は流れを除去する。
- (iii) コールバックを呼ぶ(`sort.Slice`、`then`、`map`、`forEach`): 既定は運べない。モデルの書式もまだ表せない(CB-LIST)。
- (iv) source / sink / sanitizer / boundary、そして `pure`(流れを切る): モデルが意味そのもの。

組み込みの行の形は、行の `flows` から決める(行き先が `result` でない flow があれば (ii)、`pure: true` なら pure)。(iii) は、モデルの書式が表せないので、**名前による推定**(`bench/parity/rows.go` の `callbackNames`)。モデルのない関数の形は、Go の標準ライブラリは**型から**(`gotypes.go` の `FuncShape`)、それ以外は名前から(コールバック・書き込みの類)、どちらでもなければ「分からない」。(ii) の型からの判定は**上界**: ポインタのレシーバのメソッドは、`Len` のように読むだけのものを含む。

### (D) の分け方

IR の呼び出し先の名前(`callee.symbol`、関数の名前)と、モデルの鍵(`symbol`)の**文字列の一致**。一致しなかった行は 4 つに分ける。

- **API 不在**(Go のみ): `go/types` にその名前がない(綴りの誤りか、消えた API)。
- **綴りの疑い**: 同じ API の別の綴りが IR にある(末尾が同じ: Go は `*` を除いた全体、Dart は `#` のあと、Rust は最後の 2 要素)。再エクスポート(`tonic::request::Request` と `tonic::Request`)、レシーバの `*` などで起きる。
- **別の型の同名メンバ**(Dart と Rust のみ): 同じライブラリ(crate)の別の型に、同じメンバ名の呼び出しが IR にある(`dart:core#List.join` に対する `dart:core#Iterable.join` のように、宣言した型が違う: 継承・Deref・trait の疑い)。`new`・`get` などの一般的な名前は除く。
- **使われていない**: 似た名前も IR にない。**IR に出ないだけの行と、綴りの誤りで一致しない行は、区別できない**ので、入力(実コード)を増やすと、この数は減る。

`match: implementations` の展開と、コアが作る関数値の候補(`ValueCands`)は見ない(当たりを数え落とす側の誤り)。

## 機能の差の表(モデルまわり。言語ごとの有無)

上の 4 つの数は「モデルの量と質」の差で、こちらは**道具と機構の差**。✓ は対応済み、✗ は未対応(課題か PR の番号があれば書く)、不要は「言語の性質で要らない」(理由 1 語)。「要確認」は、まだ事実を確かめていない。

| 機能 | Go | Dart | Rust |
| --- | --- | --- | --- |
| パッケージ単位の宣言(`body_replaced_packages`。ADR-100) | ✓ | ✓ | ✓ |
| extractor が宣言のある本体を読まない(段階 4。ADR-105) | ✓ | ✓ | ✓(`cargo check` はそのクレートも読むので、減るのは IR の大きさだけ。設計 7a 節) |
| 差分テスト(`bench/model-diff`) | ✓ | ✓ | 一部(綴りの確認のみ。ローカル。`-lang rust`。解析 2 回の比較は ✗) |
| 行がない関数の一覧(`-coverage-gaps`) | 一部(標準ライブラリだけ) | ✗ | ✗ |
| 行がない関数の一覧(`rivus models coverage`) | ✓ | ✓ | ✓ |
| `kind: field` の source | ✓(`pkg.Type.Field`) | ✓(dart-github) | ✓(`::` 区切りの型名。#161、#230) |
| `callback_param` の source(ADR-106) | ✓(単体テスト) | ✓(shelf_router) | 未確認(書式の検査だけ。extractor では未確認) |
| `summary.numeric_stops.char_reads`(文字の取り出しで止まった数。#185) | ✓(byte・rune の型、定数でない添字の文字列の添字) | ✓(`codeUnitAt`、`codeUnits`・`runes` の繰り返しと添字。**コールバックに渡る要素(`runes.map((r) => ...)`)は数えない**) | 一部(`bytes()`・`chars()`・`char_indices()` の繰り返し、`as_bytes()[i]`、`char` 型。**それ以外の Rust の byte の取り出し(取り出した `Vec<u8>` の添字、関数に渡った `u8` など)は数えない(#185)**) |
| `named:` の sink の選択 | 不要(名前つき引数がない) | ✓ | 不要(名前つき引数がない) |
| `verified_versions`(読み込みと `rivus models versions`) | ✓ | ✓ | ✓ |
| `summary.unverified_versions`(確認していないバージョンの uncertainty) | ✓ | ✓(`pubspec.lock`。#162) | ✓(`Cargo.lock`。この PR) |
| `analyze` のカテゴリの絞り込み(`-categories`、`-skip-categories`、`summary.categories_excluded`) | ✓ | ✓ | ✓ |
| path の major を `/v*` と書く(ADR-110) | ✓ | 不要(path に major がない) | 不要(path に major がない) |
| zoo で骨組みを依存として読む | ✓(骨組みが独立の module) | ✓(依存として。#231) | ✓(`exclude = ["skel"]`) |
| `checked_by: cell` の行(ADR-107) | 要確認(機構は共通。Go で使う行があるか未確認) | ✓ | 要確認(同左) |
| 変換できない関数を飛ばして読み続ける(`fn_skipped`。ADR-109) | 要確認(パニックの報告なし。要るかを見る) | 要確認(同左) | ✓ |
| zoo の外部パッケージのセルの置き場 | `bench/zoo-go-pkg/<名前>/` | `bench/zoo-dart/cells/packages/<名前>/` | `bench/zoo-rust-pkg/<名前>/` |
| `adopt_rules:` の書式とローダー(一括採用の段階 1。ADR-113)。言語を問わない書式 | ✓ | ✓ | ✓ |
| ポインタ越しの書き込みの別名による近似(`ptrAlias`)と、それを通った finding の `pointer_alias` の uncertainty | ✗(まだ有効にしない) | ✗(同左) | ✓(`HasEnv` の言語。uncertainty はこの PR) |
| `make model` のステップ 1(枠、候補、検証、差分テスト。設計 6.11) | PR #233(レビュー中) | ✓(`LANG=dart`。dio で実演。骨組みはステップ 2 で) | ✗ |
| クラウドで作れる | ✓ | ✓ | ✗(extractor がサンドボックスを要る) |
| ベンチの合格基準(`make bench-passline`、1 行の PASS/FAIL。`bench/deps_bench.py`) | ✓(memos。時間、メモリ、収束、CVE-2025-65799 の検出) | 未定(dart_services と cocoon の合格基準は、あとで決める。今は INFO の行だけ) | ✗(まだない) |
| アーカイブのエントリ名の source(信頼できないアーカイブの名前。zip slip の入口) | ✓(`archive/tar`、`archive/zip`。`go-archive.yaml`) | ✓(`package:tar`、`package:archive`。`dart-tar.yaml`、`dart-archive.yaml`) | ✓(`tar`、`zip` クレート。`rust-archive.yaml`。`zip` の `enclosed_name` は source にしない) |

**規則**: 新しい機能の PR は、この表の行を足すか直す。Go だけで入れるときは、ほかの言語の課題の番号を書く。

## 表の更新の手順

入力は、(1) 各言語の IR(`bench/parity/collect.sh`)、(2) 実コードの解析の要約(`rivus analyze -pass-stats`、`ANALYZE=1`)、(3) zoo のテストの出力(任意)。

```sh
export WORK=/path/to/work           # 作業ディレクトリ(IR と中間ファイル)
export SRC=/path/to/checkouts       # gogs, gin-vue-admin, gophish, casdoor, pub のチェックアウト(下の表の SHA)
export RIVUS_DART="mise exec dart@3.13.6 -- dart"       # extractor/dart で dart pub get 済み
export RIVUS_RUST=/path/to/rivus-rust                    # extractor/rust のバイナリ(自前のコード rust-curated だけに使う)
export PARSEABLE_IR=/path/to/parseable.jsonl             # すでにある parseable の IR(新しく抽出しない)。なければ、サンドボックスの下で抽出する(macOS のサンドボックスが要る。ないときは失敗。ホストでは読まない)

# 1. 重い解析は 1 度に 1 つ(collect.sh は空きメモリ 40% 以上まで待つ)
ANALYZE=1 bench/parity/collect.sh go-curated zoo-go gogs gva gophish casdoor \
          dart-curated zoo-dart dart-pub rust-curated zoo-rust parseable

# 2. zoo のテストの記録(任意。表の (C))
go test ./cmd/rivus -run 'TestZooPureGo|TestZooGo$|TestZooDart$|TestZooRust$' -v -count=1 2>&1 \
  | grep -E '^[[:space:]]+zoo[a-z_]*test.go:[0-9]+: |^(--- |ok|FAIL)' \
  | sed -E 's/^[[:space:]]+zoo[a-z_]*test.go:[0-9]+: //' > $WORK/zoo-lines.txt   # RIVUS_DART を付ける

# 3. 表を作る(go/types で標準ライブラリを読むので 10 秒ほど)
go run ./bench/parity report -work $WORK/out -table bench/parity/TABLE.md -dead bench/parity/DEAD.md -zoo $WORK/zoo-lines.txt
```

`TABLE.md` と `DEAD.md` は**手で直さない**。組み込みのモデル(`internal/model/builtin/*.yaml`)、extractor、読む入力を変えたら作り直し、コミットメッセージに、前の表からの差(一致しない行の数、(B) の割合)を書く。`-no-gotypes` は (A) と Go の形の型からの判定を省く(速い確認用)。`go test ./bench/parity`(`-short` で標準ライブラリの読み込みを省く)が、形・名前・IR の解釈・一致の判定の単体テスト。

## 入力(`TABLE.md` の 6 に書く)

| 入力 | 何 | バージョン |
| --- | --- | --- |
| gogs | Go、実コード | `8a3b8198a`(`bench/README.md`) |
| gin-vue-admin | Go、実コード(`server/`) | `7bc7c3051067` |
| gophish | Go、実コード | `27d13a0584731371033926064d750e5f4f620e7a`(`bench/wild/go`) |
| casdoor | Go、実コード | `7b0b426a76fd77b89817e0eafcccaed8d15b8cf4`(`bench/wild/go`) |
| go-curated、zoo-go | Go、fixture | このリポジトリ |
| dart-pub | Dart、実コード(`--no-deps`、`test/` は既定で読まない) | `5e24336e71a05a96b558d496953ebed223f6e40a` |
| dart-curated、zoo-dart | Dart、fixture | このリポジトリ |
| parseable | Rust、実コード(`--no-deps`) | `5c20b8e4`。**すでにある IR**(新しく抽出しない。C-19) |
| rust-curated、zoo-rust | Rust、fixture(自前のコード。外の依存なし) | このリポジトリ |

## 組み込みの行のうち、zoo が踏まない行の許可(`.dead-allow`)

`TestBuiltinRowsAreHitByTheZoo`(`cmd/rivus`)は、組み込みのモデルの行(関数の名前を鍵にするもの)が、zoo かキュレートされた対象の IR で踏まれることを確かめる。踏まれない行は、**モデルのファイルの隣の `internal/model/builtin/<ファイル>.dead-allow`** に、`symbol | reason` の 1 行で書く(言語はファイル名の頭: `go-`、`dart-`、`rust-`)。ファイルをモデルごとに分けてあるのは、モデルを足す PR が、共有の一覧を触らずに済むため。この許可の一覧は、`DEAD.md`(このディレクトリの `report` が、実コードの IR から作る生成物)とは別のもの。

理由は 3 つ: `dependency`(標準でないライブラリで、zoo にその依存がない。行の確認は、実コードの IR(この `report`)か、そのモデルの専用のテストが持つ)、`debt`(標準ライブラリで、まだ呼ぶセルがない。**減る一方**で、新しい行には足さない)、`spelling`(extractor が書く名前のある別の綴り)。踏まれるようになった行の行、モデルにない行の行、別のモデルのファイルの行は、テストが失敗にする。実行で確かめられない API(サーバ、デバイス、ネットワークが要る)だけが、恒久に残ってよい(`docs/MODELING.md`)。

**依存の深さの境界のセル(アクセスパス。ADR-119)**: Go(`cmd/rivus/depth_boundary_test.go`)と Dart(`depth_boundary_dart_test.go`)にある。依存の 3 階層目の値と、同じ 2 階層目の隣、同じ 1 階層目の隣を、深さごとに表にしたもの。**Rust は、同じ形を次の PR で足す**(まだない)。**既知の違い: Dart は、同じ深さで、Go より 1 階層粗い(参照の `*` が 1 階層に数えられる。課題 #190)。Dart の隣のフィールドの区別は、深さによらず失われる(#191)。**

## 限界(段階 1)

- **「汚染された値が届いた呼び出しのうち、モデルが一致した割合」は出せない**。`-pass-stats` の `unmodeled_top` は、モデルのない関数の上位 20 だけで、モデルがある呼び出しの `reached` と、`reached` の総数は出ない。コアの診断に、「モデルがある呼び出しに汚染が届いた数」と「モデルのない呼び出しに届いた数の総和(20 件で切らない)」を足す必要がある(コアの変更。助言者への依頼)。表の (B) の割合は、IR の呼び出し箇所を数えたもので、**届いたかを見ていない**。
- Dart は、main の extractor が `test/` などを既定で読まない(ADR-062)。dart-pub は既定で読み、参考に `--include-tests` の IR(`dart-pub-tests`、合計に入れない)を併記する。
- Rust は、自前のコードと、すでに IR のある parseable だけ(C-19)。**実コードが 1 つ**なので、Rust の (B) と (D) の「使われていない」は、Go より誤差が大きい。
- (A) は Go だけ。Dart(`dart:*`)と Rust(`std`)は、同じ基準(レシーバか引数にデータを運びうる型がある公開 API)で列挙する方法を決めてから。
- Go の「本体のない呼び出し」は、標準ライブラリ(とモデルのある関数)に限る。標準ライブラリ以外の依存は、解析が本体を読むので数えない。
- 型からの形は、`reflect.Value` のように型に出ない書き込み、ジェネリック型のメソッド(列挙から外す)を取りこぼす。
