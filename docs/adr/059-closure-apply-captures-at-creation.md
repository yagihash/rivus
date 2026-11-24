# ADR-059: 呼ばれるところが見えないクロージャは、作った場所で、キャプチャした値を適用する(C-7、採用: ユーザーが承認した)

- 状態: 有効
- 関連: DESIGN-capture
- 状態の詳細: 採用
- 要旨: `assumed_call`。008 に追記あり

**背景:** クロージャがキャプチャした値が、**ヘルパーが呼ぶクロージャの中で**使われると、流れが何も報告せずに失われた(Go の `run(func(){ exec.Command(a).Run() })`、Dart の `Button(onPressed: () => sink(t))`、`Timer.run`。CORE-F、DART-16、`CAP`)。束縛(キャプチャした値)を呼び出しに付けるのは、クロージャを作った関数の中の呼び出しだけで、ヘルパーの中の `f()` には付かない。ADR-053 は、この落ちたことを `capture_unbound` で示した。直すのは C-7(`docs/DESIGN-capture.md`)。

**決めたこと:**
1. `make_closure`(キャプチャする値がある)の命令で、クロージャの要約を、キャプチャの値で適用する(`captured`、`applyMade`)。Params は `KUndef`。**適用するもの**: sink への到達、キャプチャした変数への書き込み、グローバルへの書き込み。**適用しないもの**: throw と結果(呼ぶところのもの)。これらがキャプチャした値に依るときは、`assumed_call` の detail に書く。
2. **飛ばす条件は、使われ方**(`sg.Instr.Escapes`): クロージャの値の使われ方が、作った関数の中での直接の呼び出し(callee の位置。`go`、`defer` を含む)だけなら適用しない(その呼び出しが、値を持っている)。引数、`store`、`return`、別のクロージャのキャプチャなどが 1 つでもあれば適用する。コピーと φ はたどる。使用が 0 件のクロージャは、適用しない(呼ばれない。`capNever` は検出されないまま)。候補の解決のされ方には依らない。
3. **uncertainty**: 新しい kind `assumed_call`(`attributable_to: mixed`、ADR-008 の追記)。クロージャの要約が、キャプチャした値に依る効果を持つときだけ。`symbol` はクロージャの関数 ID、場所は `make_closure`。finding の経路に載り、`confidence` は `no_uncertainty_on_path` にならない(全 finding に効く検査を、curated に足した)。
4. エンジン: `make_closure` → クロージャのエッジを、`order` と `levelsOf` に足した(クロージャの要約が変わると、作った関数が再解析される)。
5. **書き戻しだけの書き込みは適用しない(案 D)**: クロージャが、グローバル G に、作った場所で G から全体を読んだ値(`before := G` の、コピーと φ だけを通ったもの。参照でキャプチャしたセルのときは、そのセルが、その読み出しの 1 回だけ書かれ、ほかの使われ方がない)を、そのまま G の全体へ書き戻す(モックの後始末)ときは、その要素を適用しない。**根拠**: 流れに依らない解析では、G は読まれた値を持つので、`G = G` の書き込みは G に何も足さない。足すのは、構造体のコピーの誤差だけ(`expandParam` が構造体全体の要素を読むとき、フィールドの指す先の要素を根へ持ち上げる。バックログ B)。`before + src`、φ で別の値と合流、フィールドを書き換えてから戻す、別の path への書き込み、別のグローバルへの書き込みは、判定が「そのまま」と言えないので、適用する。判定がたどれなければ、適用する。数は `stats.identity_global_stores`(飛ばしたクロージャの数)。固定: `go-curated/api/restore.go`。
6. Rust は、キャプチャのあるクロージャが `make_closure` でなく環境の `alloc` なので、次の段階(`docs/DESIGN-capture.md` の「Rust の節」)。

**限界:** 1 度も呼ばれないクロージャの流れも、渡されていれば finding になる(過検出)。呼ぶ順序は見ない。キャプチャと引数の両方に依る流れは、案 2(ADR-047)まで見つからない。結果の流れ(`WithTx2` など)は、`assumed_call` を残して、次の段階(結果をクロージャの値に持たせる)。依存の関数の中で記録された `assumed_call` は、ほかの不確実性と同じく、全体の一覧からは捨てられる(finding の経路には残る)。

**検証(`bench/compare.sh 6c5c28c <C-7>`、実コード、案 D のあと。削除 0、全部収束):**

| bench | findings | 追加 | `assumed_call` | 備考 |
| --- | --- | --- | --- | --- |
| go-curated | 50 → 66 | +17 | 0 → 44 | `@miss` だった本物の流れと、新しい固定 4 |
| dart-curated | 92 → 109 | +17 | 0 → 36 | `@miss` だった本物の流れ。tear-off の形(R-14)は含まない |
| rust-curated、go-synth-S、gin-vue-admin、go-test-bench、parseable | 同じ | 0 | 0 → 3 / 18 / 23(rust、parseable は 0) | |
| gogs 2 つ | 77 → 79 | +2 | 0 → 152 / 153 | +2 は本物: `ChangeUsername$1` の `os.Rename`(`org/setting.go:29`、`user/setting.go:73`)。案 D の前は +47(45 件は `SetMockPicture$1` の `Picture = before`、構造体のコピーの持ち上げ)。飛ばした 8(`identity_global_stores`) |
| dart-pub | 17 → 19 | +2 | 0 → 340 | `extractTarGz` → `createFileFromStream$1` の `File.new`(本物) |
| Gitea v1.26.4 | 242 → 269 | +29 | 0 → 674 | 収束(6 → 7 パス)、134 → 142 秒、RSS 11.5 → 11.8 GB。追加 29 のうち 25 は、サービスの `db.WithTx(…, func…)` の形のクロージャ(`processGiteaTemplateFile$1`、`UploadAvatar$1`、`DeleteAvatar$1`、`AcceptTransferOwnership$1`、`MkdirTempRandom$1`)を通る。ほかの 4 は、`*net/http.Request` などの param の source の組み替え(`other_sources` の割り振りが変わった)。削除の 1(`pagination.go:53`)は、同じ型の source の組み替え(`*net/http.Request` の source の数は 21 のまま) |

rivus 自身(dogfood): 39 → 44(追加の 5 は `rivus index` の `-o` の経路が `graphdb.Write` のファイル作成に届く、本物)。サイレントな見逃し(`@miss(silent) / @miss`): Go 5/15 → 2/3、Dart 31/47 → 10/30、Rust 15/16 のまま。基準の gogs の 77 件に、`conf.Picture` の持ち上げで出ているものは 0 件。

