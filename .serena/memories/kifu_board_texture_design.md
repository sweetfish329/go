# kifu_board_texture_design.md - 碁盤および碁石の質感設計知見（ライト・ダーク共通化）

## 1. 課題と背景
- **以前のライトテーマの課題**:
  - 盤面が薄い青緑色のグレーセージ（`#b8c6c0`）単色フラット塗りで、本物の木製碁盤のような温かみや高級感が不足していた。
  - 白石のグラデーションの100%地点（外周）に `--wc-border: #253530`（濃墨色）が指定されていたため、白石の縁が黒ずみ・煤けたように見えていた。
  - 黒石もハイライトが緑がかった淡灰色で、石本来の「漆黒の艶」が出ていなかった。
- **改善の目的**:
  ダークテーマで大好評だった「Ash Wood / 本榧木肌」と「那智黒石・蛤白石」の上質でリアルな質感を、ライトテーマにも完全適用し、ライト・ダーク双方で最高峰の碁盤デザインを実現する。

## 2. 採用した設計・カラーパレット
1. **盤面木肌（Ash Wood / 本榧肌）**:
   - 基本色: `--wc-board-bg: #ebdac0;`（ミルクティーベージュ・サムネイルと完全一致）
   - 自然採光グラデーション:
     - 上部採光: `--wc-board-bg-top: #f2e5d2;`
     - 下部陰影: `--wc-board-bg-bottom: #e6d3b7;`
     - SVG `<linearGradient id="boardSurfaceGrad">` で盤面にふわりと差し込む自然光を表現。
   - 工芸的木工面取り（Inner Bevel）:
     - 盤面外枠（3px）の内側に、1px の微細なハイライト面取りライン（`stroke="rgba(255, 255, 255, 0.45)"`）を配置。
     - 職人が削り出した碁盤の立体的な厚み・面研ぎ感を演出。
2. **那智黒石（Lacquered Kuroishi）**:
   - ハイライト: `--wc-go-black-grad-0: #4a5c55;`（上品な漆黒の艶）
   - 本体: `--wc-go-black-grad-35: #18221e;`
   - 最深部: `--wc-go-black-grad-100: #0a0f0d;`
   - 繊細な輪郭: `--wc-go-black-stroke: #101614;`
3. **蛤白石（Smooth Matte Shiroishi）**:
   - 純白ハイライト: `--wc-go-white-grad-0: #ffffff;`
   - 滑らかな白: `--wc-go-white-grad-75: #f8faf9;`
   - 縁の立体陰影: `--wc-go-white-grad-100: #c8d2cd;`（従来の濃墨黒ずみを完全解消、柔らかいセージグレーの陰影）
   - 輪郭線: `--wc-go-white-stroke: rgba(37, 53, 48, 0.4);`
4. **立体ドロップシャドウ**:
   - `feDropShadow dx="1.2" dy="2.0" stdDeviation="0.7"`
   - `flood-color: var(--wc-board-shadow, rgba(0, 0, 0, 0.4))` / `flood-opacity="0.38"`
   - 木肌にふわりと落ちる実在感ある影。

## 3. 実画面検証（CDP）
- `board_light_enhanced.png` / `board_light_closeup.png` にて、淡いセージグリーン調の北欧モダンUI背景の中で、Ash Wood 碁盤が美しく際立ち、白石・黒石の視認性と高級感が劇的に向上したことを確認。
