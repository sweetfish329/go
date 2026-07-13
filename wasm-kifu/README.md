# wasm-kifu

Go WASM による棋譜記録ライブラリ。スマホブラウザ (Android Chrome / iPhone Safari) で動作し、クライアントサイドで棋譜の記録と SGF エクスポートが完結します。

## 機能

- 🎯 **棋譜記録**: タップで着手を入力し、手番管理付きの正確な SGF を生成
- 📷 **画像認識**: カメラで碁盤を撮影し、石の位置を自動検出 (OpenCV.js)
- 📤 **SGF エクスポート**: 記録した棋譜を SGF 形式でエクスポート・ダウンロード
- ↩ **待った / パス / 投了**: 基本的な対局操作をサポート
- 📱 **モバイル対応**: スマホブラウザに最適化されたタッチ操作

## 技術スタック

| レイヤー | 技術 | 役割 |
|----------|------|------|
| ゲームロジック | Go 1.26+ → WASM | 着手・合法性判定・SGF 生成 |
| SGF 処理 | [rooklift/sgf](https://github.com/rooklift/sgf) | SGF パース / シリアライズ |
| 画像認識 | [OpenCV.js](https://docs.opencv.org/4.x/opencv.js) | 碁盤検出・石分類 |
| JS ブリッジ | `go:wasmexport` + ES Modules | Go ↔ JS 連携 |

## ディレクトリ構成

```
wasm-kifu/
├── cmd/wasm/             # WASM エントリーポイント
│   └── main.go
├── pkg/
│   ├── engine/           # ゲームエンジン (rooklift/sgf ラッパー)
│   │   ├── engine.go
│   │   └── engine_test.go
│   └── vision/           # 盤面差分解析
│       ├── analyzer.go
│       └── analyzer_test.go
├── web/                  # デモページ & JS ライブラリ
│   ├── index.html
│   ├── css/demo.css
│   ├── js/
│   │   ├── wasm-bridge.js    # WASM ブリッジ API
│   │   ├── board-detector.js # OpenCV.js 画像認識
│   │   └── demo.js           # デモページロジック
│   └── wasm/             # (ビルド生成物)
├── go.mod
├── Makefile
└── README.md
```

## ビルド & 実行

### 前提条件

- Go 1.26+
- Python 3 (開発用サーバー)

### ビルド

```bash
cd wasm-kifu
make build
```

### テスト

```bash
make test
```

### デモ起動

```bash
make serve
# ブラウザで http://localhost:8080 を開く
```

## JavaScript API

```javascript
import { WasmKifu } from './js/wasm-bridge.js';

const kifu = new WasmKifu();
await kifu.load('wasm/kifu.wasm');

// 新規ゲーム
kifu.newGame();
kifu.newGameWithInfo('黒太郎', '白花子', 6.5);

// 着手
kifu.playMove(3, 3);  // 0=成功, 1=違法手
kifu.pass();
kifu.undo();
kifu.resign();

// 状態取得
kifu.getBoardState();    // {stones, player, moveNumber, ...}
kifu.getCurrentPlayer(); // 1=黒, 2=白
kifu.getMoveNumber();
kifu.getLastMoveInfo();  // {x, y, color, moveNumber, isPass}

// SGF
kifu.exportSGF();        // SGF文字列
kifu.importSGF(sgfStr);  // true/false

// 画像認識
kifu.analyzeBoardImage(intersections); // 差分解析
kifu.applyDetectedMove(x, y, color);  // 検出着手の適用
```

## カメラによる画像認識

1. 「📷 カメラ」ボタンでカメラを起動
2. 碁盤の4隅をタップ (左上 → 右上 → 右下 → 左下)
3. 「📸 撮影」ボタンで盤面を認識
4. 検出された着手が自動的に記録される

## ライセンス

MIT
