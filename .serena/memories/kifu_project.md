# Serena Memory: 囲碁 棋譜ストア & 添削ツール (kifu/)

## プロジェクト概要
`kifu/` ディレクトリ配下で開発された、オンライン囲碁棋譜ストアおよび対話型の棋譜添削・指導ツール。
ユーザーがアップロードした SGF (Smart Game Format) 形式の棋譜をデータベースに格納・表示し、任意の局面から本手順と分岐した「変化図」を動的に作成・保存できる機能を提供する。

## 技術スタック
- **Backend**: Go (1.26+)
  - 標準ライブラリの `net/http` を使用してシンプルな REST API を構築。
  - SGF parser を自作し、棋譜から対局者・コミ・置き石・日付・結果などのメタデータを抽出。
- **Frontend**: Svelte (v4) + Vite + Materialize CSS (CDN)
  - SVGを活用し、レスポンシブで高精細な碁盤（19路・13路・9路対応）を自作。
  - 呼吸点計算、アゲハマ・死石判定、自殺手禁止といった基本的な囲碁ルールを処理するクライアントサイドエンジン (`goEngine.js`) を内蔵。
  - 変化図の作成・再生、コメント追加に対応した SGF 再生・編集エンジン (`sgfPlayer.js`) を実装。
- **Database**: PostgreSQL 15
  - 棋譜のメタデータ、SGFデータ本体、変化図データ（添削内容）を管理。
- **Container**: Podman (Podman Compose)
  - コンテナランタイムとして Podman を採用し、`docker-compose` 互換で複数サービスを構成。

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

