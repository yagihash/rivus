# actix-web: sink とセルの確認が、いまのモデルに入っていない

- 状態: **open**
- 種類: サイレントな見逃し
- 言語: Rust
- ROADMAP の節: 6
- 守るもの: -

## 何が見逃されるか(または、何が誤って報告されるか)

次は、`internal/model/builtin/rust-actix-web.yaml` にまだ行がなく、見逃される(サイレント)。

- **リダイレクトの行き先**: `web::Redirect::to` と `Redirect::new` の行き先、`Location` ヘッダ(`HttpResponseBuilder::insert_header` / `append_header`)。open_redirect の sink がない。
- **レスポンスの本文**: `HttpResponse::body` などに入る利用者の値(XSS)。
- **`insert_header`**: 任意のヘッダを取るので、全体を sink にすると誤報になる。ヘッダの名前と値を分けて扱う(`Location` のときだけ sink にする)書き方が、いまの形式にあるかは未確認。
- **ファイルの配信**: `actix-files` の `NamedFile::open`(別の crate)。モデルの一覧に足すかは未決。

次は、行はあるが、zoo のセルで確認できていない。

- **`web::Bytes`**: 実体は `bytes::Bytes` で、extractor は定義した crate の名前で書くので、`actix_web::web::Bytes` の行は何にも一致しない。source にしていない。
- **`web::Payload`、`web::Header<T>`**: extractor だけが作る型で、セルが値を作れない。行を書いていない。
- **`web::Path`**: source の行はあるが、フィールドが非公開で、セルが値を作れない。`Path::into_inner` の行は dead-allow。

## 原因

Rust のパッケージの zoo は、sink を zoo-mark の関数でしか確認できず、スケルトンの `Cargo.toml` が zoo-mark を指せない(テストが書き換えるのはルートの `Cargo.toml` だけ)。また、extractor でしか作れない型は、セルから値を作れない。

## 再現するセル

なし。`@miss` / `@fp` のセルは作っていない。Rust のパッケージの zoo が sink と、作れない型を扱えるようになったときの、改善の候補。
