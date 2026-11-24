# `-no-deps` を使ったことが `summary` に出ない。`-no-deps` と `body_replaced_packages` の、過大近似の向きの確認

- 状態: **open**
- 種類: 道具・出力(uncertainty が足りない)
- 言語: 全
- ROADMAP の節: 2
- 守るもの: -
- 題: `summary` に `-no-deps` の使用と、読まなかった依存の関数の数がない

[ADR-118](../adr/118-scale-mechanisms-three-conditions.md) の表の行。足りない条件: (i) が未確認(`-no-deps`、`body_replaced_packages`)、(iii) が一部(`-no-deps`)。

## 今、出力はどうなるか

- `-no-deps`: 依存への呼び出しごとに、不確実性 `unmodeled_library_call`(モデルがあれば、その行)。`-no-deps` を使ったことは、`summary` にも `stats` にも出ない(`internal/report` に該当するフィールドがない)。
- `body_replaced_packages`、`-body-replaced`: `summary.body_replaced_packages`(理由つき)、`body_replaced_ignored`。モデルのない呼び出しは `unmodeled_library_call`。

## すること

1. `summary` に、`-no-deps` を使ったことと、読まなかった依存の関数の数を出す。小さい変更。
2. モデルのない依存への呼び出しと、モデルの行が本体より少ない場合が、流れを落とす側か足す側かを確認する(コードと、差分テストの結果で)。落とす側なら、別の課題にする。

## 守るもの

1 は、`summary` のフィールドのテスト。
