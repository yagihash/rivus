# #110 取得の段階のネットワークを絞る(宛先の許可リストと、解決したアドレスの検査)

- 状態: **open**(後回しのハードニング。ADR-093)
- 種類: サイレントな見逃しではない(隔離の漏れ)
- 言語: Rust
- ROADMAP の節: 0
- 守るもの: -
- 補足(ROADMAP の節): 0(Homebrew のインストールの作業の続き)
- 補足(守るもの): まだない(直すときにテストを足す)

## 今の状態

取得のサンドボックスは、どこへでも出られる(Docker の既定の bridge と同じ)。許した取得元のリダイレクトとサブモジュールで、別の宛先へ向けられる(`TestIsolationFetchFollowsWhatTheAllowedRepositorySays` は、Docker 版を消した。サンドボックス版はまだない)。

## 終わりの条件

`DESIGN-fetch-network.md` の案を、コンテナなしで実装する: 取得のサンドボックスは、ローカルのプロキシのポートだけに出られ、プロキシ(rivus の Go のプロセスの中)が、許可リストと、解決したアドレス(ループバック、プライベート、リンクローカルなど)の検査と、検査したアドレスへの接続を行う。リダイレクトとサブモジュールの要求が「出ない」テスト(反転)。

## 経緯

- 2026-11-22: macOS のサンドボックスへの切り替え(ADR-093)で、ユーザーが「切り替えを先に、ハードニングは後」と決めた分として、課題に上げた。

## 追記(2026-11-26。今のコード)

`ProxyPort`(`internal/sandbox/sandbox.go`。Seatbelt と Landlock の実装があり、テストは `landlock_linux_test.go`)は、`sandbox` パッケージにあるが、取得の段階(`cmd/rivus/isolate_sandbox_darwin.go` と `isolate_sandbox_linux.go` の `fetchSpec`)は `Network = true` で動かしていて、`ProxyPort` を使っていない(`cmd/` に `ProxyPort` の使用はない)。宛先の絞り込みに移すときの前提。
