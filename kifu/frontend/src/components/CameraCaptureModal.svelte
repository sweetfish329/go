<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { fade, scale } from "svelte/transition";
  import { BoardDetector, type Point2D } from "@wasm-kifu/web/js/board-detector.js";
  import { WasmKifu } from "@wasm-kifu/web/js/wasm-bridge.js";
  import kifuWasmUrl from "@wasm-kifu/web/wasm/kifu.wasm?url";

  let { isOpen, onClose, onImport } = $props<{
    isOpen: boolean;
    onClose: () => void;
    onImport: (detected: { board: number[][]; blackCount: number; whiteCount: number }) => void;
  }>();

  // Mode: "camera" | "file"
  let activeTab = $state<"camera" | "file">("file");

  // OpenCV / WASM states
  let isInitializing = $state(true);
  let isCvReady = $state(false);
  let statusMessage = $state("OpenCV / WASM をロード中...");

  // Camera states
  let videoEl: HTMLVideoElement | null = $state(null);
  let overlayCanvas: HTMLCanvasElement | null = $state(null);
  let previewCanvas: HTMLCanvasElement | null = $state(null);
  let mediaStream: MediaStream | null = $state(null);
  let facingMode = $state<"user" | "environment">("environment");

  // Image & Detection states
  let loadedImageMat: HTMLImageElement | HTMLCanvasElement | null = $state(null);
  let cornerPoints = $state<Point2D[]>([]);
  let isAnalyzing = $state(false);
  let detectedBoard = $state<number[][] | null>(null);
  let detectedBlackCount = $state(0);
  let detectedWhiteCount = $state(0);
  let errorMessage = $state("");

  const detector = new BoardDetector();
  const kifu = new WasmKifu();

  onMount(async () => {
    try {
      // 1. WASM ロード
      statusMessage = "WASM エンジンを初期化中...";
      await kifu.load(kifuWasmUrl);

      // 2. OpenCV.js 待機
      statusMessage = "画像認識ライブラリを準備中...";
      await detector.waitForOpenCV();
      isCvReady = true;

      isInitializing = false;
      statusMessage = "準備完了";
    } catch (err: any) {
      console.error("Failed to initialize vision engine:", err);
      errorMessage = `初期化エラー: ${err.message || err}`;
      isInitializing = false;
    }
  });

  onDestroy(() => {
    stopCameraStream();
  });

  // Watch for isOpen changes
  $effect(() => {
    if (isOpen && activeTab === "camera" && isCvReady) {
      startCameraStream();
    } else if (!isOpen) {
      stopCameraStream();
      resetState();
    }
  });

  function resetState() {
    cornerPoints = [];
    loadedImageMat = null;
    detectedBoard = null;
    detectedBlackCount = 0;
    detectedWhiteCount = 0;
    errorMessage = "";
    detector.resetCorners();
  }

  async function startCameraStream() {
    if (!videoEl || !isCvReady) return;
    try {
      errorMessage = "";
      stopCameraStream();
      mediaStream = await detector.startCamera(videoEl, facingMode);
    } catch (err: any) {
      console.error("Camera error:", err);
      errorMessage = `カメラ起動エラー: ${err.message || "カメラにアクセスできません"}`;
    }
  }

  function stopCameraStream() {
    if (mediaStream) {
      detector.stopCamera(mediaStream);
      mediaStream = null;
    }
  }

  function toggleCameraFacing() {
    facingMode = facingMode === "environment" ? "user" : "environment";
    startCameraStream();
  }

  function handleTabChange(tab: "camera" | "file") {
    activeTab = tab;
    resetState();
    if (tab === "camera") {
      setTimeout(() => startCameraStream(), 100);
    } else {
      stopCameraStream();
    }
  }

  // ファイルアップロード処理
  function handleFileUpload(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;

    const file = target.files[0];
    const reader = new FileReader();

    reader.onload = (event) => {
      const img = new Image();
      img.onload = () => {
        const canvas = document.createElement("canvas");
        canvas.width = img.width;
        canvas.height = img.height;
        const ctx = canvas.getContext("2d");
        ctx?.drawImage(img, 0, 0);

        loadedImageMat = canvas;
        drawLoadedImageOnPreview(canvas);
        autoDetectCornersFromSource(canvas);
      };
      img.src = event.target?.result as string;
    };
    reader.readAsDataURL(file);
  }

  function handleCameraCapture() {
    if (!videoEl || !previewCanvas) return;
    const canvas = document.createElement("canvas");
    canvas.width = videoEl.videoWidth || 640;
    canvas.height = videoEl.videoHeight || 480;
    const ctx = canvas.getContext("2d");
    ctx?.drawImage(videoEl, 0, 0);

    loadedImageMat = canvas;
    drawLoadedImageOnPreview(canvas);
    autoDetectCornersFromSource(canvas);
  }

  function drawLoadedImageOnPreview(canvasSource: HTMLCanvasElement) {
    if (!previewCanvas) return;
    previewCanvas.width = canvasSource.width;
    previewCanvas.height = canvasSource.height;
    const ctx = previewCanvas.getContext("2d");
    ctx?.drawImage(canvasSource, 0, 0);
    renderCornerOverlay();
  }

  function autoDetectCornersFromSource(source: HTMLCanvasElement) {
    try {
      errorMessage = "";
      const corners = detector.autoDetectCorners(source);
      if (corners && corners.length === 4) {
        cornerPoints = corners;
        detector.setCorners(corners);
        renderCornerOverlay();
      } else {
        errorMessage = "4隅を自動検出できませんでした。画像上の4隅を直接タップして指定してください。";
      }
    } catch (err: any) {
      console.warn("Auto detect corner error:", err);
      errorMessage = "4隅を自動検出できませんでした。手動でタップして指定してください。";
    }
  }

  function handlePreviewCanvasClick(e: MouseEvent) {
    if (!previewCanvas || cornerPoints.length >= 4) return;

    const rect = previewCanvas.getBoundingClientRect();
    const scaleX = previewCanvas.width / rect.width;
    const scaleY = previewCanvas.height / rect.height;

    const point = {
      x: Math.round((e.clientX - rect.left) * scaleX),
      y: Math.round((e.clientY - rect.top) * scaleY),
    };

    cornerPoints = [...cornerPoints, point];

    if (cornerPoints.length === 4) {
      detector.setCorners(cornerPoints);
    }
    renderCornerOverlay();
  }

  function renderCornerOverlay() {
    if (!overlayCanvas || !previewCanvas) return;
    overlayCanvas.width = previewCanvas.width;
    overlayCanvas.height = previewCanvas.height;

    const ctx = overlayCanvas.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, overlayCanvas.width, overlayCanvas.height);

    if (cornerPoints.length === 0) return;

    // Draw lines connecting points
    if (cornerPoints.length > 1) {
      ctx.beginPath();
      ctx.strokeStyle = "rgba(193, 154, 107, 0.9)";
      ctx.lineWidth = 3;
      ctx.setLineDash([6, 6]);
      ctx.moveTo(cornerPoints[0].x, cornerPoints[0].y);
      for (let i = 1; i < cornerPoints.length; i++) {
        ctx.lineTo(cornerPoints[i].x, cornerPoints[i].y);
      }
      if (cornerPoints.length === 4) {
        ctx.closePath();
      }
      ctx.stroke();
      ctx.setLineDash([]);
    }

    // Draw markers
    cornerPoints.forEach((p, idx) => {
      ctx.beginPath();
      ctx.arc(p.x, p.y, 12, 0, Math.PI * 2);
      ctx.fillStyle = "#8b5a2b";
      ctx.fill();
      ctx.strokeStyle = "#ffffff";
      ctx.lineWidth = 3;
      ctx.stroke();

      ctx.fillStyle = "#ffffff";
      ctx.font = "bold 13px sans-serif";
      ctx.textAlign = "center";
      ctx.textBaseline = "middle";
      ctx.fillText(String(idx + 1), p.x, p.y);
    });
  }

  async function handleAnalyzeBoard() {
    if (!loadedImageMat) {
      errorMessage = "画像を先に撮影またはアップロードしてください。";
      return;
    }
    if (cornerPoints.length !== 4) {
      errorMessage = "4隅の指定が完了していません。4つの角をタップしてください。";
      return;
    }

    isAnalyzing = true;
    errorMessage = "";

    try {
      // 1. 実測間隔キャリブレーション（Recorded Spacings Model）
      try {
        detector.calibrateRecordedSpacings(loadedImageMat);
      } catch (err) {
        console.warn("Recorded spacings calibration fallback:", err);
      }

      // 19x19 盤面マトリクスを初期化
      const board: number[][] = Array.from({ length: 19 }, () => Array(19).fill(0));
      let bCount = 0;
      let wCount = 0;

      // 最新の 95% 超高精度パイプライン (大域幾何最適化・レンズ歪みピークスナッピング・天板アライメント) で全盤面を認識
      const detected = detector.detect(loadedImageMat);
      for (let y = 0; y < 19; y++) {
        for (let x = 0; x < 19; x++) {
          const val = detected[y * 19 + x];
          board[y][x] = val;
          if (val === 1) bCount++;
          if (val === 2) wCount++;
        }
      }

      detectedBoard = board;
      detectedBlackCount = bCount;
      detectedWhiteCount = wCount;
    } catch (err: any) {
      console.error("Board analysis error:", err);
      errorMessage = `盤面解析エラー: ${err.message || err}`;
    } finally {
      isAnalyzing = false;
    }
  }

  function handleConfirmImport() {
    if (!detectedBoard) return;
    onImport({
      board: detectedBoard,
      blackCount: detectedBlackCount,
      whiteCount: detectedWhiteCount,
    });
    onClose();
  }
</script>

{#if isOpen}
  <!-- Modal Backdrop -->
  <div
    transition:fade={{ duration: 180 }}
    class="camera-modal-backdrop"
    onclick={onClose}
    onkeydown={(e) => { if (e.key === "Escape") onClose(); }}
    role="presentation"
  >
    <!-- Modal Content -->
    <div
      transition:scale={{ duration: 220, start: 0.95 }}
      class="camera-modal-dialog"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => { if (e.key === "Escape") onClose(); }}
      tabindex="-1"
      role="dialog"
      aria-modal="true"
      aria-labelledby="camera-modal-title"
    >
      <!-- Header -->
      <div class="modal-header">
        <div class="header-title-row">
          <i class="material-icons header-icon">camera_enhance</i>
          <h5 id="camera-modal-title" class="font-mincho header-title">
            盤面写真・画像からAI自動取り込み
          </h5>
        </div>
        <button
          type="button"
          class="btn-close cursor-pointer"
          onclick={onClose}
          aria-label="閉じる"
        >
          <i class="material-icons">close</i>
        </button>
      </div>

      <!-- Tab Switcher -->
      <div class="modal-tabs">
        <button
          type="button"
          class="tab-btn cursor-pointer"
          class:active={activeTab === "file"}
          onclick={() => handleTabChange("file")}
        >
          <i class="material-icons tab-icon">image</i>
          画像ファイル選択
        </button>
        <button
          type="button"
          class="tab-btn cursor-pointer"
          class:active={activeTab === "camera"}
          onclick={() => handleTabChange("camera")}
          disabled={!isCvReady}
        >
          <i class="material-icons tab-icon">videocam</i>
          カメラで直接撮影
        </button>
      </div>

      <!-- Loading / Status Banner -->
      {#if isInitializing}
        <div class="status-banner loading">
          <div class="spinner"></div>
          <span>{statusMessage}</span>
        </div>
      {/if}

      {#if errorMessage}
        <div class="status-banner error">
          <i class="material-icons">error_outline</i>
          <span>{errorMessage}</span>
        </div>
      {/if}

      <!-- Main Body -->
      <div class="modal-body">
        {#if activeTab === "camera"}
          <!-- Camera Stream Section -->
          <div class="camera-stream-container">
            <video
              bind:this={videoEl}
              autoplay
              playsinline
              muted
              class="camera-video-element"
            ></video>
            <div class="camera-actions-overlay">
              <button
                type="button"
                class="btn-camera-ctrl cursor-pointer"
                onclick={toggleCameraFacing}
                title="カメラ切り替え"
              >
                <i class="material-icons">flip_camera_ios</i>
              </button>
              <button
                type="button"
                class="btn-shutter cursor-pointer"
                onclick={handleCameraCapture}
                title="撮影"
              >
                <i class="material-icons shutter-icon">camera</i>
              </button>
            </div>
          </div>
        {:else}
          <!-- File Upload Section -->
          <div class="file-upload-section">
            <label class="upload-dropzone cursor-pointer">
              <input
                type="file"
                accept="image/*"
                onchange={handleFileUpload}
                class="hidden-file-input"
              />
              <i class="material-icons upload-icon">cloud_upload</i>
              <span class="upload-title">写真を選択 または ここにドラッグ＆ドロップ</span>
              <span class="upload-subtitle">碁盤全体が写っている写真（PNG, JPEG, WebP）</span>
            </label>
          </div>
        {/if}

        <!-- Interactive Preview & Corner Adjustment Canvas -->
        {#if loadedImageMat}
          <div class="preview-calibration-section">
            <div class="section-title-row">
              <span class="section-title">
                <i class="material-icons">crop_free</i>
                碁盤の4隅指定（{cornerPoints.length}/4）
              </span>
              <div class="corner-btn-group">
                <button
                  type="button"
                  class="btn-tool cursor-pointer"
                  onclick={() => autoDetectCornersFromSource(loadedImageMat as HTMLCanvasElement)}
                  disabled={isAnalyzing}
                >
                  <i class="material-icons">auto_fix_high</i>
                  自動検出
                </button>
                <button
                  type="button"
                  class="btn-tool cursor-pointer"
                  onclick={() => { cornerPoints = []; renderCornerOverlay(); }}
                >
                  <i class="material-icons">restart_alt</i>
                  リセット
                </button>
              </div>
            </div>

            <div class="canvas-wrapper">
              <canvas
                bind:this={previewCanvas}
                onclick={handlePreviewCanvasClick}
                class="board-preview-canvas"
              ></canvas>
              <canvas
                bind:this={overlayCanvas}
                class="corner-overlay-canvas"
              ></canvas>
            </div>
            <p class="calibration-hint">
              ※ 角の位置がずれている場合は、左上 → 右上 → 右下 → 左下 の順に角をタップしてください。
            </p>
          </div>
        {/if}

        <!-- Detection Results Preview -->
        {#if detectedBoard}
          <div class="detected-result-card">
            <div class="result-header">
              <span class="result-title">
                <i class="material-icons">check_circle</i>
                検出された碁盤の状態
              </span>
              <span class="stone-counts">
                ⚫ 黒石: {detectedBlackCount}子 / ⚪ 白石: {detectedWhiteCount}子
              </span>
            </div>

            <!-- Mini SVG preview of detected state -->
            <div class="mini-board-preview">
              <svg viewBox="0 0 190 190" class="mini-board-svg">
                <rect width="190" height="190" fill="var(--wc-board, #dbb268)" />
                {#each Array(19) as _, i}
                  <line x1="5" y1={5 + i * 10} x2="185" y2={5 + i * 10} stroke="#2a2018" stroke-width="0.6" />
                  <line x1={5 + i * 10} y1="5" x2={5 + i * 10} y2="185" stroke="#2a2018" stroke-width="0.6" />
                {/each}
                {#each detectedBoard as row, y}
                  {#each row as cell, x}
                    {#if cell === 1}
                      <circle cx={5 + x * 10} cy={5 + y * 10} r="4.2" fill="#111111" stroke="#444" stroke-width="0.4" />
                    {:else if cell === 2}
                      <circle cx={5 + x * 10} cy={5 + y * 10} r="4.2" fill="#ffffff" stroke="#999" stroke-width="0.4" />
                    {/if}
                  {/each}
                {/each}
              </svg>
            </div>
          </div>
        {/if}
      </div>

      <!-- Footer Actions -->
      <div class="modal-footer">
        <button
          type="button"
          class="btn-cancel cursor-pointer"
          onclick={onClose}
        >
          キャンセル
        </button>

        {#if !detectedBoard}
          <button
            type="button"
            class="btn-primary cursor-pointer"
            onclick={handleAnalyzeBoard}
            disabled={isAnalyzing || !loadedImageMat || cornerPoints.length !== 4}
          >
            {#if isAnalyzing}
              <div class="spinner-small"></div>
              <span>AI解析中...</span>
            {:else}
              <i class="material-icons">bolt</i>
              <span>盤面をAI解析</span>
            {/if}
          </button>
        {:else}
          <button
            type="button"
            class="btn-primary-import cursor-pointer"
            onclick={handleConfirmImport}
          >
            <i class="material-icons">done_all</i>
            <span>この盤面を取り込む</span>
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .camera-modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(20, 16, 12, 0.65);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    z-index: 1200;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 16px;
  }

  .camera-modal-dialog {
    background: var(--wc-surface, #fdfbf7);
    border: 2px solid var(--wc-text, #2c2523);
    box-shadow: 8px 8px 0px var(--wc-text, #2c2523);
    border-radius: 4px;
    width: 100%;
    max-width: 640px;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 20px;
    border-bottom: 2px solid var(--wc-text, #2c2523);
    background: var(--wc-surface-alt, #f7f3eb);
  }

  .header-title-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .header-icon {
    font-size: 1.4rem;
    color: var(--wc-accent, #8b5a2b);
  }

  .header-title {
    margin: 0;
    font-size: 1.15rem;
    font-weight: 700;
    color: var(--wc-text, #2c2523);
  }

  .btn-close {
    background: none;
    border: none;
    padding: 4px;
    color: var(--wc-text, #2c2523);
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
  }

  .btn-close:hover {
    background: rgba(0, 0, 0, 0.08);
  }

  .modal-tabs {
    display: flex;
    border-bottom: 1.5px solid var(--wc-text, #2c2523);
    background: var(--wc-surface-alt, #f7f3eb);
  }

  .tab-btn {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 10px 16px;
    border: none;
    background: transparent;
    font-size: 0.88rem;
    font-weight: 600;
    color: var(--wc-text-muted, #7c6b52);
    border-right: 1.5px solid var(--wc-text, #2c2523);
    transition: all 0.15s ease;
  }

  .tab-btn:last-child {
    border-right: none;
  }

  .tab-btn.active {
    background: var(--wc-surface, #fdfbf7);
    color: var(--wc-text, #2c2523);
    box-shadow: inset 0 -3px 0 var(--wc-accent, #8b5a2b);
  }

  .tab-icon {
    font-size: 1.1rem;
  }

  .status-banner {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 16px;
    font-size: 0.85rem;
    border-bottom: 1px solid var(--wc-text, #2c2523);
  }

  .status-banner.loading {
    background: #fff8e1;
    color: #856404;
  }

  .status-banner.error {
    background: #f8d7da;
    color: #721c24;
  }

  .spinner {
    width: 16px;
    height: 16px;
    border: 2px solid rgba(0, 0, 0, 0.2);
    border-top-color: currentColor;
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }

  .spinner-small {
    width: 14px;
    height: 14px;
    border: 2px solid rgba(255, 255, 255, 0.4);
    border-top-color: #ffffff;
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .modal-body {
    padding: 16px 20px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  /* File upload dropzone */
  .upload-dropzone {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 28px 16px;
    border: 2px dashed var(--wc-accent, #8b5a2b);
    border-radius: 6px;
    background: rgba(139, 90, 43, 0.04);
    text-align: center;
    gap: 8px;
    transition: all 0.2s ease;
  }

  .upload-dropzone:hover {
    background: rgba(139, 90, 43, 0.08);
  }

  .hidden-file-input {
    display: none;
  }

  .upload-icon {
    font-size: 2.5rem;
    color: var(--wc-accent, #8b5a2b);
  }

  .upload-title {
    font-weight: 700;
    font-size: 0.95rem;
    color: var(--wc-text, #2c2523);
  }

  .upload-subtitle {
    font-size: 0.78rem;
    color: var(--wc-text-muted, #7c6b52);
  }

  /* Camera Stream */
  .camera-stream-container {
    position: relative;
    width: 100%;
    height: 280px;
    background: #000;
    border-radius: 4px;
    overflow: hidden;
  }

  .camera-video-element {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .camera-actions-overlay {
    position: absolute;
    bottom: 12px;
    left: 0;
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 20px;
    z-index: 5;
  }

  .btn-camera-ctrl {
    background: rgba(0, 0, 0, 0.6);
    color: #fff;
    border: 1px solid rgba(255, 255, 255, 0.4);
    border-radius: 50%;
    width: 42px;
    height: 42px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .btn-shutter {
    background: #e94560;
    color: #fff;
    border: 3px solid #ffffff;
    border-radius: 50%;
    width: 58px;
    height: 58px;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 4px 10px rgba(0, 0, 0, 0.4);
  }

  .shutter-icon {
    font-size: 1.8rem;
  }

  /* Calibration Canvas */
  .preview-calibration-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .section-title-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .section-title {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.88rem;
    font-weight: 700;
    color: var(--wc-text, #2c2523);
  }

  .corner-btn-group {
    display: flex;
    gap: 8px;
  }

  .btn-tool {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 4px 10px;
    font-size: 0.78rem;
    font-weight: 600;
    background: var(--wc-surface-alt, #f7f3eb);
    border: 1px solid var(--wc-text, #2c2523);
    box-shadow: 2px 2px 0px var(--wc-text, #2c2523);
    color: var(--wc-text, #2c2523);
    border-radius: 2px;
  }

  .btn-tool:hover {
    background: var(--wc-surface, #fdfbf7);
  }

  .canvas-wrapper {
    position: relative;
    width: 100%;
    max-height: 280px;
    background: #111;
    border: 1.5px solid var(--wc-text, #2c2523);
    border-radius: 4px;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .board-preview-canvas {
    max-width: 100%;
    max-height: 280px;
    object-fit: contain;
    cursor: crosshair;
  }

  .corner-overlay-canvas {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
  }

  .calibration-hint {
    margin: 0;
    font-size: 0.74rem;
    color: var(--wc-text-muted, #7c6b52);
  }

  /* Result Card */
  .detected-result-card {
    background: var(--wc-surface-alt, #f7f3eb);
    border: 1.5px solid var(--wc-text, #2c2523);
    border-radius: 4px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .result-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 6px;
  }

  .result-title {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.88rem;
    font-weight: 700;
    color: #2e7d32;
  }

  .stone-counts {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--wc-text, #2c2523);
  }

  .mini-board-preview {
    display: flex;
    justify-content: center;
  }

  .mini-board-svg {
    width: 160px;
    height: 160px;
    border: 2px solid var(--wc-text, #2c2523);
    box-shadow: 3px 3px 0px var(--wc-text, #2c2523);
  }

  /* Footer */
  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 12px;
    padding: 14px 20px;
    border-top: 1.5px solid var(--wc-text, #2c2523);
    background: var(--wc-surface-alt, #f7f3eb);
  }

  .btn-cancel {
    background: transparent;
    border: 1px solid var(--wc-text, #2c2523);
    color: var(--wc-text, #2c2523);
    padding: 8px 16px;
    font-size: 0.88rem;
    font-weight: 600;
    border-radius: 2px;
  }

  .btn-primary, .btn-primary-import {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: var(--wc-accent, #8b5a2b);
    border: 1.5px solid var(--wc-text, #2c2523);
    box-shadow: 3px 3px 0px var(--wc-text, #2c2523);
    color: #ffffff;
    padding: 8px 18px;
    font-size: 0.88rem;
    font-weight: 700;
    border-radius: 2px;
    transition: transform 0.1s ease;
  }

  .btn-primary-import {
    background: #2e7d32;
  }

  .btn-primary:active, .btn-primary-import:active {
    transform: translate(2px, 2px);
    box-shadow: 1px 1px 0px var(--wc-text, #2c2523);
  }

  .btn-primary:disabled {
    opacity: 0.6;
    cursor: not-allowed;
    box-shadow: none;
    transform: none;
  }
</style>
