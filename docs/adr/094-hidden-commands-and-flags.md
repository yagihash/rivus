# ADR-094: 普段使わないコマンドとフラグを分ける。`extract` は `debug extract` に移し、`-h` は普段使うフラグだけを出す(ユーザーの依頼、2026-11-22)

- 状態: 有効
- 関連: -

**決めたこと:**
1. `rivus extract` は `rivus debug extract` に移した。`rivus help` の一覧から `extract` を外し、`debug`(調査と開発のためのコマンド)を足した。`rivus debug` と `rivus debug -h` は、その下のコマンドを出す。動作、フラグ、出力は、今までの `extract` と同じ。古い `rivus extract` は、別名にせず、新しい名前を案内して終了コード 2 で止まる(`-container-*` のフラグの改名と同じ扱い。ADR-093)。
2. `rivus <command> -h` は、普段使うフラグだけを出す。診断と調整のフラグ(プロファイル、`-pass-stats`、解析の上限、サンドボックスの調整など)は、`-help-all` で全部出す(`-h -help-all` でも、`-help-all` だけでも)。通常の `-h` の末尾に、その案内が 1 行出る。フラグは消さず、動作は変えない。`-all` は `extract` が使っているので、`-help-all` にした。
3. 分け方の基準: 利用者が結果の意味を変えるために使うフラグは、普段使う側(`-numeric-taint`、`-deterministic-only`、`-isolation-env`、`-isolation-read` など)。コストや診断のためのフラグは、診断側。
4. 実装: `cmd/rivus/flagsides.go`。フラグの定義の隣で `usual(fs, 名前...)` か `diagnostic(fs, 名前...)` を呼ぶ。フラグが、どちらにも入っていない、または両方に入っていると、そのコマンドは起動時に止まり、`TestEveryFlagHasOneSide` が失敗する。新しいフラグは、足すときに側を決める。

