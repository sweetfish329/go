# 棋譜ストア & 添削ツール (Kifu Store & Reviewer)

オンラインで囲碁の棋譜（SGFファイル）を保存・閲覧し、任意の局面に対して指導コメントや変化図を挿入・再生できる Web アプリケーションです。

## 技術スタック

- **Backend**: Go (1.26+)
  - 標準ライブラリ `net/http` を使用した REST API。
  - SGFファイルをパースし、対局者名や対局結果などのメタデータを自動抽出。
- **Frontend**: Svelte (v4) + Vite + Materialize CSS (マテリアルデザイン)
  - SVGを使用した対話的かつ高精細な 19路碁盤を描画。
  - 簡易囲碁エンジン（呼吸点計算、アゲハマ・死石判定、自殺手禁止）を内蔵し、本手順からの分岐（変化図）の動的追加・再生に対応。
- **Database**: PostgreSQL 15
- **Container**: Podman (Podman Compose)

## ディレクトリ構造

```
kifu/
├── backend/            # Go 1.26+ バックエンド
│   ├── cmd/server      # エントリーポイント (main.go)
│   ├── internal/       # コアロジック (SGFパース、DB、ハンドラー、モデル、リポジトリ)
│   └── Containerfile   # バックエンド用コンテナ定義
├── frontend/           # Svelte フロントエンド
│   ├── src/            # Svelteソースコード (App, 碁盤, リスト, 詳細, 囲碁エンジン)
│   └── Containerfile   # フロントエンド用コンテナ定義
├── docker-compose.yaml # コンテナオーケストレーション定義
└── README.md           # 本ドキュメント
```

## クイックスタート (起動手順)

Podman または Docker がインストールされている環境で、`kifu/` ディレクトリ内で以下のコマンドを実行してコンテナを起動します。

```bash
# コンテナのビルドと起動
podman-compose up --build

# または docker を使用する場合
docker-compose up --build
```

起動が成功すると、以下のポートで各サービスにアクセスできます。

- **フロントエンド (Svelte App)**: [http://localhost:5173](http://localhost:5173)
- **バックエンド API**: [http://localhost:8080](http://localhost:8080)
- **データベース (PostgreSQL)**: `localhost:5432`

---

## 動作テスト用 SGF サンプル

起動後、フロントエンド画面の「新規登録」ボタンを押し、以下の SGF テキストを直接貼り付けるか、`.sgf` ファイルとして保存してアップロードしてください。

```sgf
(;GM[1]FF[4]SZ[19]KM[6.5]PB[本因坊秀策]BR[初段]PW[本因坊秀和]WR[二段]RE[B+R]DT[1851-07-04];B[qd];W[dc];B[pq];W[oc];B[cp];W[po];B[qo];W[qn];B[ro];W[pp];B[qp];W[oq];B[op];W[pn];B[np];W[qq];B[pr];W[qr];B[or])
```

### 添削のしかた
1. 登録した棋譜を開きます。
2. 画面右下の「通常再生」から「**添削モード**」に切り替えます。
3. 碁盤上の空いている交点（例えば `19手目` の後に別の手を打つなど）をクリックすると、**新しい変化図**が自動的に作成されます。
4. 添削者名と指導コメントを入力し、「**添削を保存**」をクリックします。
5. 保存後、「変化図に切り替え」ボタンが表示され、元の手順と指導手順をシームレスに行き来できるようになります。
