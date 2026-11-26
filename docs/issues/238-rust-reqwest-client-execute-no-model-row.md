# #238 Rust: `reqwest::Client::execute` にモデルの行が無く、`Request::new` で作った要求の URL への流れが、uncertainty なしで見逃される

- 状態: **open**
- 優先度: 中
- 種類: サイレントな見逃し
- 言語: Rust
- ROADMAP の節: -
- 守るもの: zoo `F/net-http/request-literal`(Rust。`bench/zoo-rust/cells/src/net_http/request_literal.rs`)
- 進める完了条件: -

## 何が起きるか(事実)

`reqwest::Url::parse` の結果(source を含む URL)で `reqwest::Request::new(Method::GET, u)` を作り、`Client::execute` に渡す形は、source から URL への流れが見つからない。

```rust
let v = m::src("...");
let u = reqwest::Url::parse(&format!("http://example.invalid/?q={v}")).unwrap();
let _ = c.execute(reqwest::Request::new(reqwest::Method::GET, u));   // 期待: v から、この execute への SSRF の流れ。実際: 出ない
```

- `reqwest::Request::new` は、sink ではない。
- `reqwest::Client::execute` に、モデルの行が無い。
- `reqwest::multipart::Form::file`(パスを取って読む)も、モデルの行が無い。
- uncertainty の痕跡が無い。結果は `silent`(サイレントな見逃し)。

Go 側の同じ形(`&http.Request{URL: u}` を `Client.Do` に渡す)は #232 で、別の課題(原因も別: Go の sink の行に `Client.Do` が無い)。

## 再現するセル

`bench/zoo-rust/cells/src/net_http/request_literal.rs`(`F/net-http/request-literal`)の vuln。source の行に `@miss(silent) @ref(#238)`。

## 進め方

モデルの行を足す作業は、領域 E(モデルの拡充)。今は扱わない。この課題は、見逃しを台帳に残すだけ。

## 関連

- #232(Go: 構造体リテラルで作った `*http.Request` を `Client.Do` に渡す形)
