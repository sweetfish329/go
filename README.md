# Go 囲碁関連ツール統合リポジトリ (sweetfish329/go)

このリポジトリは、囲碁に関する各種開発ツールや Web アプリケーションをまとめて管理するためのプロジェクトです。

---

## 1. プロジェクト一覧

現在、以下のプロジェクトが管理されています。

### 📁 [kifu/](./kifu/) — オンライン棋譜ストア & 添削ツール
Go バックエンドと Svelte フロントエンドで構築された、対話型の棋譜管理・添削 Web アプリケーションです。
- **特徴**:
  - SGFファイルのメタデータ自動抽出・保存。
  - SVGを使用した対話的かつ高精細な碁盤の描画（19路盤・13路盤・9路盤対応）。
  - クライアントサイドの簡易囲碁エンジンによる、呼吸点計算・死石除去・自殺手禁止機能。
  - 添削モードによる、本手順から分岐した「変化図」の入力・再生と、指導コメントの紐づけ保存。
  - スマホやタブレットからの閲覧・操作に対応したレスポンシブデザイン。
- **技術**: Go 1.26+, Svelte (v4), Vite, Materialize CSS, PostgreSQL 15, Podman (Podman Compose)

---

## 2. クイックスタート

Podman Compose（または Docker Compose）を使用し、ワンコマンドでローカル開発・実行環境を構築できます。

```bash
# 1. リポジトリをクローンして移動
git clone https://github.com/sweetfish329/go.git
cd go/kifu

# 2. コンテナのビルドと起動
podman-compose up --build
# もしくは docker の場合: docker-compose up --build
```

起動が成功すると、以下のURLでアクセス可能になります。
- **フロントエンド (Svelte SPA)**: [http://localhost:5173](http://localhost:5173)
- **バックエンド API**: [http://localhost:8080](http://localhost:8080)

詳細は、[kifu/README.md](./kifu/README.md) をご参照ください。
