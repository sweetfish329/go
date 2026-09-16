<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { fade, scale } from "svelte/transition";
  import { BoardDetector, type Point2D } from "@wasm-kifu/web/js/board-detector.js";
  import { WasmKifu } from "@wasm-kifu/web/js/wasm-bridge.js";
  import kifuWasmUrl from "@wasm-kifu/web/wasm/kifu.wasm?url";

  let { isOpen, onClose, onImport } = $props<{
    isOpen: boolean;
    onClose: () => void;
    onImport: (detected: {
      board?: number[][];
      blackCount?: number;
      whiteCount?: number;
      sgf?: string;
    }) => void;
  }>();

  // Mode: "stream" (三脚自動記録) | "camera" (静止画撮影) | "file" (画像ファイル)
  let activeTab = $state<"stream" | "camera" | "file">("stream");

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

  // ⚡ 三脚リアルタイム記録（Live Game Recorder）状態
  let isStreamRecording = $state(false);
  let streamAnimationId: number | null = null;
  let lastStreamFrameTime = 0;
  let stabilityProgress = $state(0);
  let isBoardStable = $state(false);
  let motionLevel = $state(0);
  let liveMoveNumber = $state(0);
  let liveCurrentPlayer = $state<1 | 2>(1); // 1: 黒, 2: 白
  let liveLastMoveText = $state("対局待機中");
  let liveBoardState = $state<number[][]>(Array.from({ length: 19 }, () => Array(19).fill(0)));
  let detectionNotification = $state<string | null>(null);
  let notificationTimer: any = null;

  const detector = new BoardDetector();
  const kifu = new WasmKifu();

  onMount(async () => {
    try {
      statusMessage = "WASM エンジンを初期化中...";
      await kifu.load(kifuWasmUrl);

      statusMessage = "画像認識ライブラリを準備中...";
      await detector.waitForOpenCV();
      isCvReady = true;

      isInitializing = false;
      statusMessage = "準備完了";

      if (isOpen && (activeTab === "stream" || activeTab === "camera")) {
        startCameraStream();
      }
    } catch (err: any) {
      console.error("Failed to initialize vision engine:", err);
      errorMessage = `初期化エラー: ${err.message || err}`;
      isInitializing = false;
    }
  });

  onDestroy(() => {
    stopStreamRecording();
    stopCameraStream();
  });

  // Watch for isOpen changes
  $effect(() => {
    if (isOpen && (activeTab === "stream" || activeTab === "camera") && isCvReady) {
      startCameraStream();
    } else if (!isOpen) {
      stopStreamRecording();
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
    liveMoveNumber = 0;
    liveCurrentPlayer = 1;
    liveLastMoveText = "対局待機中";
    liveBoardState = Array.from({ length: 19 }, () => Array(19).fill(0));
    detector.resetCorners();
    detector.resetStreamTracker();
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

  function handleTabChange(tab: "stream" | "camera" | "file") {
    activeTab = tab;
    stopStreamRecording();
    resetState();
    if (tab === "stream" || tab === "camera") {
      setTimeout(() => startCameraStream(), 100);
    } else {
      stopCameraStream();
    }
  }

  // ⚡ 三脚ストリーム記録の開始 / 停止
  function startStreamRecording() {
    if (!detector.hasCornersSet()) {
      errorMessage = "先に碁盤の4隅を指定してください（自動検出またはタップ）";
      return;
    }
    errorMessage = "";
    isStreamRecording = true;
    detector.initStreamTracker();
    kifu.newGame();
    liveMoveNumber = 0;
    liveCurrentPlayer = 1;
    liveLastMoveText = "対局開始（黒番の手番です）";
    liveBoardState = Array.from({ length: 19 }, () => Array(19).fill(0));
    showNotification("⚡ 自動記録を開始しました。着手してください。");
    requestAnimationFrame(streamLoop);
  }

  function stopStreamRecording() {
    isStreamRecording = false;
    if (streamAnimationId) {
      cancelAnimationFrame(streamAnimationId);
      streamAnimationId = null;
    }
    detector.resetStreamTracker();
  }

  function streamLoop(timestamp: number) {
    if (!isStreamRecording || !isOpen || activeTab !== "stream") return;

    // 約 15fps にスロットリング（スマホの発熱・電池消費を抑止）
    if (!timestamp || timestamp - lastStreamFrameTime > 65) {
      lastStreamFrameTime = timestamp || 0;

      if (videoEl && videoEl.readyState >= 2) {
        try {
          const res = detector.processStreamFrame(videoEl);

          stabilityProgress = Math.round(res.stableProgress * 100);
          isBoardStable = res.isStable;
          motionLevel = res.motionLevel;

          // 静止確定トリガー発火
          if (res.isTriggered && res.features) {
            handleStreamDetectedFeatures(res.features);
          }
        } catch (err: any) {
          console.warn("Stream processing warning:", err);
        }
      }
    }

    streamAnimationId = requestAnimationFrame(streamLoop);
  }

  function handleStreamDetectedFeatures(features: any[]) {
    try {
      const analysis = kifu.analyzeBoardFeatures(features);

      if (analysis.moves && analysis.moves.length === 1) {
        const move = analysis.moves[0];
        const colorNum = move.color === "B" ? 1 : 2;
        const result = kifu.applyDetectedMove(move.x, move.y, colorNum);

        if (result === 0) {
          const state = kifu.getBoardState();
          liveBoardState = state.stones;
          liveMoveNumber = kifu.getMoveNumber();
          liveCurrentPlayer = kifu.getCurrentPlayer() as 1 | 2;
          const colName = move.color === "B" ? "黒" : "白";
          const colLetters = "ABCDEFGHJKLMNOPQRST";
          const coordStr = `${colLetters[move.x]}${19 - move.y}`;
          liveLastMoveText = `${liveMoveNumber}手目: ${colName} ${coordStr}`;

          showNotification(`⚡ ${colName} ${coordStr} を記録しました`);
        } else {
          console.warn("Detected move was illegal:", move);
        }
      }
    } catch (err: any) {
      console.error("Stream move analysis error:", err);
    }
  }

  function showNotification(msg: string) {
    detectionNotification = msg;
    if (notificationTimer) clearTimeout(notificationTimer);
    notificationTimer = setTimeout(() => {
      detectionNotification = null;
    }, 3500);
  }

  function handleStreamUndo() {
    if (kifu.undo()) {
      const state = kifu.getBoardState();
      liveBoardState = state.stones;
      liveMoveNumber = kifu.getMoveNumber();
      liveCurrentPlayer = kifu.getCurrentPlayer() as 1 | 2;
      liveLastMoveText = `${liveMoveNumber}手目に戻しました`;
      showNotification("↩ 1手戻しました");
    }
  }

  function handleStreamPass() {
    kifu.pass();
    liveMoveNumber = kifu.getMoveNumber();
    liveCurrentPlayer = kifu.getCurrentPlayer() as 1 | 2;
    liveLastMoveText = `${liveMoveNumber}手目: パス`;
    showNotification("パスしました");
  }

  function handleStreamFinishAndImport() {
    stopStreamRecording();
    stopCameraStream();
    const sgf = kifu.exportSGF();
    onImport({
      sgf,
      board: liveBoardState,
      blackCount: liveBoardState.flat().filter(c => c === 1).length,
      whiteCount: liveBoardState.flat().filter(c => c === 2).length,
    });
    onClose();
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

  function detectCornersFromElement(source: HTMLCanvasElement | HTMLVideoElement | HTMLImageElement): Point2D[] | null {
    const width = (source as HTMLVideoElement).videoWidth || (source as HTMLImageElement).naturalWidth || source.width;
    const height = (source as HTMLVideoElement).videoHeight || (source as HTMLImageElement).naturalHeight || source.height;
    if (!width || !height) return null;

    // 1. オフスクリーン Canvas で RGBA データを取得
    const offscreen = document.createElement("canvas");
    offscreen.width = width;
    offscreen.height = height;
    const ctx = offscreen.getContext("2d", { willReadFrequently: true });
    if (!ctx) return null;
    ctx.drawImage(source, 0, 0, width, height);
    const imgData = ctx.getImageData(0, 0, width, height);

    // 2. Go WASM による高精度・ロバスト検出（RANSAC、ブロブ追跡、シャドウライン補正）
    if (kifu.isReady()) {
      try {
        const res = kifu.detectBoardCorners(imgData.data, width, height);
        if (res && res.corners && res.corners.length === 4) {
          return res.corners;
        }
      } catch (err) {
        console.warn("Wasm detectBoardCorners notice:", err);
      }
    }

    // 3. フォールバック: OpenCV.js
    if (detector.isReady()) {
      try {
        const detected = detector.autoDetectCorners(offscreen);
        if (detected && detected.length === 4) {
          return detected;
        }
      } catch (err) {
        console.warn("OpenCV autoDetectCorners notice:", err);
      }
    }

    return null;
  }

  function autoDetectCornersFromSource(source: HTMLCanvasElement) {
    try {
      errorMessage = "";
      const corners = detectCornersFromElement(source);
      if (corners && corners.length === 4) {
        cornerPoints = corners;
        detector.setCorners(corners);
        renderCornerOverlay();
        showNotification("🎯 4隅を自動検出しました");
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
      const source = loadedImageMat || previewCanvas;
      if (source && detector.isReady()) {
        try {
          const refined = detector.refineCorners(source, cornerPoints);
          if (refined && refined.length === 4) {
            cornerPoints = refined;
          }
        } catch (err) {
          console.warn("Corner refinement notice:", err);
        }
      }
      detector.setCorners(cornerPoints);
    }
    renderCornerOverlay();
  }

  function autoDetectCornersFromVideo() {
    if (!videoEl) return;
    try {
      errorMessage = "";
      const corners = detectCornersFromElement(videoEl);
      if (corners && corners.length === 4) {
        cornerPoints = corners;
        detector.setCorners(corners);
        renderStreamCornerOverlay();
        showNotification("🎯 4隅を自動検出しました");
      } else {
        errorMessage = "4隅を自動検出できませんでした。カメラ映像上の4隅を直接タップしてください。";
      }
    } catch (err: any) {
      console.warn("Video corner auto detect error:", err);
      errorMessage = "4隅を自動検出できませんでした。手動でタップして指定してください。";
    }
  }

  function handleStreamOverlayClick(e: MouseEvent) {
    if (!overlayCanvas || !videoEl || isStreamRecording) return;
    const rect = overlayCanvas.getBoundingClientRect();
    const scaleX = (videoEl.videoWidth || overlayCanvas.width) / rect.width;
    const scaleY = (videoEl.videoHeight || overlayCanvas.height) / rect.height;

    const point = {
      x: Math.round((e.clientX - rect.left) * scaleX),
      y: Math.round((e.clientY - rect.top) * scaleY),
    };

    if (cornerPoints.length >= 4) {
      cornerPoints = [point];
    } else {
      cornerPoints = [...cornerPoints, point];
    }

    if (cornerPoints.length === 4) {
      detector.setCorners(cornerPoints);
    }
    renderStreamCornerOverlay();
  }

  function renderStreamCornerOverlay() {
    if (!overlayCanvas || !videoEl) return;
    overlayCanvas.width = videoEl.videoWidth || 640;
    overlayCanvas.height = videoEl.videoHeight || 480;

    const ctx = overlayCanvas.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, overlayCanvas.width, overlayCanvas.height);

    if (cornerPoints.length === 0) return;

    // Draw lines connecting points
    if (cornerPoints.length > 1) {
      ctx.beginPath();
      ctx.strokeStyle = "rgba(46, 204, 113, 0.9)";
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
      ctx.arc(p.x, p.y, 14, 0, Math.PI * 2);
      ctx.fillStyle = "#27ae60";
      ctx.fill();
      ctx.strokeStyle = "#ffffff";
      ctx.lineWidth = 3;
      ctx.stroke();

      ctx.fillStyle = "#ffffff";
      ctx.font = "bold 14px sans-serif";
      ctx.textAlign = "center";
      ctx.textBaseline = "middle";
      ctx.fillText(String(idx + 1), p.x, p.y);
    });
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

      // 2. 多次元特徴量抽出 (ドーナツ走査 + 距離変換スナッピング + 局所背景適応)
      const features = detector.extractFeatures(loadedImageMat);

      // 3. Go WASM による全盤面 (361交点) 一括高精度解析
      let fullBoard: any = null;
      if (kifu.isReady()) {
        try {
          fullBoard = kifu.analyzeFullBoard(features);
        } catch (err) {
          console.warn("WASM analyzeFullBoard notice:", err);
        }
      }

      if (fullBoard && fullBoard.grid && fullBoard.grid.length === 361) {
        for (let y = 0; y < 19; y++) {
          for (let x = 0; x < 19; x++) {
            const val = fullBoard.grid[y * 19 + x];
            board[y][x] = val;
          }
        }
        bCount = fullBoard.blackCount ?? 0;
        wCount = fullBoard.whiteCount ?? 0;
      } else {
        // フォールバック: OpenCV.js 直接分類
        const detected = detector.detect(loadedImageMat);
        for (let y = 0; y < 19; y++) {
          for (let x = 0; x < 19; x++) {
            const val = detected[y * 19 + x];
            board[y][x] = val;
            if (val === 1) bCount++;
            if (val === 2) wCount++;
          }
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
          class:active={activeTab === "stream"}
          onclick={() => handleTabChange("stream")}
          disabled={!isCvReady}
        >
          <i class="material-icons tab-icon">play_circle_filled</i>
          ⚡ 三脚自動記録
        </button>
        <button
          type="button"
          class="tab-btn cursor-pointer"
          class:active={activeTab === "camera"}
          onclick={() => handleTabChange("camera")}
          disabled={!isCvReady}
        >
          <i class="material-icons tab-icon">photo_camera</i>
          📷 静止画撮影
        </button>
        <button
          type="button"
          class="tab-btn cursor-pointer"
          class:active={activeTab === "file"}
          onclick={() => handleTabChange("file")}
        >
          <i class="material-icons tab-icon">image</i>
          📁 ファイル選択
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
        {#if activeTab === "stream"}
          <!-- ⚡ Live Stream Recorder Section -->
          <div class="stream-recorder-container">
            <!-- Top Status Bar & Stability Gauge -->
            <div class="stream-status-panel">
              <div class="status-badge-row">
                {#if isStreamRecording}
                  {#if isBoardStable}
                    <span class="badge stable animate-pulse">🕊 盤面静止中（着手待機）</span>
                  {:else}
                    <span class="badge moving">🖐 手を検知中（変化量: {motionLevel}）</span>
                  {/if}
                {:else}
                  <span class="badge standby">⏸ 記録待機中</span>
                {/if}
                <span class="move-info-badge">
                  {liveMoveNumber}手目 | {liveCurrentPlayer === 1 ? "● 黒番" : "○ 白番"}
                </span>
              </div>

              <!-- Stability Progress Bar -->
              <div class="stability-gauge-container">
                <div class="gauge-label-row">
                  <span class="gauge-title">静止安定度（確定ゲージ）</span>
                  <span class="gauge-percent">{stabilityProgress}%</span>
                </div>
                <div class="gauge-track">
                  <div
                    class="gauge-fill"
                    style="width: {stabilityProgress}%; background: {isBoardStable ? '#27ae60' : '#e67e22'};"
                  ></div>
                </div>
              </div>

              {#if detectionNotification}
                <div class="notification-toast animate-fade-in">
                  <i class="material-icons toast-icon">info</i>
                  <span>{detectionNotification}</span>
                </div>
              {/if}
            </div>

            <!-- Video + Canvas Overlay Container -->
            <div class="stream-video-wrapper">
              <video
                bind:this={videoEl}
                autoplay
                playsinline
                muted
                class="stream-video-feed"
              ></video>
              <canvas
                bind:this={overlayCanvas}
                onclick={handleStreamOverlayClick}
                class="stream-overlay-canvas cursor-pointer"
              ></canvas>

              <div class="stream-floating-tools">
                <button
                  type="button"
                  class="btn-float-ctrl cursor-pointer"
                  onclick={toggleCameraFacing}
                  title="カメラ反転"
                >
                  <i class="material-icons">flip_camera_ios</i>
                </button>
                <button
                  type="button"
                  class="btn-float-ctrl cursor-pointer"
                  onclick={autoDetectCornersFromVideo}
                  title="4隅を自動検出"
                >
                  <i class="material-icons">auto_fix_high</i>
                </button>
              </div>

              {#if cornerPoints.length < 4}
                <div class="corner-guide-hint">
                  <p>碁盤の4隅をタップして指定してください（{cornerPoints.length}/4）</p>
                </div>
              {/if}
            </div>

            <!-- Recorder Action Bar -->
            <div class="stream-controls-bar">
              {#if !isStreamRecording}
                <button
                  type="button"
                  class="btn-stream-action btn-start-rec cursor-pointer"
                  onclick={startStreamRecording}
                  disabled={cornerPoints.length < 4}
                >
                  <i class="material-icons">fiber_manual_record</i>
                  ⚡ 自動記録を開始
                </button>
              {:else}
                <button
                  type="button"
                  class="btn-stream-action btn-pause-rec cursor-pointer"
                  onclick={stopStreamRecording}
                >
                  <i class="material-icons">pause</i>
                  一時停止
                </button>
              {/if}

              <button
                type="button"
                class="btn-stream-action btn-sec cursor-pointer"
                onclick={handleStreamUndo}
                disabled={liveMoveNumber === 0}
              >
                <i class="material-icons">undo</i>
                待った
              </button>

              <button
                type="button"
                class="btn-stream-action btn-sec cursor-pointer"
                onclick={handleStreamPass}
                disabled={!isStreamRecording}
              >
                <i class="material-icons">redo</i>
                パス
              </button>

              <button
                type="button"
                class="btn-stream-action btn-finish-import cursor-pointer"
                onclick={handleStreamFinishAndImport}
                disabled={liveMoveNumber === 0}
              >
                <i class="material-icons">check_circle</i>
                対局終了・棋譜を反映
              </button>
            </div>

            <!-- Live Status & Mini Board Preview -->
            <div class="live-status-card">
              <div class="live-status-header">
                <span class="live-status-title">
                  <i class="material-icons">history</i>
                  最新着手: <b>{liveLastMoveText}</b>
                </span>
                <span class="live-stones-count">
                  ● {liveBoardState.flat().filter(c => c === 1).length} / ○ {liveBoardState.flat().filter(c => c === 2).length}
                </span>
              </div>

              <!-- Mini SVG Board -->
              <div class="mini-svg-board-container">
                <svg viewBox="0 0 380 380" class="mini-board-svg">
                  <!-- Board Background -->
                  <rect x="0" y="0" width="380" height="380" fill="#e4ba7a" rx="4" />
                  
                  <!-- Grid Lines -->
                  {#each Array(19) as _, idx}
                    <line
                      x1={10 + idx * 20} y1="10"
                      x2={10 + idx * 20} y2="370"
                      stroke="#8b5a2b" stroke-width="0.8"
                    />
                    <line
                      x1="10" y1={10 + idx * 20}
                      x2="370" y2={10 + idx * 20}
                      stroke="#8b5a2b" stroke-width="0.8"
                    />
                  {/each}

                  <!-- Star Points (3, 9, 15) -->
                  {#each [3, 9, 15] as sy}
                    {#each [3, 9, 15] as sx}
                      <circle cx={10 + sx * 20} cy={10 + sy * 20} r="2.5" fill="#8b5a2b" />
                    {/each}
                  {/each}

                  <!-- Stones -->
                  {#each liveBoardState as row, y}
                    {#each row as stone, x}
                      {#if stone === 1}
                        <circle
                          cx={10 + x * 20}
                          cy={10 + y * 20}
                          r="8.5"
                          fill="#1a1a1a"
                          stroke="#333"
                          stroke-width="0.8"
                        />
                      {:else if stone === 2}
                        <circle
                          cx={10 + x * 20}
                          cy={10 + y * 20}
                          r="8.5"
                          fill="#f7f7f7"
                          stroke="#bbb"
                          stroke-width="0.8"
                        />
                      {/if}
                    {/each}
                  {/each}
                </svg>
              </div>
            </div>
          </div>
        {:else if activeTab === "camera"}
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

  /* ⚡ Stream Recorder Mode Styles */
  .stream-recorder-container {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .stream-status-panel {
    background: var(--wc-surface-alt, #f7f3eb);
    border: 1.5px solid var(--wc-text, #2c2523);
    border-radius: 4px;
    padding: 10px 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .status-badge-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .badge {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 4px 10px;
    font-size: 0.8rem;
    font-weight: 700;
    border-radius: 4px;
  }

  .badge.standby {
    background: #e0e0e0;
    color: #424242;
  }

  .badge.moving {
    background: #fff3e0;
    color: #e65100;
    border: 1px solid #ffb74d;
  }

  .badge.stable {
    background: #e8f5e9;
    color: #2e7d32;
    border: 1px solid #81c784;
  }

  .move-info-badge {
    font-size: 0.82rem;
    font-weight: 700;
    color: var(--wc-text, #2c2523);
    background: var(--wc-surface, #ffffff);
    padding: 3px 8px;
    border-radius: 4px;
    border: 1px solid var(--wc-border, #d7ccc8);
  }

  .stability-gauge-container {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .gauge-label-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--wc-text-muted, #7c6b52);
  }

  .gauge-track {
    width: 100%;
    height: 8px;
    background: #e0dacf;
    border-radius: 4px;
    overflow: hidden;
  }

  .gauge-fill {
    height: 100%;
    transition: width 0.1s linear, background-color 0.2s ease;
  }

  .notification-toast {
    display: flex;
    align-items: center;
    gap: 6px;
    background: #2e7d32;
    color: #ffffff;
    padding: 6px 12px;
    border-radius: 4px;
    font-size: 0.82rem;
    font-weight: 700;
  }

  .toast-icon {
    font-size: 1rem;
  }

  .stream-video-wrapper {
    position: relative;
    width: 100%;
    height: 280px;
    background: #000;
    border-radius: 4px;
    overflow: hidden;
    border: 1.5px solid var(--wc-text, #2c2523);
  }

  .stream-video-feed {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .stream-overlay-canvas {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    z-index: 2;
  }

  .stream-floating-tools {
    position: absolute;
    top: 10px;
    right: 10px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 5;
  }

  .btn-float-ctrl {
    width: 38px;
    height: 38px;
    border-radius: 50%;
    background: rgba(0, 0, 0, 0.65);
    border: 1.5px solid #ffffff;
    color: #ffffff;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.15s ease;
  }

  .btn-float-ctrl:hover {
    background: rgba(0, 0, 0, 0.85);
  }

  .corner-guide-hint {
    position: absolute;
    bottom: 8px;
    left: 8px;
    right: 8px;
    background: rgba(0, 0, 0, 0.75);
    color: #ffffff;
    padding: 4px 8px;
    border-radius: 4px;
    font-size: 0.75rem;
    text-align: center;
    pointer-events: none;
    z-index: 4;
  }

  .stream-controls-bar {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .btn-stream-action {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 8px 14px;
    font-size: 0.84rem;
    font-weight: 700;
    border-radius: 3px;
    border: 1.5px solid var(--wc-text, #2c2523);
    box-shadow: 2px 2px 0px var(--wc-text, #2c2523);
    transition: transform 0.1s ease;
  }

  .btn-stream-action:active {
    transform: translate(1px, 1px);
    box-shadow: 1px 1px 0px var(--wc-text, #2c2523);
  }

  .btn-start-rec {
    background: #d32f2f;
    color: #ffffff;
    flex: 1;
    min-width: 140px;
    justify-content: center;
  }

  .btn-pause-rec {
    background: #f57c00;
    color: #ffffff;
    flex: 1;
    min-width: 140px;
    justify-content: center;
  }

  .btn-sec {
    background: var(--wc-surface, #ffffff);
    color: var(--wc-text, #2c2523);
  }

  .btn-finish-import {
    background: #2e7d32;
    color: #ffffff;
    flex-basis: 100%;
    justify-content: center;
    margin-top: 4px;
  }

  .btn-stream-action:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    transform: none;
    box-shadow: none;
  }

  .live-status-card {
    background: var(--wc-surface, #ffffff);
    border: 1.5px solid var(--wc-text, #2c2523);
    border-radius: 4px;
    padding: 10px 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .live-status-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.85rem;
    font-weight: 600;
  }

  .live-status-title {
    display: flex;
    align-items: center;
    gap: 4px;
    color: var(--wc-text, #2c2523);
  }

  .live-stones-count {
    font-size: 0.8rem;
    color: var(--wc-text-muted, #7c6b52);
  }

  .mini-svg-board-container {
    width: 100%;
    max-width: 280px;
    margin: 0 auto;
    border: 1px solid #8b5a2b;
    border-radius: 4px;
    overflow: hidden;
  }

  .mini-board-svg {
    width: 100%;
    height: auto;
    display: block;
  }

  .animate-pulse {
    animation: pulse 1.2s infinite ease-in-out;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.65; }
  }
</style>
