# ADR-096: 組み込みモデルの範囲を、標準ライブラリと代わりのないパッケージに、Google Cloud と pub の人気の上位を加えた 4 つにする(ユーザーの決定の整理、2026-11-22。相談役)

- 状態: 一部覆った(→ADR-097)
- 関連: README、ARCHITECTURE 5.5、DESIGN-model-authoring、DART-MODEL-CANDIDATES
- 状態の詳細: ADR-097 が範囲を書き直した(外部のサービスのクライアントを足し、Google Cloud を範囲の 1 つ目にした)。ADR-044、063、077 の線引きは、これらの範囲で読む
- 要旨: 公開の指標だけで選ぶ

**決めたこと:**
1. 組み込みのモデルにするのは、(a) 標準ライブラリ、(b) 代わりのないパッケージ、(c) Google Cloud(Firebase を含む。ADR-077 の例外)、(d) pub の人気の上位。(d) は公開の指標(pub.dev の順位、直近 30 日のダウンロード数、いいね、依存される数)だけで選ぶ。基準と一覧は `docs/DART-MODEL-CANDIDATES.md`。特定の利用者が何を使うかでは選ばない。
2. 進める順は `docs/DART-MODEL-CANDIDATES.md` の 3 節: `http`、Google Cloud、Flutter に依存しないサーバ側と基本のパッケージの上位(`dio`、`shelf` の残り、`shelf_router`、`logging`、`postgres`、`grpc` のクライアント側、`web_socket_channel`、`dart_jsonwebtoken`、`process_run`)、Flutter のクライアント側。純粋な計算のパッケージ(`xml`、`yaml`、`crypto` など)は後回し。
3. 理由: 依存を API の境界のモデルにして、本体を読まないことで、動的な呼び出しの合流と解析のコストを下げる。流れだけのモデルを入れる条件は、ADR-092 のとおり(測定で必要が示されたもの。何も報告せずに切ることはしない)。
4. 置き換えるもの: `docs/DESIGN-model-authoring.md` の「組み込みは標準ライブラリと、代わりのないパッケージまで」という線引き(既存の ADR の本文は書き換えない。ADR-063 の 2026-11-22 の追記、ADR-077 は、この ADR の範囲で読む)。利用者が自分の依存のモデルを作るための定型のタスクは、そのまま進める。
5. 行は一致しなければならない(`docs/MODELING.md` の 5 節)、肯定と否定のフィクスチャを付ける。これまでの条件は変えない。

