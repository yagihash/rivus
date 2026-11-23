# ADR-089: Rust の `write!` / `writeln!`(`Write::write_fmt`)のモデル。レシーバが `arg0` のとき `.content` の置き先を使い、標準のライターは全体に置く

- 状態: 有効
- 関連: -

**背景:** `write!(w, "{}", x)` は `w.write_fmt(format_args!(..))` になる。`std::io::Write::write_fmt` のモデルが無く、Go の `fmt.Fprint` に一致する形が見つからなかった(zoo `dyn/fprint-writer-own` の Rust が `@miss(unmodeled_library_call)`)。`format_args!` は `Arguments::new_v1` / `Argument::new_display`(既存の行: 引数が結果へ)を通り、`write_fmt` の `arg1` に届く。

**決めたこと:** (1) `rust-std.yaml` に `std::io::Write::write_fmt`(`arg1` → `arg0.content`、`calls: [{method: write, with: [arg1]}]`)と `std::fmt::Write::write_fmt`(`write_str`)。標準の `write_fmt` は `write_all` を経て `write` を呼ぶので、`method:`(ADR-080)は `write` を直接指す。(2) `contentOnly`(ADR-082)は、`self_first` の集合(Rust)の `arg0` をレシーバとして扱う。プロジェクトの型のライターでは、データは `.content` に置き、本体(`write`)を適用した先のフィールドにだけ届く(`w.name` のような別のフィールドを読む sink は検出されない)。標準の型(`Vec<u8>`、`String`)は本体がなく、モデルがライターの全体を読むので、全体に置く。(3) `applyMethodRow`: レシーバの型が分かり、それが標準の型(`std::` / `alloc::` / `core::`)なら、プロジェクトの実装は呼ばない(別の型の実装へ流れが漏れた)。

**確かめたこと:** zoo(Rust)の `dyn/fprint-writer-own` の vuln と decoy の本物の流れが見つかる(`@miss` を外した)。decoy の `w.name` の sink は検出されない。一時的にライターを `Vec<u8>`、`String`(`fmt::Write::write_fmt`)にした形でも見つかり、他のセルへの漏れは無い。

**限界:** `fmt::Formatter::write_fmt`(`impl Display` の中の `write!(f, ..)`)は、今までどおり `pure`(行を変えていない)。`impl Display` を通る流れは別の作業。

