# #236 Rust: dyn 呼びと型パラメータ越しの呼びの candidates が、trait の同一性を正しく取った結果、約 9 倍に増える(大きい対象で、メモリの余裕を食う恐れ)。候補の数の上限の案

- 状態: **open**
- 優先度: 低〜中(rustypaste では許容できる範囲。大きい対象は未測定)
- 種類: 性能・精度(過大近似が増える)
- 言語: Rust
- ROADMAP の節: B / C
- 守るもの: -
- 進める完了条件: -

## 何が起きるか(事実)

PR #620(`feat/rust-trait-identity-by-def-path`。extractor が trait の同一性を「クレート名 + 定義パス」で比べる)の測定(2026-11-27)。#620 は未マージ。対象は rustypaste(`bench/deps_bench.py -no-ledger`、`pass_line: rustypaste`、150 秒、8 GiB の footprint)。

- `dynamic_dispatch` の 176 件は、場所が同じで、候補の数が変わった。候補 0 だった 113 件のうち 84 件が、候補ありに変わった。
- **候補の合計は 284 → 2497(約 9 倍)**。
- 大きく増えた呼び出し先:

| 呼び出し先 | 合計(前 → 後) |
| --- | --- |
| `serde::Deserializer::deserialize_identifier` | 78 → 780 |
| `serde::Deserializer::deserialize_struct` | 60 → 600 |
| `rustypaste::tracing::log::Log::enabled` | 0 → 306(51 件) |
| `serde::Deserializer::deserialize_enum` | 18 → 180 |
| `serde::Serializer::serialize_struct` | 36 → 144 |

- `summary.dispatch_candidates` の分布: 前 `{0:113, 1:18, 2-5:17, 6-20:27, 21+:1}` → 後 `{0:29, 1:33, 6-20:85, 21+:29}`。
- IR 全体の dynamic 呼びの候補数: 0 件が 6496 → 5454、21 件以上が 8720 → 9875。

原因(推定。断定ではない): 再 export 越しの `serde::` と `serde_core::` の trait が同じ trait と認識され、`serde_core` 側の実装が候補に入った。`serde::Deserializer` は `serde_core::Deserializer` の再 export なので、同一視そのものは正しい。

## 影響(事実)

同じ測定(rustypaste、#620 の前 → 後):

- taint 段階: 約 6.5〜7.1 秒 → 9.5〜9.6 秒。
- `stats.functions`: 30570 → 31697。
- `summary_elems`: 36632 → 45720(+25%)。
- `reanalyses`: 77020 → 79682。passes: 5 → 4。
- footprint: +0.7〜0.9 GiB。**再測定の結果は後で追記する**。#620 の再測定の結果は #620 の本文に書く。
- uncertainty: `dependency_body_unread` が 5 → 8 と増えただけ(場所は `config.rs` の `serde::de::Error::missing_field` の呼び出し)。`depth_limit` も、finding 内に付く件が出た。

## 懸念

別の対象(大きい Rust の対象)で、同じ増え方がメモリの余裕を食う恐れ。**未測定**。

## 進め方(案。決定ではない)

候補が N 個を超えたら、候補の列挙をやめて narrowing(絞り込み)に任せる(`dispatch_not_narrowed` にする)。N の値は決めていない。

今は実装しない。#620 の再測定の結果で、許容できる範囲と判断された(advisor の判断)。大きい対象で同じ増え方を測ってから、この案を進めるか決める。

## 関連

- #234(到達性のラベル。#620 の測定は、そちらにも事実を足した)
- #235(型パラメータ越しの呼びの candidates の過大近似。こちらは「登録されない impl も届く」形。この課題は、候補の数そのものの増加)
- ADR-058(再 export された trait の dyn 呼び)
- PR #620
