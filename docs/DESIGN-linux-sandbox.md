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

対象のプログラムの標準エラーの内容を、閉じ込めの成否の判定には使わない(macOS の `Available()` と同じ原則)。端末に流すときは、macOS と同じ `sanitizeForTerminal` を通す(`cmd/rivus/isolate_sandbox_linux.go` の `sandboxResult`。SBX-4)。

## 3. 何を閉じるか

| 項目 | 閉じ方 |
| --- | --- |
| ファイルの読み取り・書き込み・実行 | Landlock。許可した場所だけ。書き込みは作業用ディレクトリだけ |
| ネットワーク(抽出) | seccomp で `socket()` を全拒否(TCP も UDP も raw も UNIX ソケットも)+ ABI 4 以上なら Landlock の TCP も全拒否 |
| ネットワーク(取得、プロキシのポート指定) | Landlock でそのポートだけに `connect`(ABI 4 以上が必須。足りなければ止まる)+ seccomp で `socket()` を IPv4/IPv6 の TCP だけに(プロトコルは既定か TCP: ポートの規則は TCP だけを知るので、同じストリームでも SCTP と MPTCP は拒否) |
| ネットワーク(取得、開放) | 絞らない(今の macOS の取得段階と同じ) |
| 他プロセスへのシグナル、abstract UNIX ソケット | Landlock の scope(ABI 6、Linux 6.12 以上) |
| 他プロセスのメモリ・fd・環境 | seccomp で `ptrace`、`process_vm_*`、`pidfd_getfd` を拒否。`/proc` は許可に入れない |
| `io_uring`、`bpf`、`perf_event_open`、`userfaultfd`、`keyctl` | seccomp で拒否(`io_uring` は、フィルタされた system call を通らずに、ソケットを作りファイルを開ける) |
| データグラムの `socketpair`、`ioctl`(`FS_IOC_SETFLAGS`、`FS_IOC_FSSETXATTR`)、`MSG_FASTOPEN` つきの送信(プロキシのポートの指定のとき) | seccomp で拒否。`socketpair` は、つながった端だけ(STREAM と SEQPACKET。Rust の標準ライブラリが使う)。データグラムの端は任意のアドレスへ送れる。inode のフラグの `ioctl` は、読み取りで開いた fd で効き、Landlock が止めない属性の変更。TCP Fast Open の送信は、Landlock のポートの規則が見る `connect` を通らずに接続する |
| 別の ABI の system call | seccomp が、アーキテクチャが違うものと、x86-64 の x32 の番号を、プロセスごと殺す |
| SysV の共有メモリ・メッセージキュー・セマフォ、POSIX メッセージキュー、`kcmp` | seccomp で拒否。IPC の名前空間はホストのもので(名前空間は作らせない)、Landlock も止めないので、同じユーザーの他のプロセス(X11 クライアントの共有メモリなど)のものを、接続して読み書きできてしまう |
| 起動した側が開けたままにした fd | 6 番以降を close-on-exec にする(`close_range`。できなければ、何も実行せずに止まる)。Landlock と seccomp は、すでに開いている fd の使用を止めない |
| ユーザーの他のプロセスの上限・優先度・CPU・メモリの置き場所(`prlimit64`、`setpriority`、`ioprio_set`、`sched_set*`、`migrate_pages`、`move_pages`、`process_madvise`) | seccomp で、自分(引数の 0)以外は `EPERM`(`process_*` は全部拒否)。Landlock が止めるのはシグナルだけで、同じユーザーのプロセスには、これらが通り、たとえば他のプロセスの `RLIMIT_NOFILE` を 3 に下げられた(実測)。引数の自分の番号(`getpid()` の値)も、フィルタには自分と分からないので、拒否する(0 を使う) |
| setuid | `NO_NEW_PRIVS` |
| root の特権(rivus を root で動かしたとき) | 実行の前に、全 capability を落とし、**落とせたか確かめて**、落とせなければ(root で、`CAP_SETPCAP` がないコンテナ)、何も実行せずに止まる。上限(`RLIMIT`)も、設定できなければ止まる。落とす内容は(bounding set を空に、exec で root に特権を戻す規則(`SECBIT_NOROOT`)を止めて固定し、ambient と inheritable を空に)。`NO_NEW_PRIVS` は特権を落とさない。落とさないと、対象は特権のまま動き、他人のファイルの所有者を変えるなどができた(root で動かしたときのテストで見つけた) |
| ユーザー名前空間、mount、カーネルモジュール | seccomp で `unshare`、`setns`、名前空間を作る `clone`、`mount` 系、`init_module` 系、`kexec`、`reboot` を拒否。`clone3` は `ENOSYS`(C ライブラリは `clone` に戻る)。名前空間の中で root になれると、非特権では届かないカーネルの部分に届く |

## 4. macOS との違い(弱くなる所と、強くなる所)

- **弱い: 取得段階の接続先。** Landlock はポート番号だけで、アドレスは指定できない(macOS は `localhost:PORT`)。動くのは信頼する `cargo fetch` だけで、`build.rs` やマクロは動かない(取得の前の検査と、設定の除去は macOS と同じ)ので、受け入れた(ユーザーの判断、2026-11-25)。
- **弱い: ファイルの存在と属性は見える。** Landlock は `stat` を止めない(読む、列挙する、開くは止まる)。
- **弱い: 「名指しのプログラムだけ実行できる」は、境界ではない。** Landlock の実行の権限は `execve` だけを見る。動的ローダーを直接呼べば(`ld.so /path/prog`)、**読み取りを許した場所**(ライブラリの `/usr/lib`、ツールチェイン)のプログラムは、名指しがなくても動く(実測)。読み取りを許さない場所(`/usr/bin` など)のプログラムは、読めないので動かない。だから、実行の許可は、ファイルとネットワークの境界(秘密が読めない、外へ書けない、ソケットが作れない)に重ねた、もう一段として持つ。ライブラリの置き場所は、読み取りだけを許し、実行は動的ローダーだけに与える(CI の `ubuntu-latest` で、Rust 版の coreutils が `/usr/lib/cargo/bin` にあり、置き場所ごと実行を許していたため、名指しでない `id` が動いた)。
- **ファイルの属性の変更(`chmod`、`chown`、`utimes`、`setxattr` の系)は、rivus が代行し、外のものは拒否する(ユーザーの判断、2026-11-25。外部の設計者のレビューで、「事後の報告」が偽装できると分かった)。** Landlock は、読む・書く・作る・消す・実行する・切り詰めるを止めるが、これらは止めない(実測: 許可していない、同じユーザーのファイルのモードが 777 に変わった)。seccomp ではパスを見分けられず、一律に拒否すると GNU ld のリンクが壊れる(出力のモードを `chmod` で決める)。そこで、seccomp の user notification で、これらの呼び出しを rivus の親に渡し、**親が代わりに実行する**。`SECCOMP_USER_NOTIF_FLAG_CONTINUE` で通すと、カーネルが、パスをもう一度、対象のメモリから読む。対象の別のスレッドが、親が読んだあと、カーネルが読み直す前に、パスを書き換えると、検査したファイルと、変更されるファイルが、別になる(実測: 外のファイル 400 個のうち約 190 個のモードが変わり、そのうち約半数は、報告にも出なかった。seccomp_unotify(2) もこの使い方を戒めている。fd を使う形も、別のスレッドが `dup2` で fd の指す先を変えると、同じ)。代行では、検査と変更が、同じファイルに対して行われる。
  - **引数は、対象のメモリから一度だけコピーする**(パス、xattr の名前と値、時刻)。
  - **ファイルは、一度だけ開く**(`O_PATH`。fd を使う形は `pidfd_getfd` で、そのときの開いたファイルを取る)。**検査するのは、開いたファイルのパス**(`/proc/self/fd/N` の `readlink`。読み直せる文字列ではない)で、変更も、そのファイルに対して行う。作業ディレクトリの外のファイルは、`EPERM` を返し(何も変更しない)、報告に出す。
  - **パスの解決は、カーネルがする**(`openat` の `O_PATH`。`..` もリンクも、カーネルの解釈どおり)。`/proc/self/…` と `/dev/fd/…` は、rivus ではなく、対象のものとして読み替える。
  - **代行するスレッドは、capability を持たない**(rivus を root か capability つきで動かすと、対象と同じになるよう、先にスレッドの capability を捨てる。スレッドごとの性質なので、このスレッドだけ。捨てられなければ、すべて拒否する)。対象より多くのことを、代わりにしないため(root で、他のユーザーに渡す、など)。
  - **`/dev/null` は、作業用ディレクトリの外**(ベースラインで書けるが、ホストのデバイス): 属性の変更は拒否する。rivus を root で動かしたとき、代行すると、ホストの `/dev/null` のモードを変えられ、ほかのユーザーが開けなくなる。
  - **代行する形**は、`chmod`、`fchmod`、`fchmodat`、`fchmodat2`、`chown`(`lchown`、`fchown`、`fchownat`)、`utimensat`(`futimens` を含む)、`setxattr`(`lsetxattr`、`fsetxattr`)、`removexattr`(`lremovexattr`、`fremovexattr`)。リンクそのもの(`lchown`、`AT_SYMLINK_NOFOLLOW`)と `AT_EMPTY_PATH` も扱う。リンクの `chmod` は `EOPNOTSUPP`、リンクの `user.*` は `EPERM`(カーネルと同じ)。**ほかの形は、`EPERM` で拒否する**: 古い `utime`、`utimes`、`futimesat`(x86-64 だけ)、新しい `setxattrat`、`removexattrat`(Linux 6.13)、`file_setattr`(6.17)。ビルドに要らない形を、引数の最後まで理解して代行するより、拒否する方が確実なため。
  - **検査できなかった呼び出しは、拒否する**(対象のメモリが読めない: `ptrace` が許されない環境、たとえば Yama の `ptrace_scope` が 2 以上)。件数を警告に出す。**そういう環境では、リンカの `chmod` も拒否されるので、ビルドが失敗する**(安全な側の失敗)。
  - 読み取りが失敗したときは、連続して 20 回(10 ミリ秒の間隔)までやり直す(読めたら数え直す)。1 回目で記述子を閉じると、フィルタは対象に残るので、以降の呼び出しが `ENOSYS` で失敗する。かといって、無制限に待つと、応答を待つ呼び出しが止まり、実行は時間の上限まで終わらない。上限に達したら、記述子を閉じ(待っている呼び出しと以降の呼び出しは、すぐ `ENOSYS` で失敗する)、警告に出す。
  - 取得の段階(信頼する cargo)では使わない(制限しない)。
- **古いカーネル(ユーザーの判断、2026-11-25)。** ABI 3 未満(Linux 6.2 未満)は、`truncate` で、同じユーザーのファイルを空にできるので、**何も実行せずに止まる**(`ErrUnavailable`)。ABI 6 未満(6.12 未満)は、同じユーザーの、ほかのプロセスにシグナルを送れる(`kill -9 -1` で、利用者のほかのプロセスを止められる)ので、**警告を出して動かす**(`Result.Warnings`)。ABI 6 以上(実測: ABI 7)では、シグナルは止まる。
- **弱い: 資源の上限は、緩い `RLIMIT` だけ**(ユーザーの判断、2026-11-25)。プロセス(スレッド)の数は、今ある数 + 20000(`RLIMIT_NPROC`。ユーザー全体の数に効くが、効くのは、この上限を持つプロセスの `fork` だけ)、メモリは、マシンのメモリの大きさ(`RLIMIT_DATA`。書くメモリ。マップしたファイルは数えない)、ファイルの大きさは 32 GiB(`RLIMIT_FSIZE`)、core は無し。普通の大きなプロジェクトのビルドは届かず、フォーク爆弾やメモリを使い切るものは届く、緩さ。強い上限(cgroup)は、利用者が頼れないので、使わない。呼び出し側(`ulimit` で rivus を動かした人)が決めた上限は、ゆるめない(小さい方を取る)。CPU は、時間の上限だけ。
- **弱い: ユーザー境界がない**(macOS と同じ)。
- **古いカーネル**: ABI 4 未満では、ポート指定の取得ができない(止まる)。ABI 6 未満では、他プロセスへのシグナルは、塞げない(seccomp では、宛先を見分けられない)。抽出の段階の「ネットワークなし」は、ABI によらず seccomp で成り立つ。
- **強い: `/proc`(他プロセスの `environ` など)が読めない。** macOS は `kern.procargs2` を拒否できなかった。

## 5. テスト

`internal/sandbox/landlock_linux_test.go`。プローブ(`probelinux`、静的にビルドする)が、敵対的なコードの試みを 1 つずつして、`BLOCKED` / `ESCAPED` を言う。

- サンドボックスなしでは、試みが**できる**こと(`TestSandboxProbeEscapesOnTheHost`)。これがないと、「拒否」がプローブの失敗かもしれない。
- サンドボックスの下では、全部 `BLOCKED`(`TestSandboxClosesEveryWayOut`)。seccomp を外すと、UDP、UNIX ソケット、`io_uring`、`ptrace` が `ESCAPED` になることを、手で確かめた(Landlock だけでは足りない、という根拠)。
- プロキシのポートだけに届く、別のポートは届かない(`TestSandboxNetworkProxyOnly`)。
- 実際の `cargo check` で、`build.rs` が C コンパイラは動かせて、秘密の読み取り、ホームへの書き込み、ネットワーク、`curl` はできない(`TestSandboxBuildScriptIsContained`)。cargo がない環境では飛ばす。
- CI(`ci.yml`): `sandbox-linux` は、`ubuntu-latest`、`ubuntu-26.04-arm`、`ubuntu-24.04`、`ubuntu-24.04-arm` で、同じテストを回す(**どのランナーも Linux 7.0 系**で、Landlock は新しい。24.04 との違いは、ユーザーランド(GNU の coreutils。`ubuntu-latest` は Rust 版で、`/usr/lib/cargo/bin` にある)と、アーキテクチャ)。**古いカーネルの経路**は、実機がないので、テストのバイナリだけで、`RIVUS_SBX_FAKE_ABI=N` が Landlock の ABI を N に下げる(古いカーネルが知らない権限を扱わないので、古いカーネルと同じ設定になる)ことで作る(`TestSandboxOldKernels`: ABI 2 は拒否、3 と 5 は警告つきで動き、シグナルが素通りする、6 は警告なし)。`RIVUS_SBX_RED=1` で、サンドボックスが作れないランナーは、飛ばさずに失敗にする。ランナーは root でないので、root のときだけ効く確認(capability を落とすこと、ファイルを他のユーザーに渡せないこと)は、同じテストを `sudo` で、もう一度回す。`sandbox-e2e-linux`(と、macOS の `sandbox-e2e-darwin`)は、固定の nightly、extractor(`rivus setup rust`)、悪意のある `build.rs` と proc macro の依存を持つ crate を、`rivus analyze -lang rust` で読む(`scripts/sandbox-e2e.sh`。macOS も同じ。対照として、同じ `build.rs` をホストで直接動かすと、出られることも確かめる)。各ジョブが動く条件: PR では、変更したファイルによって、macOS のジョブ(`sandbox`、`sandbox-e2e-darwin`)と Linux のジョブ(`sandbox-linux`、`sandbox-e2e-linux`)が、それぞれ必要なときだけ動きます(`ci.yml` の `changes` ジョブの `isolation_common`、`isolation_darwin`、`isolation_linux`。共通のファイル(`internal/sandbox/sandbox.go`、`cmd/rivus/isolate*.go`、`lang.go`、`setup.go`、`internal/report/**`、`extractor/rust/**`、`go.mod`、`ci.yml` など)は両方、`*_darwin*` は macOS だけ、`*_linux*` は Linux だけ)。main への push、週 1 回、手動では、両方が動きます。 seccomp のフィルタのない arch(386、arm、riscv64、ppc64le、s390x、loong64、mips64le)でも、ビルドと `go vet` が通ることを、`go-packages` で確かめる(そこでは、ヘルパーが「作れない」と言って、何も実行しない)。

## 6. まだのもの


**確認済み(2026-11-25、クラウドのセッション。x86-64、Landlock ABI 7)**: 固定の nightly を `rustup` で取得し、`rivus setup rust -source extractor/rust -y` で extractor を作り、`serde`(proc macro と `build.rs`)、`serde_json`、`shlex` を依存に持つ crate と、悪意のある `build.rs` を持つ crate を、`rivus analyze -lang rust` で読んだ。取得(ネットワークあり。名前解決と TLS を含む)から抽出(ネットワークなし)まで通り、`summary.isolation.mode` は `sandbox`、悪意のある `build.rs` の書き込みは作られなかった。

- `bench/zoo-rust`、`bench/rust-curated` の同一性の確認(Linux で抽出した IR の finding の ID が、macOS のものと、`cfg(target_os)` の違いを除いて一致するか)。
- CI の `sandbox-linux`(`ubuntu-latest` と `ubuntu-26.04-arm`)の結果。
- ABI 4 未満、ABI 6 未満の実機での確認。arm64 の実行(コードは `GOARCH=arm64` でビルド、vet は通る。実行は CI の `ubuntu-26.04-arm` で初めて分かる)。
- `isolate_sandbox_linux.go` は、`isolate_sandbox_darwin.go` の写しに近い。共通の部分(取得の検査、結果の扱い)を、`darwin || linux` のファイルに寄せる整理が要る。
