# ADR-093: Rust の extractor の隔離を、Docker から macOS のサンドボックス(Seatbelt)に替える(ユーザーの決定、2026-11-22。ADR-064 を一部覆す)

- 状態: 有効
- 関連: rust-isolation、PRINCIPLES 8 節
- 状態の詳細: ユーザーの決定(2026-11-22)。ADR-064 を一部覆す。Linux では止まる(`-unsafe-host-exec`)
- 要旨: 承知している弱さと、後回しのハードニングを載せた

**背景:** ADR-064 は、信頼しない Rust の対象を、Docker のコンテナの中で、2 段階(取得 = ネットワークあり、抽出 = ネットワークなし)で読む、と決めた。ユーザーの目的は、`brew install` だけで rivus を使えるようにすること。そのために、Docker への依存をなくす。使い手の環境は macOS(Apple Silicon)。CI は既知のコード(このリポジトリの中のもの、ロックファイルで固定した依存)しか読まず、隔離なしのホストで動いている。Docker が要っていたのは、隔離そのもののテストだけだった。

**決めたこと(ユーザーの決定):**
1. **macOS では、Rust の抽出を、OS のサンドボックス(Seatbelt、`sandbox-exec`)の下で動かす。Docker の経路は、CI を含めて消す**(`docker/rust-extractor/`、コンテナの呼び出し、`-container-*` と `-rust-image` のフラグ、CI の `rust-isolation` ジョブ)。コードは `internal/sandbox/`(プロファイルの生成と起動)と `cmd/rivus/isolate_sandbox_darwin.go`(Rust の 2 段階)。
2. **2 段階は、そのまま**: 取得(`cargo fetch --locked`。ネットワークと `network-outbound`、`system-socket` を許す。mach のサービスは、1 つも許さない。対象の cargo の設定は読ませない = cargo は設定をカレントから探すので、空の作業ディレクトリから実行する。取得前の検査(`Cargo.lock` / `Cargo.toml` の取得元、対象の設定)と、前回の `config*` / `credentials*` の削除は、Docker 版のまま)と、抽出(ネットワークなし。mach は全拒否。書き込みは作業ディレクトリだけ。ホームは対象以外を読ませない)。ビルドスクリプトと proc macro は、作業ディレクトリの中で動く(= 対象のコードの実行そのもの。絞れない)。
3. **rivus 専用の `CARGO_HOME`**(`~/Library/Application Support/rivus/rust/rivus-cargo-<ハッシュ>/`)。取得だけが書き、抽出は読み取り専用。利用者の `~/.cargo` は、読みも書きもしない。**固定の nightly は、rustup を使わず、`bin` を直接** `PATH` に置く(extractor は `RIVUS_RUST_DIRECT=1` のとき `+toolchain` を付けない)。ツールチェインと extractor の用意は、明示のコマンド `rivus setup rust -source <extractor/rust>`(サンドボックスの外で、信頼できるソースから)。
4. **使えないときは、何も実行せずに止まる**(`sandbox-exec` がない、プロファイルが受け入れられない = 対象を動かす前に rivus が確かめる。Linux)。何も報告せずにホストで動かすことはしない。`-unsafe-host-exec` は今のまま(Linux では、これが普通の使い方になる)。**隔離の下か、ホストでかを、`summary.isolation`(`mode`: `sandbox` / `host`)に出す**(Rust の抽出をしたときだけ)。
5. **規則**: rivus は、自分が起動したプロセスグループの外のプロセスに、シグナルを送らない(探して kill する後始末は作らない。事故: 見つけ方の誤りが、利用者の他のプロセス約 120 個の kill になった)。

**承知している弱さ(ユーザーが受け入れた。Docker より弱い / ない所):**
- `sandbox-exec` は、Apple が非推奨にしたまま動いている仕組み。
- メモリ、CPU、プロセス数の**強い上限がない**(時間の上限、プロセスグループごとの kill で代える)。メモリとプロセス数の監視は、後回し。
- **ユーザー境界がない**(利用者と同じユーザーで動く。防御は、プロファイルだけ)。ホストのカーネルに直接触れる(Docker Desktop では、Linux の VM のカーネルだった)。
- **システムのファイルが、読み取りの範囲で見える**。実行できるプログラムは、ホストにある名指しの一覧(`/bin/sh`、`/bin/bash`、`cc`、`ld` など)とツールチェイン。
- **同じユーザーの他のプロセスの引数と環境(`kern.procargs2`)が、サンドボックスの下から読める**(拒否できなかった: 名前、`kern.` の接頭辞、`process-info*` の拒否、どれも止まらない。全体の拒否は、Go のランタイムが異常終了する)。**受け入れの根拠: 読んだものを外に出す経路が、抽出の段階にない**(テストの組 `TestSandboxClosesEveryWayOut`)。**残る経路 = rivus の出力(解析の結果とログ)に、対象が決める文字列が載ること。これも承知のうえで受け入れた**(信頼できないコードの解析の結果とログは、そのコードが書いたものを含みうる。そのまま公開の場所に出さない)。
- **グループを抜けるプロセス**(`setsid`、二重フォーク)は、実行の終わりに止められない(サンドボックスの下のまま。ファイルとネットワークには触れない)。残ったかもしれないことを、報告するだけ。
- **取得の段階のネットワークは、絞っていない**(宛先の許可リストがない)。Docker の既定の bridge と同じで、今より弱くはならない。
- **解析する構成が、macOS 向けに変わる**: `cfg(target_os)` で分かれるコードは、macOS の側を読む(テスト用の crate で確かめた: Docker は `only_on_linux`、サンドボックスは `only_on_macos`)。
- **Linux では、信頼できないコードの Rust の解析ができなくなる**(ユーザーが受け入れた。理由: 主な使い手は全員 macOS。`-unsafe-host-exec` で、隔離なしに読める)。
- **祖先の `.cargo/config.toml` があると、何も実行せずに止まる**(cargo はそれを読めない。ホームの `~/.cargo/config.toml` を持つ利用者は、ホームの下の対象を、サンドボックスの下で抽出できない。回避: そのファイルを一時的に退ける。信頼できるコードなら `-unsafe-host-exec`)。

**後回しにしたハードニング**(課題の一覧に 1 件ずつ。#110 以降): 取得のプロキシ(宛先の許可リスト、解決したアドレスの検査)、`bsd.sb` の許可の棚卸しと変わったら失敗するテスト、メタデータ(存在)が見える範囲の絞り込みとホームの外の利用者のデータのテスト、メモリとプロセス数の監視、残ったかもしれないプロセスの報告(列挙だけ)、プロファイルの各行の「拒否が効いている」テスト、シグナルの `same-sandbox`、端末(制御文字)、macOS の CI のワークフロー(と macOS の `TestDogfood`)、`rivus setup rust` の作り込み、祖先の設定を名指しで読み取り専用に許す案。

**確かめたこと:** `bench/zoo-rust`(174 件)、`bench/rust-curated`(280 件)、crates.io の依存を持つ `extractor/rust`(関数 179 件)で、サンドボックスの下で抽出した結果の finding の ID が、Docker(取り直した基準)と一致。悪意のあるビルドスクリプトが `probe` を動かすテスト(`extractor/rust/test/hostile_sandbox`)と、外に出る経路の組(21 項目)、取得の段階で対象の設定を読まないこと、時間・出力の上限・環境変数・使えないとき止まる、が通る。(最初の Docker の基準 298 / 597 は、古いイメージのもので、誤り。取り直した。)

**フラグ**: `-container-timeout` / `-container-max-output` / `-container-env` / `-container-ro-mount` は `-isolation-timeout` / `-isolation-max-output` / `-isolation-env` / `-isolation-read` になった(古い名前は、別名にせず、新しい名前を示して止まる)。`-container-memory` / `-container-cpus` / `-container-pids` と `-rust-image` は消えた(対応する上限と、イメージがない)。

**ADR-064 との関係:** 2 段階(取得と抽出)、取得で対象の設定を読ませないこと、取得前の検査、`-unsafe-host-exec` の意味は、そのまま。コンテナ(非 root、`--cap-drop ALL`、`--read-only`、メモリ・CPU・プロセス数の上限、専用ネットワークの案)は、サンドボックスに替わり、上の弱さを承知している。

**限界(まだ届かないもの):** macOS の CI のワークフローがない(テストは、開発のマシンで `RIVUS_SBX_RED=1` / `RIVUS_SBX_NET=1` で回す)。extractor のバージョンの確認(古い extractor を何も報告せずに使うことはしない。課題 #109)。

**追記(2026-11-22、ユーザーの決定: 「既定で Rust も解析、必要な警告と [Y/n] の確認だけ」): `-lang` なし(auto)でも、聞いたうえで Rust を読む。**
1. macOS でサンドボックスが使え、標準入力が端末のとき、警告(ビルドが要り build script と proc macro が動く、サンドボックスの下で動く、依存を crates.io から取る、`-lang rust` / `-yes` / `-lang go,dart` の名指し、答えによって別の index)を出して `[Y/n]` を聞く。Enter は Yes。`-yes` で聞かない。`-lang rust` は聞かない(今までどおり)。
2. 聞けないとき(端末でない)は、何も報告せずに Yes にすることはしない。`-yes` がなければ、混在は Rust を読まずにその旨を出す、Rust だけはエラー。サンドボックスがない環境は、auto では聞かず、読まない(`-unsafe-host-exec` があっても Yes 既定にしない)。extractor かツールチェインがなければ、聞く前に分かって、聞かずに読まない。
3. 読まなかった言語は、`summary.unread.skipped_languages` と `meta.unread.skipped_languages`(`why`: `declined` / `no-terminal` / `not-installed` / `no-sandbox`)に残る。`failed_packages` には数えない。そういう index の `path` の「経路なし」は `exhaustive: false`。Rust だけで断ると何も読まず、終了コード 3。
4. 答えは 1 回の実行で 1 度だけ決める。index は、答えを `lang` に記録する(あとの `graph -C` は聞かない)。

