# #235 Rust: 型パラメータ越しの呼び(`F: Trait` の `f.method()`)の candidates が、その trait の全 impl になり、登録されない impl も届くと読む(過大近似)

- 状態: **open**
- 優先度: 低〜中(見逃しより安全な側)
- 種類: 精度(過大近似。見逃しではない)
- 言語: Rust(dyn 呼びと `Fn::call` の候補も同じ規則)
- ROADMAP の節: B / C
- 守るもの: zoo-rust-app の `orphan`(`bench/zoo-rust-app/app/src/server.rs`)。期待は今 `// not_from_entry` で、過大近似の姿(`// from_entry`)に直す予定。それは、期待を過大近似として書く PR で扱う
- 進める完了条件: -

## 何が起きるか(事実)

型パラメータ越しの呼び(`F: Trait` の `f.method()`)は、extractor が trait の同一性を「クレート名 + 定義パス」で取れた場合、candidates がその trait の全 impl になる。その impl の中に、誰にも登録されない(値として作られない)ものがあっても、candidates に入り、そこから届くと読む。

2026-11-27 の実測(`bench/zoo-rust-app`、PR #617、#618。IR は `~/tmp/rivus-developer/traitpath-out/{base,new}.jsonl`):

- `ServiceConfig::service<F: HttpServiceFactory>` の `factory.register(..)` は、`by_trait` の呼び(型パラメータ越し)。
- trait の同一性を「クレート名 + 定義パス」で取る変更の前(`base`)は、candidates が 4 個(zoo_web のタプル impl だけ)。再 export で trait の同一性が取れず、ハンドラが全部 `not_from_entry` だった(課題 #234)。
- 変更の後(`new`)は、candidates がその trait の全 impl(zoo_web のタプル impl 4 個と、zoo_app の impl 全部で 11 個)。
- 誰にも登録されない impl `orphan` も、その candidates に入り、`from_entry` になる。

## 同じ規則の他の場所(事実。コードを読んで確かめた)

- dyn 呼び(`CalleeDynamic` の candidates)。
- `Fn::call` の候補。`internal/sg/build.go` の `ValueCands`(読んだのは `SetCellCandidates` の周りだけ。候補を作る側は未確認)。
- `internal/valuecalls/narrow.go`: entry がある場合(`f.Entry && !f.Init`)、外から呼ばれるマーク(`ExternalEntry`)が無く、extractor が呼び出し元のマークを付けている(`prog.CallerMarks`)とき、`n.closed = entry && !external && prog.CallerMarks` で閉じた世界にする。閉じた世界で絞るのは「引数の呼び出し元がすべて読んだコードの中」という点で、candidates の中身を「値として作られた型」に絞る働きではない(`closed_world_entry` の根拠は `internal/taint/calls.go:1375`)。
- `CallOut` の名前は、`internal` 配下の grep で見つからなかった。**未確認**(別の名前か、依頼の側の記憶違いか)。

## 影響

- 到達性のラベルでは、「届くかもしれない」と言う側に外れる。見逃しより安全。
- finding の数や uncertainty では、登録されない impl の中の sink に、偽の流れが届く恐れがある。**実対象で起きるかは未測定**。
- rustypaste も同じ形(`ServiceConfig::service<F>`)なので、登録されない impl があれば同じ過大近似になるはず。**未測定**(PR `feat/rust-trait-identity-by-def-path`(#620、MERGED 2026-11-26)の測定は #236 にあるが、finding の前後は書いていない)。

## 進め方(案。原因の断定ではない)

候補を「値として作られた型の impl」に絞る(RTA 風。Rapid Type Analysis)。型の値が作られる命令(alloc、構築子、`Default` など)から、その型の集合を作り、candidates をその型の impl に絞る。

入れる場所は 3 つ考えられ、設計が要る。architect への相談の候補。

- extractor の `candidates` の生成
- `link_across_crates`
- Go 側の `internal/valuecalls`

この課題は、#234(Rust の実対象で到達性のラベルが信用できない)の解消のあとに残る、精度の改善。

## 関連

- #234(到達性のラベル。この課題はその解消のあとに残る)
- #209(Dart: 登録表から引いて呼ぶ関数がグラフのエッジにならない。別の言語の、似た形の候補の課題)
- ADR-058(再 export された trait の dyn 呼び。同一性が取れず候補が空だった)
- ADR-055
