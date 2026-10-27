# kifu_dark_theme_ui_redesign.md - ダークテーマUI全体（盤以外）の和モダン深玄セージ最適化知見

## 1. 背景と根本原因の分析
- **課題**:
  ダークテーマにおいて、碁盤以外のUI全体（ヘッダー、ナビゲーションバー、カード、フォーム入力、ボタン、ダイアログ等）に「真っ白に発光する影（ハロー）」や硬い白枠が散乱し、チープで可読性が著しく低下していた。
- **根本原因**:
  - ライトテーマでは `--wc-text: #253530`（濃墨色）だったため、`box-shadow: 4px 4px 0px var(--wc-text)` や `border: 2px solid var(--wc-text)` が「濃墨のソリッドポップ影」として美しく成立していた。
  - しかしダークテーマになると `--wc-text` が `#ece5ec` や `#e4ece8`（白・淡色系）になるため、すべてのカード・ボタン・入力枠・ヘッダーの右下に**「真っ白に発光するソリッド影」**が落ちてしまい、画面全体が白黒漫画やゴーストのように破綻していた。

## 2. 採用した解決策と設計原則
1. **`--wc-shadow-solid` と `--wc-border-solid` の分離新設**:
   - ライトテーマ（`:root`）:
     - `--wc-shadow-solid: var(--wc-text);`
     - `--wc-border-solid: var(--wc-text);`
     これにより、ユーザーが満足している既存のライトテーマデザイン・トーン（淡いセージグリーンと濃墨ソリッド影）を100%完全保護。
   - ダークテーマ（`:root.dark` および `@media (prefers-color-scheme: dark)`）:
     - `--wc-bg: #121816;`（深玄セージ背景）
     - `--wc-surface: #1a2420;`（落ち着いたセージサーフェス）
     - `--wc-border-solid: #3b4e47;`（落ち着いたダークセージ境界線）
     - `--wc-shadow-solid: #080d0b;`（深墨色のソリッド影）
2. **包括的オーバーライドルール（`neumorphism.css`）**:
   - 背景ドットパルス（`.em-bg-pulse-dots`）を静寂で品格ある薄さ（`rgba(143, 168, 159, 0.08)`）に抑制。
   - 巨大透かし文字（`.em-huge-title`, `.masthead-bg-title`）を目立ちすぎない上質な透かし（透過率4%）に調整。
   - 入力フォーム（`.nm-input`, `input`, `textarea`, `select`）の内側シャドウと外側フォーカスリング（ゴールドアンバー）の最適化。
   - スライダーつまみ、一覧カード（`.kifu-tile-card`）、各種ボタンの深墨影化。
3. **全コンポーネントの置換**:
   - `App.svelte`, `KifuCreator.svelte`, `KifuList.svelte`, `KifuDetail.svelte`, `Auth.svelte`, `ShareDialog.svelte`, `UsernameDialog.svelte`, `AdminAuth.svelte`, `AdminDashboard.svelte`, `Board.svelte` 全体にわたって `var(--wc-text)` による直接の border/shadow 指定を `--wc-border-solid` / `--wc-shadow-solid` に置換。

## 3. 検証結果
- Chrome DevTools (CDP) による実画面撮影にて、ホーム・クリエイター・ドロワー・一覧の各画面で白影・白枠が完全に解消され、Ash Wood 碁盤と調和する極上の和モダン深玄セージUIへと昇華されたことを確認。
- ライトテーマ側の表示が以前と全く変わらず完璧に維持されていることを確認。
