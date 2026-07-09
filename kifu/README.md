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

---

## 本番環境での Nginx リバースプロキシ設定

本番環境で Nginx をリバースプロキシとして配置する場合、**OGPの動的生成（クローラー対応）**や**OIDC/OAuth2 認証（GoogleやLINEなど）**を正常に動作させるために、いくつかのプロキシヘッダーの設定が必須となります。

### 必須のプロキシヘッダー

Go バックエンドはリクエストヘッダー（`X-Forwarded-Proto` や `Host`）を読み取って、OGP 画像の絶対 URL や OAuth2 のリダイレクト URI (Redirect URI) を自動構築します。Nginx では必ず以下のヘッダーを転送するように設定してください。

- `proxy_set_header Host $host;`
- `proxy_set_header X-Real-IP $remote_addr;`
- `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`
- `proxy_set_header X-Forwarded-Proto $scheme;`

### 設定例1: シンプルなリバースプロキシ構成（推奨）

すべてのリクエスト（静的アセット、API、HTMLページ）を Go バックエンドに流す最もシンプルな構成です。Go バックエンドがシングルバイナリで静的ファイル（`./dist` 配下）のサーブと OGP の動的挿入（`RootHandler`）を両方行うため、Nginx 側は単純に中継するだけで完璧に動作します。

```nginx
server {
    listen 80;
    server_name kifu.example.com;
    # HTTPSへリダイレクト
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name kifu.example.com;

    ssl_certificate /etc/letsencrypt/live/kifu.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/kifu.example.com/privkey.pem;

    # OGPタグ生成とOAuth2/OIDCコールバックのためにヘッダーをフォワードする
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    location / {
        # Goバックエンドコンテナ（またはプロセス）のポートへプロキシ
        proxy_pass http://kifu-backend:8080;
        
        # WebSockets や大容量ファイルのアップロード（SGF）をサポートする場合の設定
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        client_max_body_size 10m;
    }
}
```

### 設定例2: アセット配信を分離する構成（パフォーマンス最適化）

静的アセット (`/assets/*`, `/favicon.ico` 等) を Nginx から直接配信し、API（`/api/*`）および HTML ページ要求を Go バックエンドに流す構成です。
※HTML要求（`/`, `/u/*`）を Nginx で直接 `index.html` として返してしまうと、**Go バックエンドによる OGP メタタグの動的埋め込みがバイパスされてしまうため、HTML 要求は必ず Go バックエンドへ転送してください**。

```nginx
server {
    listen 443 ssl;
    server_name kifu.example.com;

    ssl_certificate /etc/letsencrypt/live/kifu.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/kifu.example.com/privkey.pem;

    # ビルド済み静的ファイルのディレクトリパス（ボリュームマウント等でNginxからアクセス可能な場合）
    root /app/dist;

    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    # 1. 静的アセットは Nginx で直接配信して高速化
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, no-transform";
        try_files $uri =404;
    }

    location = /favicon.ico {
        log_not_found off;
        access_log off;
    }

    # 2. APIリクエストは Go バックエンドへ
    location /api/ {
        proxy_pass http://kifu-backend:8080;
    }

    # 3. その他（HTMLの要求）は OGP 注入のため Go バックエンドへプロキシする
    location / {
        proxy_pass http://kifu-backend:8080;
    }
}
```

### OGP と OIDC (OAuth2) の設定に関する注意点
- **リダイレクトURI**: Google や LINE などの OAuth2 管理画面でリダイレクトURIを登録する際は、`https://kifu.example.com/api/auth/oauth/callback/{provider}` のように `https` スキーマかつ本番ドメインを指定してください。
- **外部URL設定**: 本番運用の際は、管理画面の「システム設定」などで `external_url` (例: `https://kifu.example.com`) を設定しておくと、リダイレクトURIの解決がより確実になります。
- **OAuthのState用クッキー**: OAuth認証時に一時クッキー（`oauth_state_*`）を使用するため、Nginx で `proxy_cookie_path` や `proxy_cookie_domain` の設定を極端に厳しく制限している場合は、クッキーが正常にブラウザに届くか確認してください（通常はデフォルトのままで問題ありません）。

