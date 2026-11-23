# ADR-058: Rust で、`Reachable` が、再 export されたトレイトの dyn 呼び出しと、環境の `alloc` のクロージャをたどる（ADR-051 の続き）

- 状態: 有効
- 関連: -
- 状態の詳細: 051 の続き

**背景:** Rust の parseable（actix-web）で、finding 159 件の 100 % が `not_from_entry` だった。`Reachable` が届く関数は 8,557 のうち 101。仮説は「`Route::to(関数アイテム)` の関数アイテムがたどられない」だった。**IR を見ると、そうではなかった**: 関数アイテムは、`make_closure`（`Args` なし）として、そのまま `Route::to` の引数に出ており、`KFunc` としてたどられる（`convert` を経る関数ポインタは、`Reachable` も graph の `refs` もたどっていなかったが、parseable の止まる原因ではない）。止まっていたのは、そこに着くまでの鎖で、原因は 2 つ。
1. **dyn 呼び出しの候補が空**（主因）。`main` の `Box<dyn ParseableServer>` の `server.init()` は、callee のシンボルが `parseable::ParseableServer::init`（バイナリ側から見た、クレートの根での再 export `pub use`）、impl は `<Server as parseable::handlers::http::modal::ParseableServer>::init`（定義された所の名前）で、`same_trait`（`extractor/rust/src/lower.rs`）が全体のパスで比べていたので、候補が 0 になった。`init` → `start` → `HttpServer::new(move || …)` の中のルートの登録は、すべて、届かない。
2. **キャプチャのあるクロージャ**（`HttpServer::new(move || App::new()…)`）は、Rust では、`make_closure` でなく、環境の `alloc`（型 `{closure@file:l:c: l:c}`）で作られる。ADR-051 がたどる `make_closure` ではないので、`Reachable` も graph の `refs` も、たどらなかった。

**決めたこと:**
- `same_trait` は、**同じクレートで、同じ名前で、片方がクレートの根のパス（`crate::Trait`）のとき**も、同じトレイトとする（再 export）。別のクレートの同名、同じクレートで根でない同名（2 つのモジュール）は、別のまま。dyn 呼び出しの候補が、増える（精度の方向: 欠けていたエッジが付く）。
- `sg.Program` は、環境の `alloc` の型の位置（`file:line:col`）で、クロージャの関数（その位置にある、1 つだけの関数）を引き（`EnvClosures`。C-7 の Rust の節の仕組みと同じ「型の文字列でクロージャの関数を引く」。別の仕組みは作らない）、`Reachable` がたどる。graph の `refs` は、同じものを `closure` 種別の参照として書く。
- `sg.Value.FuncValue` / `FuncRef`: 関数値が、`copy` や、関数型から関数型への `convert`（関数アイテム → 関数ポインタ）を経ても、同じ関数と見る。`Reachable` と graph の `refs` が、同じ関数を使う（片方だけ直すと、graph の `--from-entry` の説明「`Reachable` と同じ規則」が嘘になる）。
- `refs` の索引の内容が変わる（行が増える）が、スキーマは変えない（`user_version` は上げない）。DB は使い捨て（旧い DB は、`rivus_version` の違いで古いと分かる）。

**`Reachable` の使い道（grep で確認）:** `internal/taint/engine.go` の `e.reach` だけ。finding の `reachability` の値と、代表の経路の選び方の最初の基準。**絞り込みには使われない**（finding は増減しない）。

**検証（parseable、同じ rules）:** 届く関数 101 → 114（`Reachable` の変更だけ）→ 3,867（抽出器の変更と合わせて）。finding の集合: 追加 0、削除 0（159 件）、fingerprint 0 件、代表の経路 0 件が変化。`reachability`: `not_from_entry` 159 → 7（`from_entry` 152）。抽出器だけだと 3 件、`Reachable` だけだと 0 件: **両方が必要**。残る 7 件は、`tonic` の gRPC サービス（`AirServiceImpl` のメソッドを、依存の `tonic` が呼ぶ）で、読まなかった依存のコードの側のエッジなので、ADR-052 の説明のとおり。新しい固定: `cmd/rivus/rust_reach_test.go`（`extractor/rust/test/reach`: 再 export したトレイトの dyn 呼び出し、関数アイテム、関数ポインタ、キャプチャのあるクロージャが、`Reachable` と graph の `--from-entry` の両方で届き、参照のない関数は届かない）。

