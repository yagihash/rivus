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
| root の特権(rivus を root で動かしたとき) | 実行の前に、全 capability を落とす(bounding set を空に、exec で root に特権を戻す規則(`SECBIT_NOROOT`)を止めて固定し、ambient と inheritable を空に)。`NO_NEW_PRIVS` は特権を落とさない。落とさないと、対象は特権のまま動き、他人のファイルの所有者を変えるなどができた(root で動かしたときのテストで見つけた) |
| ユーザー名前空間、mount、カーネルモジュール | seccomp で `unshare`、`setns`、名前空間を作る `clone`、`mount` 系、`init_module` 系、`kexec`、`reboot` を拒否。`clone3` は `ENOSYS`(C ライブラリは `clone` に戻る)。名前空間の中で root になれると、非特権では届かないカーネルの部分に届く |

## 4. macOS との違い(弱くなる所と、強くなる所)

- **弱い: 取得段階の接続先。** Landlock はポート番号だけで、アドレスは指定できない(macOS は `localhost:PORT`)。動くのは信頼する `cargo fetch` だけで、`build.rs` やマクロは動かない(取得の前の検査と、設定の除去は macOS と同じ)ので、受け入れた(ユーザーの判断、2026-11-25)。
- **弱い: ファイルの存在と属性は見える。** Landlock は `stat` を止めない(読む、列挙する、開くは止まる)。
- **弱い: 「名指しのプログラムだけ実行できる」は、境界ではない。** Landlock の実行の権限は `execve` だけを見る。動的ローダーを直接呼べば(`ld.so /path/prog`)、**読み取りを許した場所**(ライブラリの `/usr/lib`、ツールチェイン)のプログラムは、名指しがなくても動く(実測)。読み取りを許さない場所(`/usr/bin` など)のプログラムは、読めないので動かない。だから、実行の許可は、ファイルとネットワークの境界(秘密が読めない、外へ書けない、ソケットが作れない)に重ねた、もう一段として持つ。ライブラリの置き場所は、読み取りだけを許し、実行は動的ローダーだけに与える(CI の `ubuntu-latest` で、Rust 版の coreutils が `/usr/lib/cargo/bin` にあり、置き場所ごと実行を許していたため、名指しでない `id` が動いた)。
- **弱い: ファイルの属性の変更は、止められない。止めずに、事後に報告する(ユーザーの判断、2026-11-25)。** Landlock は、読む・書く・作る・消す・実行する・切り詰めるを止めるが、`chmod`、`chown`、`utimes`、`setxattr` は止めない(実測: 許可していない、同じユーザーのファイルのモードが 777 に変わった。`ptrace` などと違い、seccomp では、パスを見分けられない)。対象のコードは、利用者のファイルのモード(`~/.ssh` の鍵の 600 を 777 にする、など)と時刻を変えられる。中身は読めず、書けない。macOS の Seatbelt は、これを止める。塞ぐには、別の LSM が要る。GNU ld は出力のモードを `chmod` で決めるので、`chmod` を一律に拒否すると、リンクが壊れる。そこで、seccomp の user notification で、この種の呼び出し(`chmod`、`chown`、`utimes`、`setxattr` の系)を rivus の親に渡し、親がパスを調べて(対象の呼び出しの引数を、`/proc/<pid>/mem` から読み、相対パスは `/proc/<pid>/cwd` と fd から解く)、**書いてよい場所(作業ディレクトリ)の外のものだけ**を、実行の終わりに stderr に出す。呼び出しは、止めずに通す(`SECCOMP_USER_NOTIF_FLAG_CONTINUE`)ので、これは報告で、守りではない。報告の読み取りが失敗しても、通知のファイル記述子は、実行が終わるまで開けておく(閉じると、フィルタは対象に残るので、以降の `chmod` が `ENOSYS` で失敗し、ビルドが壊れる)。失敗は報告に書く。パスは、リンクそのものを対象にする呼び出し(`lchown`、`AT_SYMLINK_NOFOLLOW`)ではリンクの名前、`AT_EMPTY_PATH` ではディスクリプタのファイル。出るのは、対象のコードが求めたこと。取得の段階(信頼する cargo)では使わない。パスが読めないとき(`ptrace` が許されない環境)は、件数だけを出す。
- **古いカーネル(ユーザーの判断、2026-11-25)。** ABI 3 未満(Linux 6.2 未満)は、`truncate` で、同じユーザーのファイルを空にできるので、**何も実行せずに止まる**(`ErrUnavailable`)。ABI 6 未満(6.12 未満)は、同じユーザーの、ほかのプロセスにシグナルを送れる(`kill -9 -1` で、利用者のほかのプロセスを止められる)ので、**警告を出して動かす**(`Result.Warnings`)。ABI 6 以上(実測: ABI 7)では、シグナルは止まる。
- **弱い: 資源の上限は、緩い `RLIMIT` だけ**(ユーザーの判断、2026-11-25)。プロセス(スレッド)の数は、今ある数 + 20000(`RLIMIT_NPROC`。ユーザー全体の数に効くが、効くのは、この上限を持つプロセスの `fork` だけ)、メモリは、マシンのメモリの大きさ(`RLIMIT_DATA`。書くメモリ。マップしたファイルは数えない)、ファイルの大きさは 32 GiB(`RLIMIT_FSIZE`)、core は無し。普通の大きなプロジェクトのビルドは届かず、フォーク爆弾やメモリを使い切るものは届く、緩さ。強い上限(cgroup)は、利用者が頼れないので、使わない。CPU は、時間の上限だけ。
- **弱い: ユーザー境界がない**(macOS と同じ)。
- **古いカーネル**: ABI 4 未満では、ポート指定の取得ができない(止まる)。ABI 6 未満では、他プロセスへのシグナルは、塞げない(seccomp では、宛先を見分けられない)。抽出の段階の「ネットワークなし」は、ABI によらず seccomp で成り立つ。
- **強い: `/proc`(他プロセスの `environ` など)が読めない。** macOS は `kern.procargs2` を拒否できなかった。

## 5. テスト

`internal/sandbox/landlock_linux_test.go`。プローブ(`probelinux`、静的にビルドする)が、敵対的なコードの試みを 1 つずつして、`BLOCKED` / `ESCAPED` を言う。

- サンドボックスなしでは、試みが**できる**こと(`TestSandboxProbeEscapesOnTheHost`)。これがないと、「拒否」がプローブの失敗かもしれない。
- サンドボックスの下では、全部 `BLOCKED`(`TestSandboxClosesEveryWayOut`)。seccomp を外すと、UDP、UNIX ソケット、`io_uring`、`ptrace` が `ESCAPED` になることを、手で確かめた(Landlock だけでは足りない、という根拠)。
- プロキシのポートだけに届く、別のポートは届かない(`TestSandboxNetworkProxyOnly`)。
- 実際の `cargo check` で、`build.rs` が C コンパイラは動かせて、秘密の読み取り、ホームへの書き込み、ネットワーク、`curl` はできない(`TestSandboxBuildScriptIsContained`)。cargo がない環境では飛ばす。
- CI(`ci.yml`): `sandbox-linux` は、`ubuntu-latest`、`ubuntu-26.04-arm`、`ubuntu-24.04`、`ubuntu-24.04-arm` で、同じテストを回す(**どのランナーも Linux 7.0 系**で、Landlock は新しい。24.04 との違いは、ユーザーランド(GNU の coreutils。`ubuntu-latest` は Rust 版で、`/usr/lib/cargo/bin` にある)と、アーキテクチャ)。**古いカーネルの経路**は、実機がないので、テストのバイナリだけで、`RIVUS_SBX_FAKE_ABI=N` が Landlock の ABI を N に下げる(古いカーネルが知らない権限を扱わないので、古いカーネルと同じ設定になる)ことで作る(`TestSandboxOldKernels`: ABI 2 は拒否、3 と 5 は警告つきで動き、シグナルが素通りする、6 は警告なし)。`RIVUS_SBX_RED=1` で、サンドボックスが作れないランナーは、飛ばさずに失敗にする。ランナーは root でないので、root のときだけ効く確認(capability を落とすこと、ファイルを他のユーザーに渡せないこと)は、同じテストを `sudo` で、もう一度回す。`sandbox-e2e-linux`(と、macOS の `sandbox-e2e-darwin`)は、固定の nightly、extractor(`rivus setup rust`)、悪意のある `build.rs` と proc macro の依存を持つ crate を、`rivus analyze -lang rust` で読む(`scripts/sandbox-e2e.sh`。macOS も同じ。対照として、同じ `build.rs` をホストで直接動かすと、出られることも確かめる)。各ジョブが動く条件: PR では、変更したファイルによって、macOS のジョブ(`sandbox`、`sandbox-e2e-darwin`)と Linux のジョブ(`sandbox-linux`、`sandbox-e2e-linux`)が、それぞれ必要なときだけ動きます(`ci.yml` の `changes` ジョブの `isolation_common`、`isolation_darwin`、`isolation_linux`。共通のファイル(`internal/sandbox/sandbox.go`、`cmd/rivus/isolate*.go`、`lang.go`、`setup.go`、`internal/report/**`、`extractor/rust/**`、`go.mod`、`ci.yml` など)は両方、`*_darwin*` は macOS だけ、`*_linux*` は Linux だけ)。main への push、週 1 回、手動では、両方が動きます。 seccomp のフィルタのない arch(riscv64)でも、ビルドと `go vet` が通ることを、`go-packages` で確かめる。

## 6. まだのもの


**確認済み(2026-11-25、クラウドのセッション。x86-64、Landlock ABI 7)**: 固定の nightly を `rustup` で取得し、`rivus setup rust -source extractor/rust -y` で extractor を作り、`serde`(proc macro と `build.rs`)、`serde_json`、`shlex` を依存に持つ crate と、悪意のある `build.rs` を持つ crate を、`rivus analyze -lang rust` で読んだ。取得(ネットワークあり。名前解決と TLS を含む)から抽出(ネットワークなし)まで通り、`summary.isolation.mode` は `sandbox`、悪意のある `build.rs` の書き込みは作られなかった。

- `bench/zoo-rust`、`bench/rust-curated` の同一性の確認(Linux で抽出した IR の finding の ID が、macOS のものと、`cfg(target_os)` の違いを除いて一致するか)。
- CI の `sandbox-linux`(`ubuntu-latest` と `ubuntu-26.04-arm`)の結果。
- ABI 4 未満、ABI 6 未満の実機での確認。arm64 の実行(コードは `GOARCH=arm64` でビルド、vet は通る。実行は CI の `ubuntu-26.04-arm` で初めて分かる)。
- `isolate_sandbox_linux.go` は、`isolate_sandbox_darwin.go` の写しに近い。共通の部分(取得の検査、結果の扱い)を、`darwin || linux` のファイルに寄せる整理が要る。
