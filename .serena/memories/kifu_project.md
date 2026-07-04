# Serena Memory: 囲碁 棋譜ストア & 添削ツール (kifu/)

## プロジェクト概要
`kifu/` ディレクトリ配下で開発された、オンライン囲碁棋譜ストアおよび対話型の棋譜添削・指導ツール。
ユーザーがアップロードした SGF (Smart Game Format) 形式の棋譜をデータベースに格納・表示し、任意の局面から本手順と分岐した「変化図」を動的に作成・保存できる機能を提供する。

## 技術スタック
- **Backend**: Go (1.26+)
  - 標準ライブラリの `net/http` を使用してシンプルな REST API を構築。
  - SGF parser を自作し、棋譜から対局者・コミ・置き石・日付・結果などのメタデータを抽出。
- **Frontend**: Svelte (v5) + Vite + Materialize CSS (CDN)
  - パッケージマネージャーには `bun` / `bunx` を採用し、`bun.lock` を管理。
  - SVGを活用し、レスポンシブで高精細な碁盤（19路・13路・9路対応）を自作。
  - 呼吸点計算、アゲハマ・死石判定、自殺手禁止といった基本的な囲碁ルールを処理するクライアントサイドエンジン (`goEngine.js`) を内蔵。
  - 変化図の作成・再生、コメント追加に対応した SGF 再生・編集エンジン (`sgfPlayer.js`) を実装。
- **Database**: PostgreSQL 15
  - ユーザー情報（複数OAuthプロバイダ対応）、棋譜のメタデータ、SGFデータ本体、変化図データ（添削内容）を管理。
- **Container**: Podman (Podman Compose)
  - コンテナランタイムとして Podman を採用し、ホスト側ポートは `8822`（競合回避のため）にバインド。


## 開発の学び・重要知見

### 1. Go 1.26 + Podman 環境下におけるビルド・依存解決の順序
- **問題点**:
  一般的な Go の Docker/Containerfile 設計では、ビルドキャッシュ効率化のために `go.mod` / `go.sum` を先に `COPY` してから `go mod download` を行う。しかし、Go 1.26 + alpine 環境において `go mod tidy` を用いて依存パッケージ（`github.com/lib/pq` 等）を自動検出・ビルドに含める場合、ソースコードがコピーされる前に `go mod tidy` を実行すると「依存インポートがない」と判定され、`go.mod` に依存関係が反映されず、最終的な `go build` でコンパイルエラーとなる。
- **解決策**:
  `Containerfile` において、先にソースコード全体を `COPY` してから `go mod tidy && go mod download` を実行する。
  ```dockerfile
  # ソースコードを先にコピーして go mod tidy がインポート文をスキャンできるようにする
  COPY . .
  RUN go mod tidy && go mod download
  RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server
  ```

### 2. 不規則な SGF 日付 (DT) のクレンジング処理
- **問題点**:
  SGF の `DT` (Date) プロパティは、対局アプリケーションや出力環境によってフォーマットが極めて不規則（例: `2026-03-19 02 02 54` や `2026/03/19` など）。これらを PostgreSQL の `DATE` カラムにそのまま挿入しようとすると `invalid input syntax for type date` エラーになる。
- **解決策**:
  Go側のSGFパース処理で、正規表現による文字列クレンジングを施し、PostgreSQL 互換の `YYYY-MM-DD` 形式に整形する。
  - `YYYY-MM-DD` または `YYYY/MM/DD` にマッチした場合は `-` 区切りに統一。
  - `YYYYMMDD` の連続数字は `YYYY-MM-DD` にフォーマット。
  - 年のみ（`YYYY`）の場合は `YYYY-01-01` として補完。
  - いずれにもマッチしない、あるいは空文字の場合は現在日付（`time.Now()`）でフォールバック。
  ```go
  var dateRegex = regexp.MustCompile(`(\d{4})[-/](\d{2})[-/](\d{2})`)
  var simpleDateRegex = regexp.MustCompile(`(\d{4})(\d{2})(\d{2})`)

  func cleanDate(dt string) string {
      dt = strings.TrimSpace(dt)
      if dt == "" {
          return time.Now().Format("2006-01-02")
      }
      if matches := dateRegex.FindStringSubmatch(dt); len(matches) == 4 {
          return fmt.Sprintf("%s-%s-%s", matches[1], matches[2], matches[3])
      }
      if matches := simpleDateRegex.FindStringSubmatch(dt); len(matches) == 4 {
          return fmt.Sprintf("%s-%s-%s", matches[1], matches[2], matches[3])
      }
      var yearRegex = regexp.MustCompile(`^(\d{4})$`)
      if yearRegex.MatchString(dt) {
          return dt + "-01-01"
      }
      return time.Now().Format("2006-01-02")
  }
  ```

### 3. Svelte + Materialize CSS (CDN) でのスタイリング制御
- **知見**:
  Svelte のコンポーネント内 CSS はデフォルトでコンポーネントスコープに限定されるため、CDN 経由で読み込んだ Materialize CSS のグローバルな要素・クラス（`.card-content` や `.card-action` など）に対してスタイリングを上書きする際、セレクタが無視される問題が発生する。
- **解決策**:
  `:global()` 疑似クラス（例: `:global(.card-content) { ... }`）を使用して、コンポーネントスコープをバイパスして意図通りにスタイルを適用する。

### 4. Svelte フロントエンドの TypeScript 移行
- **移行内容**:
  - `src/main.js` から `src/main.ts` への移行、および `index.html` からの読み込み先更新。
  - `goEngine.js`, `sgfPlayer.js` の型定義付き `.ts` ファイルへの移行。
  - Svelte コンポーネント (`App.svelte`, `Board.svelte`, `KifuList.svelte`, `KifuDetail.svelte`) への `lang="ts"` の適用および型安全化。
- **重要知見**:
  - **TypeScript 7.0.1-rc の非互換性**:
    `typescript@7.0.1-rc` は Microsoft による実験的な Go 移植版のプレビューパッケージ (`typescript-go`) であり、従来の JavaScript API (コンパイラ API) をエクスポートしていません。そのため、`svelte-check` や `vite-plugin-svelte` がモジュール読み込み時に `ERR_PACKAGE_PATH_NOT_EXPORTED` エラーでクラッシュします。
    Svelte+Viteのビルド環境では、JS版 TypeScript の最新安定版である `6.0.3` などを採用する必要があります。
  - **Svelte コンポーネント内での null チェック**:
    `strict: true` 設定下では、API等から取得する非同期データ（例: `kifu` などの対局メタデータ）が `null` になる可能性を TypeScript が検出するため、HTML テンプレート側で `{kifu?.prop}` または `{#if kifu}` によるガードを入れる必要があります。

### 5. 複数プロバイダのOAuth認証連携とユーザー名変更機能
- **DB設計とモデルの拡張**:
  同一の論理ユーザーに対して複数のOAuthアカウント（Google, LINE, Meta）をマッピングする `user_oauths` テーブルを追加。OAuth登録時はパスワードが不要になるため `users.password_hash` の NULL 許容化を実行。
- **ニックネーム更新**:
  `PUT /api/auth/username` エンドポイントを実装し、ユーザーのニックネーム変更をサポート。フロントエンドには `UsernameDialog.svelte` を導入し、ヘッダーのユーザー名クリックで開くUIを構成。

### 6. ポート 8080 競合による接続障害の回避
- **問題点**:
  ローカルの Windows 環境において、NVIDIA Broadcast の設定サーバーなど、別の Express バックエンドが `127.0.0.1:8080` を排他リスニングしているケースがある。この場合、Podman 側で `0.0.0.0:8080` へのバインドが成功しても、ブラウザで `localhost:8080` にアクセスした際は NVIDIA Broadcast 側の Express サーバーへ解決され、`Cannot GET /` (Express 404) になりアプリケーションに接続できない状態になる。
- **解決策**:
  ホスト側のポート競合を避けるため、[docker-compose.yaml](file:///D:/project/go/github.com/sweetfish329/go/kifu/docker-compose.yaml) 上のポートマッピングを **`8822:8080`** に変更した。

### 7. Bun へのパッケージマネージャー完全移行
- **変更内容**:
  `npm`/`npx` によるパッケージ管理およびフックを排除し、すべて `bun` / `bunx` へ移行。
  - フロントエンドに `bun.lock` を生成し、`package-lock.json` を削除。
  - `Containerfile` でのフロントエンドビルドに `oven/bun:alpine` と `bun install` / `bun run build` を適用。
  - `lefthook.yml` に定義されている pre-commit / pre-push 時のフックコマンドを `bunx` および `bun run` に変更。

### 8. @sabaki/sgf の全面的導入によるSGF解析の堅牢化
- **導入の背景**:
  自作の SGF 解析（パーサー）およびシリアライザでは、SGF 規格の複雑なエスケープ処理（コメント内の括弧 `]` 等の退避）や、複数分岐（変化図）のツリー表現における処理に限界があった。そのため、オープンソースの囲碁エディタ Sabaki のコアライブラリである `@sabaki/sgf` を全面的に導入した。
- **実装上の工夫**:
  フロントエンド全体の型安全性を担保するために、`@types/sabaki__sgf` も合わせて開発環境に追加した。
  既存コード（SgfPlayerクラスやSvelteコンポーネント）との互換性を崩さないよう、[sgfPlayer.ts](file:///D:/project/go/github.com/sweetfish329/go/kifu/frontend/src/lib/sgfPlayer.ts) の `parseSgf` と `stringifySgf` の内部のみを `@sabaki/sgf` でラップ・再設計することで、他のクラスや画面コードに破壊的変更を与えることなく SGF 解析の堅牢化を実現した。



