### 6.9 module の path に major を持つ Go のパッケージ(`/v*`。ADR-110)

背景: Go の module は、major が 2 以上になると、path に `/vN` を持つ(`github.com/google/go-github/v93`、`github.com/jackc/pgx/v5`)。シンボル(`(*github.com/google/go-github/v93/github.Client).Do`)も、組み込みのモデルの行の鍵も、この path をそのまま含む。そのため、1 つの行は 1 つの major にしか当たらない。major ごとにファイルや行を写すと、行が major の数だけ増え、`.dead-allow` と zoo のセルも同じだけ増える。

**書式**: シンボル(`symbol`)、`kind: field` の `field`、`kind: param_type` の `type`、`verified_versions` の `package` で、module path の major の要素を `/v*` と書く。

```yaml
verified_versions:
  - package: github.com/google/go-github/v*
    versions: ">=93.0.0 <94.0.0"      # 差分テストを通した範囲。いまは v93 だけ
sinks:
  - symbol: (*github.com/google/go-github/v*/github.RepositoriesService).GetContents
```

- **`/v*` が当たるのは、path の 1 要素で、`/v` と数字(2 以上、先頭が 0 でない)だけ**: `/v2`、`/v93` には当たる。`/vendor`、`/v2x`、`/v`、`/v1`、`/v0` と、major のない path には当たらない(`v0`、`v1` は、Go の module の path に現れない)。任意の glob ではない。
- 1 つの文字列に、1 回だけ書ける。module の path のあとに、要素の全体として書く(あとに `/`、`.`、`)` が来る、または文字列の終わり: `x/v*/pkg.F`、`(*x/v*/pkg.T).M`、`x/v*.F`)。それ以外(`v*x`、`v*2`、2 回、path の先頭)は、読み込みで失敗する(検証)。Go のモデルだけ。Dart と Rust には、この書式はない。
- **照合のしくみ**: 索引は、シンボルの文字列そのものが鍵なので、鍵は変えない。解析の読み込みのとき(`loadGo`)、プログラムが持つ package の import path(依存を含む)から、`/v*` の行ごとに、module の path が合う major を集め、major ごとに行の写しを、具体的なシンボルの鍵で足す(`match: implementations` の展開と同じ形。`Rules.ExpandMajors`)。モデルのファイルの行は変わらない。プログラムがその module を使っていなければ、写しはできず、行は何にも当たらない(今までの、当たらない行と同じ)。`rivus models coverage` は、解析の結果から出すので、そのまま動く。
- **道具の側**: `bench/model-diff` は、`-pkg` に `/v*` を書けて、`-version`(`v93.0.0`)の major で具体的な package に直す(`-version` がなければ失敗)。`TestBuiltinRowsAreHitByTheZoo` は、`/v*` の行を、どれかの major のシンボルへの呼び出しが当てれば、当たったとみなす。`.dead-allow` は、行と同じ文字列(`/v*` つき)で書く。`verified_versions` の `package` の `/v*` は、全部の major を指す記録で、`versions` は、実際に差分テストを通した範囲(そのときに確認した major だけ)にする。
- 接尾辞のない v0 / v1 の path には `/v*` は当たらない。v1 と v2 以降の両方を対象にするパッケージは、接尾辞のない行と `/v*` の行の両方が要る(一覧の 14 行で該当するものがあれば、そのパッケージの回で扱う)。
- **確認する major**: 上流が今もサポートしている major が対象(既定は最新。複数をサポートしていれば、それぞれで差分テストを通し、`verified_versions` に並べ、その major にしかない API の行も書く)。「サポートしている」は上流の公開の事実(サポートの方針、README、リリースのブランチ、直近のリリース)で決め、根拠を 1 行で書く。分からないときは最新だけにする。サポートを終えた major は対象にしない(行は当たり、6.10 の注記が出る)。
- **しないこと(この節では)**: 解析した対象の major が `verified_versions` の範囲にないとき、`summary` に注記を出す(6.7 の 3。別の PR)。`body_replaced_packages` の `package` に `/v*` を書くこと。`-ir` で渡された Go の IR への展開(展開は、Go の解析の読み込みの中だけ)。

