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
  * **パフォーマンス最適化**: APIレスポンスなどの変更をせず再代入のみを行う大きなデータには、Proxy of オーバーヘッドを避けるために `$state.raw(...)` を使用してください。
  * 状態から計算される値の同期には、必ず `$derived()` を優先して使用し、`$effect()` を状態の同期目的で絶対に使用しないでください。
  * `$effect()` は、外部連携やDOMの直接操作などの「脱出ハッチ」として扱い、多用を避けてください。
  * **コンポーネント外（JS/TSファイル）でのルーン使用**:
    通常の `.js` / `.ts` ファイル内で `$state()` などのルーンを使用する場合、**ファイルの拡張子を必ず `.svelte.js` または `.svelte.ts` にする必要があります**。通常の拡張子のままだと Vite/Svelte コンパイラを通らずにそのままブラウザに配信され、ランタイムで `ReferenceError: $state is not defined`（画面が真っ白になる）を引き起こします。
    また、`.svelte.ts` / `.svelte.js` ファイルをインポートする際は、インポート側（`.svelte` コンポーネント等）で `import { auth } from './lib/auth.svelte';` のように **`.svelte` 拡張子を末尾に明示して解決する**必要があります。
* **プロップス ($props)**
  * 引数は `let { propA, propB } = $props();` のようにオブジェクトの分割代入形式で受け取る必要があります。
  * コンポーネント内の子要素は `children` プロップとして渡され、テンプレート内では `{@render children()}` でレンダリングします。
* **ビルトインリアクティブクラス**
  * `Set`, `Map`, `Date` などをリアクティブにする場合は、`svelte/reactivity` パッケージのビルトインクラスを使用してください。

### ⑤ 複数プロバイダOAuth認証 & ユーザー名変更機能
* **DB設計**:
  同一ユーザーが複数の外部プロバイダ（Google, LINE, Meta）と連携できるよう、`user_oauths` テーブルを追加しました。これに伴い、OAuth専用アカウントに対応するため `users.password_hash` の `NOT NULL` 制約が解除されています。
* **ユーザー名変更**:
  `PUT /api/auth/username` API によりニックネームを変更可能です。フロントエンド側ではヘッダーのユーザー名をクリックすると `UsernameDialog.svelte` モーダルが立ち上がります。

### ⑥ パッケージマネージャの Bun 移行
* 本プロジェクトは `npm`/`npx` を完全に廃止し、**`bun` / `bunx` に移行**しています。
* ローカルでのパッケージインストールやリンター実行（`lefthook.yml`）には `bun install` や `bunx` を使用してください。ロックファイルは `bun.lock` を追跡します。

### ⑦ ポート競合と接続障害のトラブルシューティング
* ポート `8080` は Windows 環境において `NVIDIA Broadcast` やローカルの別 Express サーバーと競合し、ブラウザでアクセスした際に `Cannot GET /` (Expressの404エラー) になる障害が発生しました。
* この競合を回避するため、ホスト側のマッピングポートは **`8822`** に変更されています（コンテナ内のポートは `8080` のまま）。アクセスする際は **`http://localhost:8822`** を使用してください。

### ⑧ SGFのパース＆シリアライズライブラリの統一
* 既存の自作 SGF パーサーおよびシリアライザは、変化図（分岐）やコメントのエスケープ処理に一部課題があったため、全面的に **`@sabaki/sgf`** ライブラリに移行しました。
* `@types/sabaki__sgf` も導入されており、TypeScript の恩恵を安全に受けられます。
* `sgfPlayer.ts` の `parseSgf` および `stringifySgf` は内部で `@sabaki/sgf` の `parse` / `stringify` を利用したラッパーとして再実装されているため、他の UI コンポーネントへの変更波及を最小限に抑えています。
