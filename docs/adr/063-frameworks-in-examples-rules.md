# ADR-063: 組み込みの範囲 — 特定のフレームワークは、組み込みでなく `examples/rules` に置く(採用: ユーザーが承認した、2026-11-21。ADR-044 の続き)

- 状態: 一部覆った(→ADR-101)
- 関連: README, examples/rules
- 状態の詳細: 2026-11-22 の追記で、線引きは固定でなくなった(組み込みに入れるかは、その都度判断。保留は決着); 覆した先・注意: 044 に逆リンク。ADR-101 が、MODEL-QUEUE の範囲で上書き
- 要旨: 標準 + 代わりのないパッケージ + SQL ドライバ

**背景:** ADR-044 は「めちゃくちゃ有名なサードパーティ」を組み込みに入れる、と決め、目安は暫定だった。Go の gin は、同じ理由(半数に満たない、ベンチの都合)で `examples/rules` に戻した。Rust の actix-web は、組み込みの `rust-web.yaml` に残っていて、基準が揃っていなかった。

**決めたこと(ユーザーの線引き):**
1. **組み込み = 標準ライブラリ + 代わりのないパッケージ(Rust の `serde`、`tonic`、`tokio`、`futures`、`bytes`、`libc`、エコシステムの基礎の `http` / `url` / `serde_json` / `serde_urlencoded` / `urlencoding`、`log` / `tracing`、Dart の公式の `http`)+ SQL のドライバ(あらゆるプロダクトで必要: `sqlx`、`rusqlite`、Dart の `sqflite` / `sqflite_common`)**。
2. **特定のフレームワークのモデルは、`examples/rules/`(書き方の見本。利用者が自分のルールで足す)**。
3. **これは、既定の検出が減る変更で、ユーザーが承認した。** 第 1 弾は actix-web(`actix_web::HttpRequest`、`&HttpRequest`、`web::Query` / `Path` / `Json` / `Form` / `Bytes` の source と、それらの getter の propagator 13 個)を、`rust-web.yaml` から `examples/rules/rust-actix-web.yaml` に移した。`rust-web.yaml` に残るのは、axum、rocket、tide、rouille、warp、hyper の source(下の「保留」)と、`http`、`url`、`serde_json`、`serde_urlencoded`、`urlencoding` の propagator。
4. **利用者に伝える:** 組み込みでなくなったフレームワーク(`cmd/rivus/notbuiltin.go` の表。いまは actix-web)を、プロジェクトのコード(依存でない関数)が呼んでいる、またはその型の引数を取っているのに、ルール(組み込み、`.rivus/rules`、`-rules`)のどこにも `actix_web::` の名前がないとき、`analyze` が stderr に 1 行出す(`-rules examples/rules/rust-actix-web.yaml` で入口が見える)。**結果(finding、`stats`)は変えない**。README に「組み込みの範囲」の節と、移したフレームワークの一覧を足した。
5. **ベンチとテスト:** parseable の `rules.yaml` は、もともと actix の source を持っていた。足りなかった `&HttpRequest` と `web::Form` の source、getter の propagator 13 個を足した(`examples/rules/rust-actix-web.yaml` と同じ内容)。rust-curated(`curated::parseable::web::*` の自前の型)と、Rust のテストは、組み込みの actix のモデルを使っていなかったので、変更なし。

**保留(移さず、ユーザーの判断を待つ。基準は「特定のフレームワーク → 移す」):**
- Rust の `axum`、`rocket`、`tide`、`rouille`、`warp`(`rust-web.yaml` の source)。`hyper` は HTTP の基礎(`http` と同じ側)で残す案。
- Dart の `shelf`(`dart-packages.yaml`。公式のパッケージ(dart-lang)で、サーバー側の標準的な土台なので、残す案もある)。
- `reqwest`、`ureq`(HTTP クライアント。代わりがあるが、sink だけで、クライアントは基礎)は残す案。

**測定:** `compare.sh <base> <この変更>`: parseable、rust-curated、dart-curated、dart-pub で、finding の追加・削除 0(結果はコミットに記す)。

- **追記(ユーザーの判断、2026-11-22。保留の決着):** 「組み込むかどうかは、その都度判断する。ただし、組み込みのモデルも、ある程度充実させる方向でいく。特定のフレームワークを、組み込みに移してもよい」。つまり、決めたこと 1〜2 の線引き(特定のフレームワークは `examples/rules/`)は、**固定の基準ではなくなった**。組み込みに入れるかは、フレームワークごとに、その都度、相談役が判断する(基準の目安: Dart の基盤 = shelf、dart_frog、Flutter、Firebase、dio / http など。3.4 の 5 番)。決めたこと 3〜4(actix-web を移したこと、`notbuiltin.go` の 1 行の知らせ)は、そのまま。保留の項目(axum、rocket、tide、rouille、warp、hyper、shelf、reqwest、ureq)は、その都度の判断の対象に戻る。組み込みに入れるときの条件は、変わらない: 行は一致しなければならない(MODELING 5 節)、流れのモデルは測定で必要が示されたものだけ(ADR-092 を含む)、肯定と否定のフィクスチャ。ADR-077(Google Cloud)の「基準ではなく、例外」という書き方は、この追記で、「基準が固定でなくなった」ので、読み替える: Google Cloud は、その都度の判断で入れた最初のもの。

