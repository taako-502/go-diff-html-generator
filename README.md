# go-diff-html-generator

2つのテキスト（またはJSON）を比較して、非エンジニアにも見やすいHTML差分レポートを生成するGo CLIです。

## 特徴

- Goだけで完結（Node.js不要）
- 追加/削除/変更を色分け表示
- 左右2カラムで「変更前」「変更後」を比較
- 変更行は行内差分（どの文字が変わったか）も表示
- `-mode json` でJSONを正規化してから比較（キー順を安定化）

## 使い方

```bash
make diff
```

JSON比較モード:

```bash
make diff-json
```

生成された `diff.html` をブラウザで開くと、差分を視覚的に確認できます。

## オプション

- `-before`: 比較元ファイル（必須）
- `-after`: 比較先ファイル（必須）
- `-out`: 出力先HTML（デフォルト: `diff.html`）
- `-mode`: `text` または `json`（デフォルト: `text`）
- `-title`: レポートタイトル（デフォルト: `テキスト差分レポート`）

## テスト・静的解析

```bash
make check
```

## License

MIT
