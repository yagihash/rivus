# ADR-128: Linux では、Rust の extractor の隔離を、Landlock + seccomp で作る(試作。ユーザーの了承、2026-11-25。ADR-093 の「Linux では止まる」を一部覆す)

- 状態: 有効
- 関連: ADR-093、ADR-064、rust-isolation、DESIGN-linux-sandbox、DESIGN-macos-sandbox 8 節、#148
- 状態の詳細: 試作。サンドボックス、cargo での閉じ込め、固定の nightly と extractor での端から端まで(取得と抽出。クラウドのセッションで確認)は実装・確認した。同一性の確認と CI の結果は、まだ
- 要旨: bubblewrap でなく Landlock + seccomp + `NO_NEW_PRIVS`。取得段階のネットワークの弱さ(ポートまでしか絞れない)は、敵対的なコードが動かない段階なので受け入れた

**背景:** ADR-093 は、Rust の隔離を macOS の Seatbelt にし、Linux では「何も実行せずに止まる」とした(使い手が macOS だったため)。Linux の隔離は課題 #148 に残り、DESIGN-macos-sandbox の 8 節は bubblewrap を最有力としていた。bubblewrap は、外部のプログラムと非特権のユーザー名前空間に頼るが、Ubuntu 24.04 の AppArmor の制限やコンテナの中では、後者が塞がれていることが多い。

**決めたこと:**
1. **Linux の隔離は、Landlock(ファイルと TCP のポート)+ seccomp(ソケット全般と、Landlock を迂回する system call)+ `NO_NEW_PRIVS`。** 外部のプログラムも、ユーザー名前空間も要らない。`internal/sandbox` の `Spec` を、macOS と同じ形で写す。
2. **作り方: rivus 自身をヘルパーとして再実行し、そこで自分を閉じ込めてから対象を exec する。** 閉じ込めの成否は、専用のパイプで受け取る(対象の標準エラーは判定に使わない)。作れなければ、何も実行せずに止まる(ホストでの実行に切り替えない)。
3. **抽出の段階のネットワークなしは、seccomp で `socket()` を全拒否にして成り立たせる**(Landlock は TCP しか知らない。seccomp を外すと UDP、UNIX ソケット、`io_uring` が抜けることを確かめた)。`io_uring`、`ptrace`、`process_vm_*`、`pidfd_getfd`、`bpf` などは塞ぐ。`/proc` は許可に入れない。seccomp はアーキテクチャごと(x86-64、arm64)に作り、ほかの ABI の system call は殺す。
4. **受け入れた弱さ(ユーザーの了承): 取得段階のポート指定は、アドレスまで絞れない**(macOS は `localhost:PORT`)。取得で動くのは信頼する `cargo fetch` だけで、対象のコードは動かない。今の取得は、macOS と同じく、ネットワークを開けている。
5. **Landlock ABI の下限**: 抽出は ABI 3(Linux 6.2)以上 + seccomp(ABI 3 未満は `truncate` を止められないので、止まる。下の 7)。プロキシのポート指定の取得は ABI 4(Linux 6.7)以上(足りなければ止まる)。他プロセスへのシグナルの scope は ABI 6(6.12)以上で、足りないとき、その分は塞げない。
6. **CI**: `sandbox-linux`(`ubuntu-latest`、`ubuntu-26.04-arm`、`ubuntu-24.04`、`ubuntu-24.04-arm`。どれも Linux 7.0 系なので、古いカーネルは、テストのバイナリで Landlock の ABI を下げて作る)と、root での再実行、`sandbox-e2e-linux` と `sandbox-e2e-darwin`。macOS のジョブと Linux のジョブは、PR では、変更したファイルによって、それぞれ必要なときだけ動く(共通のファイルは両方)。`RIVUS_SBX_RED=1` で、サンドボックスが作れないランナーは失敗にする。

7. **Landlock が止めないもの、古いカーネル、資源の上限の扱い(ユーザーの判断、2026-11-25)**: (a) `chmod`、`chown`、`utimes`、`setxattr` の系は、Landlock が止めない(パスを見分けられず、`chmod` を一律に拒否すると GNU ld が壊れる)。最初は、受け入れ、seccomp の user notification で親に渡して、作業ディレクトリの外のものだけを事後に報告する(呼び出しは `CONTINUE` で通す)としたが、外部の設計者のレビューで、**この報告は、別のスレッドがパスを書き換えると偽装できる**(カーネルが、パスを、検査のあとに読み直す)と分かり、実験でも確かめた(外のファイル 400 個のうち約 190 個が変わり、約半数が報告に出なかった)。そこで、**rivus が代わりに実行する**ことにした(ユーザーの判断、2026-11-25: 主要な形だけ代行し、ほかは拒否する): 引数は一度だけコピーし、ファイルは一度だけ開き、開いたファイルのパスを検査して、そのファイルを変更する。外は `EPERM`。fd の形は `pidfd_getfd` で固定し、代行するスレッドは capability を捨てる。ほかの形(`utime`、`utimes`、`futimesat`、`setxattrat`、`removexattrat`、`file_setattr`)は拒否する。(b) Landlock ABI 3 未満(Linux 6.2 未満)は、`truncate` を止められないので、何も実行せずに止まる。ABI 6 未満(6.12 未満)は、シグナルを止められないので、警告を出して動かす。(c) 資源は、緩い `RLIMIT`(プロセス数、メモリ(`RLIMIT_DATA`)、ファイルの大きさ、core 無し)を足す。cgroup は、利用者が頼れないので使わない。
8. **root で動かしたとき**: 実行の前に、全 capability を落とす。ユーザー名前空間と `mount` 系は、seccomp で拒否する。
9. **クラウドのセッションでも、Rust は隔離の下で読める**(この環境で確認)ので、モデル作成の「Rust は CI に任せる」の線は、あらためた(MODEL-RUNBOOK.md の 0 節)。取得したクレートのコードを、隔離なしで実行しない規則は、そのまま。

**帰結:** ADR-093 の、macOS の決定と弱さの一覧は、そのまま。「Linux では、信頼できないコードの Rust の解析ができない」だけが覆る(Landlock のない Linux では、今も止まる)。解析する構成が、実行する OS 向けになる点(`cfg(target_os)`)は、macOS と同じ(課題 #146)。
