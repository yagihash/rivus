### 6.8 zoo のセルの依存(骨組みか、本物のパッケージか。調査と案)

背景: モデルのセルを作る担当が、骨組み(`bench/zoo-dart/cells/packages/<名前>`)を自分で書くと、手間がかかるだけでなく、**モデルを書いた本人が骨組みも書くので、本物とのずれに気づけない**(確認の意味が弱い)。骨組みが本物と名前でぶつかる、本物と違う引数の渡し方をしてテストが失敗する、という問題が、検証用の 1 本目(Dart の `shelf_router`)で起きた。

**今、骨組みにしている理由(事実。`bench/zoo/README.md`、`docs/DESIGN-zoo.md`、コミット `f70d2b1`、`1f33c8e`)**:
1. **exec が骨組みの記録に依存する。** `bench/zoo-dart/exec` は、セルを実行して、`zoo_mark` の記録(`dependency_overrides` で、宣言だけの `zoo_mark` を、記録する `zoo_mark` に置き換える)で、sink に値が届いたか、届かなかったかを確かめる(実行で裏付ける)。`package:http` の骨組みの記録は、sink でない `recordAny`。本物のパッケージの関数(`Client.get`)は、記録しないので、実行の裏付けができない。
2. **本物のコードが動く。** 本物の `http` のセルを `dart test` で実行すると、本物の `Client.get` が動き、ネットワークに出る(`example.invalid` への接続)。骨組みなら、何も起きない。
3. 「Flutter の本体は入れない(SDK が重く、`pub get` が不安定)」(DESIGN-zoo.md)、Rust の zoo は「依存が一切ない」(`cargo check` が `build.rs` と proc macro を動かすので、zoo は自前のコードだけ)。
4. `pubspec.lock` は、セルを足す PR 同士の conflict を避けるためにコミットしない(`1f33c8e`。CI と `make setup` が `dart pub get`)。本物の依存は、ロックがないと、推移的な依存のバージョンが変わりうる。

**実験(Dart。この機械で。`dart pub get --offline`、実行はしていない)**: `bench/zoo-dart/cells` のコピーで、`packages/http`(骨組み)を、**バージョンを 1 つに固定した本物**(`http: 1.2.2`。キャッシュにあった)に替えて、`rivus analyze -lang dart` を比べた。
| | 骨組み | 本物(`http: 1.2.2`) |
| --- | --- | --- |
| finding | 197 | 197(`http_client_get` のセルは、どちらも 2 件。同じ) |
| コアが読む関数 | 915 | 1,565(依存の 657 関数が増える) |
| load(extractor + IR) | 6.1 秒 | 7.2 秒 |
| 注記 | `unmodeled_library_call` 80 | `unmodeled_library_call` 76、`dependency_body_unread` 16(`Client.send` のモデルがあるので、依存の実装を読まない)、`summary.dependency_sites`(`searched: false`、依存の関数 657) |
- **本物にしたときに変わること**: 依存として読まれる(`Dep`)。`-allow-deps-under` は要らない(pub キャッシュは、常に信頼される。今回のフラグなしの実行で確認)。`summary.dependency_sites` の注記が付く。zoo のテストが、注記の数や種類を比べているなら、期待値が変わる(要確認。`cmd/rivus/zoo_dart_test.go`)。宣言(`body_replaced_packages`)を付けたパッケージは、本体を読まないので、むしろ自然(読み込みが減る)。**行のシンボルが、本物のファイルの構成(`package:http/src/client.dart#Client.send`)と一致するかを、本物で確かめられる**(骨組みは、ファイルのパスまで真似る必要があった)。
- **本物の依存の置き方**: セルのパッケージ(`cells/pubspec.yaml`)に足すと、ロックをコミットしないので、推移的な依存が変わりうる。**本物のパッケージごとに、別のディレクトリ(`bench/zoo-dart/real/<パッケージ>/`)に、自分の `pubspec.yaml` と、コミットした `pubspec.lock`** を置けば、PR 同士で conflict しない(別のファイル)。`bench/dart-gcp` が、同じ形(ロックをコミット)で、公開のベンチになっている。

**exec の安全(事実と判断)**: 本物のパッケージの exec は、取得した第三者のコードの実行になる(`dart test`。依存の本物のコードが呼ばれる)。規則「取得したコードは実行しない」と、衝突する。ただし、CI には前例がある: `gcp-exec`(`bench/gcp`)は、Google Cloud の本物のクライアントを、フェイクに向けて実行している(loopback 以外は拒否するテスト)。**クラウドのセッションでは、この規則のため、本物の exec は走らせられない(CI に任せる)。** 判断: 相談役。

**Go と Rust の同じ点**:
- Go(`bench/zoo-go`): 依存は、自前のモジュール `zoodep`(`go.mod` の `replace zoodep => ./dep`)だけ。「zoo の依存の代わり」で、第三者のパッケージと同じく本体が読まれる(`Dep`)が、第三者のコードではない。本物のモジュールにするなら、`go.mod` の `require` を固定して、`go.sum` をコミットする(Go はロックが `go.sum` で、conflict は、足す行が別なので、起きにくい)。
- Rust(`bench/zoo-rust`): **依存が一切ない**(std だけ。`build.rs` なし、proc macro なし)。Rust の extractor は `cargo check` で `build.rs` と proc macro を動かすので、本物のクレートに依存させると、第三者のコードの実行(隔離の下でだけ許される。6.5)になる。**Rust は、骨組みのままにする**(生成する。下の案 (b))。

**差分テスト(段階 1)との分担**(手順書に書ける形): **zoo のセルは、「sink / source / sanitizer の位置の確認」に絞る**(行が、本物のシンボルに一致して、肯定のセルで届き、否定のセルで届かない)。**流れの確認(引数 → 戻り値、書き込み先、レシーバ)は、差分テストに任せる**(本物の本体と照合する)。zoo のセルは、モデルの流れの正しさを、骨組みで確かめない。骨組みが本物とずれても、位置の確認だけなら、害が小さい。

**案**:
- (a) 本物のパッケージに依存させる(Dart だけ先に)。条件: CI のオフライン(`pub get` は CI で既にオンライン)、ロック(パッケージごとのディレクトリ)、exec の安全。**exec の記録ができない**(骨組みの記録がない)ので、そのセルは、解析だけの確認になり、「実行で裏付ける」を失う。
- (b) 骨組みのまま、ただし、**本物の公開シグネチャから機械的に生成する**(生成器は段階 3 と共有。モデルを書いた本人が手で書かない)。exec の記録(`recordAny`)は、生成した骨組みの中に、機械的に入れる。
- (c) 現状のまま。

**推奨: (b) に、本物での解析の照合を足す。** セルは、生成した骨組みで、解析と exec を行う(実行の裏付けを残す。本物のコードは動かさない)。**同じセルを、本物のパッケージ(バージョン固定。パッケージごとのディレクトリ、ロックをコミット)に対しても、解析だけして、骨組みの解析と、finding の集合が同じことを確認する**(上の実験で、`http` は 197 件が同じだった)。これで、(i) 骨組みの手書きのずれ(名前、ファイルのパス、引数の渡し方)が、照合で見つかる(骨組みが本物とずれると、finding が違う)、(ii) 本物のコードは実行しない(規則と整合)、(iii) 生成器が入る前も、照合は今すぐ入れられる(生成器は後)。理由: (a) は、exec の記録と、本物の実行の安全を失う。(c) は、本物とのずれに気づけない問題が残る。

**外部のパッケージを使うセルの置き方(3 言語で 1 つの方針。事実は、一時の場所で骨組みだけを置いて確かめた。取得はしていない)**:

方針: **セルは、解析では骨組み(本物の公開のシグネチャから作る)に向け、実行では、第三者のコードを新たに動かさない。パッケージごとに、独立したモジュール(Go)、独立したクレート(Rust)に置く**。共有のファイルは、足さない。

| | Go | Rust | Dart |
| --- | --- | --- | --- |
| 置き場所 | `bench/zoo-go-pkg/<パッケージ>/`。自分の `go.mod`(`module zoopkg/<パッケージ>`、`replace zoo => ../../zoo-go`(`zoo/mark` を使う)、`replace <本物のモジュールのパス> => ./skel`)、`skel/`(骨組み。自分の `go.mod` の名前は本物のモジュールのパス)、`cells/`、`rules.yaml` | `bench/zoo-rust-pkg/<パッケージ>/`。自分の `Cargo.toml`(空の `[workspace]` で、zoo の workspace に入れない)、`skel/`(骨組みのクレート。名前は本物)、`zoo-mark`、`src/`(セル)、コミットした `Cargo.lock`、`rules.yaml` | `bench/zoo-dart/cells/packages/<パッケージ>`(path の依存)。`cells/pubspec.yaml` と `exec/pubspec.yaml` は、そのディレクトリから生成する(コミットしない)。`test` の依存と名前がぶつかる骨組みは、自分の `pubspec.yaml` に `zoo_exec_override: <範囲>` を書く(exec の `dependency_overrides` に出る) |
| 解析で何を読むか | 骨組み(`replace` で `./skel`)。別のモジュールなので、`Func.Dep` になる(試した: finding は 1 件で、`dependency_functions` は 6) | 骨組みのクレート(path の依存)。`cargo check` は、自前のコードだけ(`build.rs`、proc macro なし)。サンドボックスの下で読めた(finding 1 件) | 骨組み(cells の path の依存)。本物の依存は、cells に入らない |
| 実行(exec) | パッケージごとの `cmd/observe`(`-tags zooexec`)で、記録する `zoo/mark` に向ける。骨組みは、記録のために `zoo/mark` を呼べる(`zoo` を `replace` で持つ) | パッケージごとの exec(記録する `zoo-mark`)。骨組みは自前のコード | exec は、`dependency_overrides` で、`test` がすでに動かしている本物(`shelf` など、`test` の依存と名前がぶつかるもの)に解決する |
| 共有のファイル | なし(`zoo-go` の `go.mod` は読むだけ。`cmd/observe` はテストが overlay で作る) | なし(`zoo-rust` の `Cargo.toml` と `exec/src/cases.rs` に足さない。実行の crate はテストが書く) | なし(パッケージのディレクトリだけ。2 つの `pubspec.yaml` は生成) |

事実(試して分かったこと):
- Go: 独立したモジュール(`zoo-go` の外)は、`zoo-go` の `./...` に入らない。`go build`、`-tags zooexec`、`rivus analyze` が通り、`github.com/gin-gonic/gin` の骨組みの `Context.Query` から `mark.SinkCmd` への finding が出た。
- Rust: **サンドボックスは、対象のディレクトリの外にある path の依存を読めない**(`../../zoo-rust/zoo-mark` は、取得の段で読めずに失敗した)。`zoo-mark` は、テストが対象のディレクトリに写す(今の `zooRustCopy` と同じ)。写せば、サンドボックスの下で動き、finding が出た。取得したクレートは動かしていない(取得そのものがない)。
- **実装済み(`cmd/rivus/zoo_pkg_test.go`)**: テストは、`bench/zoo-go-pkg/*` と `bench/zoo-rust-pkg/*` を、ディレクトリから探す(一覧はない)。`TestZooPkgGo` と `TestZooPkgRust` が、パッケージごとに、3 回の実行(全部のケースが走り、全部の文が走る)、真の注釈の照合、rivus の照合を行う。Go の実行用の `cmd/observe` は、overlay で作る(リポジトリに生成しない)。Rust の実行用の crate は、一時のディレクトリに書く(共有の target で、zoo-mark は 1 度だけビルド)。マークの固定(`zooMarksRows`)と、一致した行の数え上げ(`goZooSymbols`、`rustZooSymbols`)も、同じディレクトリを探す。**zoo 全体の検査(`TestFuncValues*`、パリティ、純粋の行、コールバックの行など)は、`bench/zoo-go`、`bench/zoo-rust` だけを読む**ので、骨組みの `Dep` は入らない。依存の関数を数えるのは、一致した行の数え上げだけ(骨組みを呼ぶことが、その行に当たることなので)。合成の `zoo_synth`(Go と Rust)で確かめた。
- CI: paths-filter の dart と rust に `!bench/zoo-go-pkg/**`(rust と dart)と `!bench/zoo-rust-pkg/**`(dart)を足した。Go のパッケージは `test` のジョブが、Rust のパッケージは `rust` のジョブが、テストの中で拾う(ステップを足さない)。Rust のパッケージは、1 つのジョブの中で順に、共有の target で動く(zoo-mark と標準ライブラリのビルドは 1 度。パッケージごとに足されるのは、骨組みとセルのビルドと、抽出の数秒)。
- **zoo-mark の渡し方(Rust のサンドボックス)**: (a) テストが、パッケージを一時のディレクトリに写し、zoo-mark も写して、`Cargo.toml` の path を書き換える(採用。共有のファイルが増えない。コミットしたパッケージは、リポジトリの中でも `cargo check` が通る)。(b) サンドボックスの読み取りの許可に、zoo-mark の場所を足す(エンジンの隔離の変更になり、対象の外を読ませる許可が増える。採らない)。(c) パッケージごとに zoo-mark をコミットする(写しが増える。採らない)。

| 条件 | Go | Rust | Dart |
| --- | --- | --- | --- |
| (i) 共有のファイルをさわらない | 満たす | 満たす | 満たす(2 つの `pubspec.yaml` は生成物) |
| (ii) zoo 全体を検査するテストに、依存のコードが入らない | 満たす(別のモジュールで、固定のディレクトリに入らない。骨組みの検査は、パッケージごとの処理で行う) | 満たす(同じ) | 満たす(cells に本物の依存がない) |
| (iii) 第三者のコードを動かさない | 満たす(取得なし) | 満たす(取得なし。`build.rs` なし) | 満たす(本物は `test` がすでに動かすものだけ) |
| (iv) 骨組みは機械的に生成する | Go は済み(6.11 の段階 2。`make model ... ARGS="-gen-skeleton"`) | 未(同じ) | 未(同じ) |

**開発用の依存とぶつかる骨組み(Dart。`shelf_router` で起きた。解決済み)**: 骨組みの `shelf_router` が本物の `shelf` に依存すると、`shelf` とその依存(`async`、`collection`、`http_parser` など)が zoo のプログラムに入り、zoo 全体を検査するテスト(`TestFuncValuesDart` など)が、依存の関数に引っかかる。骨組みの `shelf` を置くと、`exec` の `test` が持ってくる本物の `shelf` と名前でぶつかる。解決は、**解析と実行で、依存の解決を分ける**こと。cells(`bench/zoo-dart/cells`)は、`shelf` を骨組み(`packages/shelf`、path の依存)に解決し、rivus は骨組みだけを読む。exec は、`dependency_overrides` で `shelf` を本物(`test` がすでに動かしているもの)に解決し、セルのコードは本物で動く。骨組みは本物の公開のシグネチャから作るので、セルは両方で型が合う。取得したコードを新たに動かさない(上の「exec の安全」の線の内側)。検査のテストは、依存の関数を対象から外さない(zoo に本物の依存を置けないことを守る検査でもあるため)。`test` の依存の名前が、モデルの候補(`shelf`、`shelf_web_socket`、`web_socket_channel`、`async` など)と重なるが、この形で骨組みを置ける。

実装の順(コードは書いていない): 1. 照合の仕組み(`bench/zoo-dart/real/<パッケージ>/` と、比べるテスト。CI で `dart pub get`)。2. 生成器(段階 3)ができたら、骨組みを生成に替える。手順書の 2 節(作る)には、相談役の判断のあとに反映する。

**生成器の要件(骨組み)**: 本物の型の埋め込みと、インターフェースの実装の関係を、本物と同じ形にする(昇格したメソッドが、別のパッケージの行に当たるため。実例: Go の `sqlx.DB` は `*sql.DB` を埋め込む。骨組みが埋め込まないと、昇格したメソッドを呼ぶセルが、本物と違う形で通る)。

