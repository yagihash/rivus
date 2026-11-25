# 設計: Rust の extractor の隔離を、Linux のサンドボックス(Landlock + seccomp)にも広げる

> **状態: 一部実装(試作。サンドボックス、cargo での閉じ込め、固定の nightly と extractor での端から端まで(取得と抽出)は、実装・確認した。同一性の確認と CI の結果は、まだ)。** macOS の Seatbelt([DESIGN-macos-sandbox.md](DESIGN-macos-sandbox.md))と同じ形で、Linux に隔離を足す。課題 #148 の 8 節の論点への答え。正はコード(`internal/sandbox/landlock_linux.go`、`seccomp_linux.go`、`cmd/rivus/isolate_sandbox_linux.go`)。

決定の記録は [ADR-128](adr/128-linux-sandbox-landlock-seccomp.md)。利用者向けの説明は [rust-isolation.md](rust-isolation.md)。

## 1. 何を使うか

Landlock(ファイルと TCP のポート)+ seccomp(ソケット全般と、Landlock を迂回する system call)+ `NO_NEW_PRIVS`。bubblewrap は使わない。

- 外部のプログラムも、非特権のユーザー名前空間も要らない。Ubuntu 24.04 の AppArmor や、コンテナの中で、userns が塞がれていても動く。
- Seatbelt と同じ「既定拒否 + 許可を 1 行ずつ」の形で、`Spec`(`Read` / `Exec` / `Write` / `ExecFiles` / `ReadFiles` / `ProxyPort` / `Network`)をそのまま写せる。
- 作れないとき(Landlock がない、seccomp が通らない、カーネルが古い)は、何も実行せずに止まる(`ErrUnavailable`)。ホストでの実行に切り替えることはしない。

## 2. 作り方

Landlock と seccomp は、自分と子にだけ効き、外せない。そこで、`rivus` 自身をヘルパーとして再実行する(`argv[0]` が `rivus-sandbox-helper`。`sandbox` パッケージの `init` が `main` の前に拾う)。

1. 親が、許可の一覧を fd 3 のパイプで渡す。fd 4 は、失敗の理由を書くパイプ。
2. ヘルパーが、`NO_NEW_PRIVS`、Landlock、seccomp の順に自分を閉じ込めて、`execve` で対象のプログラムになる。
3. `execve` で fd 4 が閉じる(CLOEXEC)。**何も書かれずに閉じたら、閉じ込めは済んでいる**。何か書かれていたら、対象は 1 行も動いていない(`ErrUnavailable` + 理由)。

対象のプログラムの標準エラーの内容を、閉じ込めの成否の判定には使わない(macOS の `Available()` と同じ原則)。

## 3. 何を閉じるか

| 項目 | 閉じ方 |
| --- | --- |
| ファイルの読み取り・書き込み・実行 | Landlock。許可した場所だけ。書き込みは作業用ディレクトリだけ |
| ネットワーク(抽出) | seccomp で `socket()` を全拒否(TCP も UDP も raw も UNIX ソケットも)+ ABI 4 以上なら Landlock の TCP も全拒否 |
| ネットワーク(取得、プロキシのポート指定) | Landlock でそのポートだけに `connect`(ABI 4 以上が必須。足りなければ止まる)+ seccomp で `socket()` を IPv4/IPv6 の TCP だけに |
| ネットワーク(取得、開放) | 絞らない(今の macOS の取得段階と同じ) |
| 他プロセスへのシグナル、abstract UNIX ソケット | Landlock の scope(ABI 6、Linux 6.12 以上) |
| 他プロセスのメモリ・fd・環境 | seccomp で `ptrace`、`process_vm_*`、`pidfd_getfd` を拒否。`/proc` は許可に入れない |
| `io_uring`、`bpf`、`perf_event_open`、`userfaultfd`、`keyctl` | seccomp で拒否(`io_uring` は、フィルタされた system call を通らずに、ソケットを作りファイルを開ける) |
| 別の ABI の system call | seccomp が、アーキテクチャが違うものと、x86-64 の x32 の番号を、プロセスごと殺す |
| setuid | `NO_NEW_PRIVS` |

## 4. macOS との違い(弱くなる所と、強くなる所)

- **弱い: 取得段階の接続先。** Landlock はポート番号だけで、アドレスは指定できない(macOS は `localhost:PORT`)。動くのは信頼する `cargo fetch` だけで、`build.rs` やマクロは動かない(取得の前の検査と、設定の除去は macOS と同じ)ので、受け入れた(ユーザーの判断、2026-11-25)。
- **弱い: ファイルの存在と属性は見える。** Landlock は `stat` を止めない(読む、列挙する、開くは止まる)。
- **弱い: 「名指しのプログラムだけ実行できる」は、境界ではない。** Landlock の実行の権限は `execve` だけを見る。動的ローダーを直接呼べば(`ld.so /path/prog`)、**読み取りを許した場所**(ライブラリの `/usr/lib`、ツールチェイン)のプログラムは、名指しがなくても動く(実測)。読み取りを許さない場所(`/usr/bin` など)のプログラムは、読めないので動かない。だから、実行の許可は、ファイルとネットワークの境界(秘密が読めない、外へ書けない、ソケットが作れない)に重ねた、もう一段として持つ。ライブラリの置き場所は、読み取りだけを許し、実行は動的ローダーだけに与える(CI の `ubuntu-latest` で、Rust 版の coreutils が `/usr/lib/cargo/bin` にあり、置き場所ごと実行を許していたため、名指しでない `id` が動いた)。
- **弱い: ユーザー境界がない**(macOS と同じ)。
- **古いカーネル**: ABI 4 未満では、ポート指定の取得ができない(止まる)。ABI 6 未満では、他プロセスへのシグナルは、塞げない(seccomp では、宛先を見分けられない)。抽出の段階の「ネットワークなし」は、ABI によらず seccomp で成り立つ。
- **強い: `/proc`(他プロセスの `environ` など)が読めない。** macOS は `kern.procargs2` を拒否できなかった。
- **未実装: メモリ・プロセス数の上限。** `RLIMIT_AS` が Linux では使える(macOS は使えない)ので、足せる。今は、時間の上限とプロセスグループごとの kill だけ(macOS と同じ)。

## 5. テスト

`internal/sandbox/landlock_linux_test.go`。プローブ(`probelinux`、静的にビルドする)が、敵対的なコードの試みを 1 つずつして、`BLOCKED` / `ESCAPED` を言う。

- サンドボックスなしでは、試みが**できる**こと(`TestSandboxProbeEscapesOnTheHost`)。これがないと、「拒否」がプローブの失敗かもしれない。
- サンドボックスの下では、全部 `BLOCKED`(`TestSandboxClosesEveryWayOut`)。seccomp を外すと、UDP、UNIX ソケット、`io_uring`、`ptrace` が `ESCAPED` になることを、手で確かめた(Landlock だけでは足りない、という根拠)。
- プロキシのポートだけに届く、別のポートは届かない(`TestSandboxNetworkProxyOnly`)。
- 実際の `cargo check` で、`build.rs` が C コンパイラは動かせて、秘密の読み取り、ホームへの書き込み、ネットワーク、`curl` はできない(`TestSandboxBuildScriptIsContained`)。cargo がない環境では飛ばす。
- CI: `ci.yml` の `sandbox-linux`(`ubuntu-latest` と `ubuntu-26.04-arm`)。`RIVUS_SBX_RED=1` で、サンドボックスが作れないランナーは、飛ばさずに失敗にする。

## 6. まだのもの

**確認済み(2026-11-25、クラウドのセッション。x86-64、Landlock ABI 7)**: 固定の nightly を `rustup` で取得し(約 1 分)、`rivus setup rust -source extractor/rust -y` で extractor を作り(数秒)、`serde`(proc macro と `build.rs`)、`serde_json`、`shlex` を依存に持つ crate と、悪意のある `build.rs` を持つ crate を、`rivus analyze -lang rust` で読んだ。取得(ネットワークあり。名前解決と TLS を含む)から抽出(ネットワークなし)まで通り、`summary.isolation.mode` は `sandbox`、悪意のある `build.rs` の書き込みは作られなかった。

- `bench/zoo-rust`、`bench/rust-curated` の同一性の確認(Linux で抽出した IR の finding の ID が、macOS のものと、`cfg(target_os)` の違いを除いて一致するか)。
- CI の `sandbox-linux`(`ubuntu-latest` と `ubuntu-26.04-arm`)の結果。
- ABI 4 未満、ABI 6 未満の実機での確認。arm64 の実行(コードは `GOARCH=arm64` でビルド、vet は通る。実行は CI の `ubuntu-26.04-arm` で初めて分かる)。
- `isolate_sandbox_linux.go` は、`isolate_sandbox_darwin.go` の写しに近い。共通の部分(取得の検査、結果の扱い)を、`darwin || linux` のファイルに寄せる整理が要る。
