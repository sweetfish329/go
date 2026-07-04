# GEMINI.md - Coder's Guide to sweetfish329/go

このファイルは、本リポジトリ (`sweetfish329/go`) を開発する AI アシスタント (Gemini などの LLM) のための開発補助ガイドです。プロジェクト全体の構造やこれまでに得られた重要な設計上の意思決定、バグ回避のナレッジを記述しています。

---

## 1. プロジェクト構造

リポジトリは囲碁関係の各種ツールを統合して管理することを目的としています。

```
.
├── .serena/               # Serena (長期記憶システム) の設定・メモリ
│   └── memories/          # プロジェクトに関する知見の記録
├── kifu/                  # オンライン棋譜ストア & 添削ツール (第一弾ツール)
│   ├── backend/           # Go 1.26+ バックエンド
│   ├── frontend/          # Svelte + Materialize フロントエンド
│   └── docker-compose.yaml# Podman Compose コンテナ定義
├── GEMINI.md              # AI用開発ガイド (本ドキュメント)
└── README.md              # ユーザー向け総合 README
```

---

## 2. 実装上の重要知見 & トラブルシューティング

AIアシスタントが今後コードの拡張や修正を行う際は、以下の点に十分注意してください。

### ① Containerfile ビルド時の Go 依存解決順序
`kifu/backend/Containerfile` で `go mod tidy` を実行して依存関係（PostgreSQLドライバ `github.com/lib/pq` など）を解決する際、**ソースコード全体がコピーされる前に `go mod tidy` を実行してはいけません。**
Goのモジュールシステムは、ソースコード内の `import` 宣言に基づいて依存モジュールを検出するため、コードが未コピーの状態で実行すると `go.mod` に何も追記されず、最終的な `go build` でインポートエラーになります。

* **正しい定義順序**:
  ```dockerfile
  # 1. ソースコード全体を先にコピー
  COPY . .
  # 2. その後 go mod tidy を実行して依存関係をダウンロード
  RUN go mod tidy && go mod download
  # 3. ビルド
  RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server
  ```

### ② SGF 日付データのパース & DBクレンジング
SGFファイルの対局日 (`DT` プロパティ) は、記録した対局ソフトによってフォーマットが千差万別です (例: `2026-03-19 02 02 54` や `2026/03/19` など)。
データベースの `DATE` カラムへの INSERT エラーを防ぐため、Goのバックエンド側で必ずクレンジングを挟んでください。
* **実装先**: [sgf.go:cleanDate()](file:///D:/project/go/github.com/sweetfish329/go/kifu/backend/internal/sgf/sgf.go)
  正規表現により `YYYY-MM-DD` 部分を抽出し、無効な日付の場合は現在日付（`time.Now()`）でフォールバックさせています。

### ③ フロントエンドのレスポンシブ対応
* 碁盤の描画には SVG を使用しており、縦横比 1:1 を保ちながらスマホの幅に合わせて自動スケーリングします。
* CSSの変更時には、Svelteのスコープにより Materialize CSS のカードやボタンといった外部クラスへのスタイル適用が無視されないよう、`:global(.card-content)` や `:global(.card-action)` のように `:global()` 擬似クラスを使用してください。

### ④ Svelte 開発時の重要参考情報 (Svelte 5対応)
Svelteに関する開発を行う際は、Svelte公式のLLM用ドキュメント [Svelte LLMs](https://svelte.dev/llms.txt) を必ず参照してください。

#### 主なベストプラクティスと制約（Svelte 5 / ルーンモード）
* **リアクティビティ (Runes)**
  * 状態の宣言には `$state()` を使用します。
  * **パフォーマンス最適化**: APIレスポンスなどの変更をせず再代入のみを行う大きなデータには、Proxyのオーバーヘッドを避けるために `$state.raw(...)` を使用してください。
  * 状態から計算される値の同期には、必ず `$derived()` を優先して使用し、`$effect()` を状態の同期目的で絶対に使用しないでください。
  * `$effect()` は、外部連携やDOMの直接操作などの「脱出ハッチ」として扱い、多用を避けてください。
* **プロップス ($props)**
  * 引数は `let { propA, propB } = $props();` のようにオブジェクトの分割代入形式で受け取る必要があります。
  * コンポーネント内の子要素は `children` プロップとして渡され、テンプレート内では `{@render children()}` でレンダリングします。
* **ビルトインリアクティブクラス**
  * `Set`, `Map`, `Date` などをリアクティブにする場合は、`svelte/reactivity` パッケージのビルトインクラスを使用してください。
