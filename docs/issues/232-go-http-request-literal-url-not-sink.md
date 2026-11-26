# #232 Go: 構造体リテラルで作った `*http.Request` を `Client.Do` に渡す形で、URL への流れが見つからない

- 状態: **open**
- 優先度: 中
- 種類: サイレントな見逃し
- 言語: Go
- ROADMAP の節: -
- 守るもの: zoo `F/net-http/request-literal`
- 進める完了条件: -

## 何が起きるか(事実)

`net/http` で、`&http.Request{URL: u}` のように構造体リテラルで作った要求を `Client.Do` に渡す形は、source から sink への流れが見つからない。`u` は `url.Parse("http://example.invalid/?q=" + v)` の結果で、`v` は source。

```go
v := mark.Src("...")
u, _ := url.Parse("http://example.invalid/?q=" + v)
c.Do(&http.Request{Method: "GET", URL: u})   // 期待: v から、この Do への SSRF の流れ。実際: 出ない
```

確かめた(2026-11-26): zoo のセル `F/net-http/request-literal` の vuln で、`TestZooTruthGo` と `TestZooGo/alone/net-http/request-literal` が `missing flow: ... [net-http-request-literal-vuln]`(source から `Do` への流れが無い)を出す。`NewRequest("GET", u.String(), nil)` を `Do` に渡す形(`F/dart/dio-geturi`)は見つかる。

## 原因の見当(読んで分かる範囲)

`internal/model/builtin/go-net-http.yaml` の `ssrf` の sink は、`net/http.Get` `Head` `Post` `PostForm`、`(*net/http.Client).Get` `Head` `Post`、`net/http.NewRequest`(引数 1)、`net/http.NewRequestWithContext`(引数 2)。`(*net/http.Client).Do` は sink のエントリに無い(propagator のエントリだけ: 引数 0 から result0)。構造体リテラルの `URL` フィールドも sink のエントリに無い。そのため、`NewRequest` を通らない要求は、どこにも sink として出ない。uncertainty が付くかは未確認。

## 影響

`http.Request` のリテラル(`URL` に `*url.URL` を入れる)は、`NewRequest` を使わずに要求を作る書き方として普通にある。この形の SSRF は、検出されない。

## 修正の案(案であって、試していない)

- (a) `(*net/http.Client).Do` の引数 0(`*http.Request`)の `URL` フィールドの値を、`ssrf` の sink にする(モデルのエントリで、フィールドを指す sink の書き方があればそれを使う。なければ、モデルの書式の拡張が要る)。
- (b) 構造体リテラルの `net/http.Request` の `URL` フィールドへの代入を、`NewRequest` の引数 1 と同じ扱いにする。

## zoo

- 守るセル: `F/net-http/request-literal`(`bench/zoo-go/cells/net_http/request_literal/`)。vuln の source の位置に、サイレントな見逃しのマークがある。
- `F/dart/dio-geturi` は、この見逃しを避けるため `u.String()` を `NewRequest` に渡す形のまま(`@shape` に書いてある)。見つかる形だけを測る。
- 見つかった経緯: #607 の Go のセル `dio-geturi` の作成時。
