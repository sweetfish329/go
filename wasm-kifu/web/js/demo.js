/**
 * wasm-kifu デモページ メインスクリプト
 *
 * 碁盤描画・ユーザー操作ハンドリング・カメラ連携・SGFエクスポート
 */
import { WasmKifu } from './wasm-bridge.js';
import { BoardDetector } from './board-detector.js';

// ======================
// 初期化
// ======================
const kifu = new WasmKifu();
const detector = new BoardDetector();

let cameraMode = false;
let cameraStream = null;
let cornerPoints = [];

// DOM 要素
const boardCanvas = document.getElementById('board-canvas');
const statusEl = document.getElementById('status');

// ======================
// 起動
// ======================
async function init() {
    try {
        // WASM ロード (キャッシュバスター付き)
        setStatus('WASM をロード中...', 'loading');
        await kifu.load(`wasm/kifu.wasm?t=${Date.now()}`);
        setStatus('WASM ✓ / OpenCV.js ロード中...', 'loading');

        // 新規ゲーム開始
        kifu.newGame();
        drawBoard();

        // OpenCV.js の非同期ロード待機
        detector.waitForOpenCV().then(() => {
            document.getElementById('btn-camera').disabled = false;
            setStatus('準備完了 ✓', 'ready');
        }).catch(() => {
            setStatus('WASM ✓ / OpenCV.js ロード失敗（カメラ機能無効）', 'error');
        });
    } catch (err) {
        setStatus(`エラー: ${err.message}`, 'error');
        console.error('Init error:', err);
    }
}

function setStatus(message, type) {
    statusEl.textContent = message;
    statusEl.className = `status-${type}`;
}

// ======================
// 碁盤描画
// ======================
function drawBoard() {
    const state = kifu.getBoardState();
    const ctx = boardCanvas.getContext('2d');
    const size = boardCanvas.width;
    const padding = 15;
    const gridSize = size - 2 * padding;
    const spacing = gridSize / 18;
    const lastMove = kifu.getLastMoveInfo();

    // 背景（碁盤の色）
    const gradient = ctx.createRadialGradient(
        size / 2, size / 2, 0,
        size / 2, size / 2, size * 0.7
    );
    gradient.addColorStop(0, '#dbb268');
    gradient.addColorStop(1, '#c9993a');
    ctx.fillStyle = gradient;
    ctx.fillRect(0, 0, size, size);

    // 木目テクスチャ（軽量シミュレーション）
    ctx.strokeStyle = 'rgba(160, 120, 60, 0.15)';
    ctx.lineWidth = 0.5;
    for (let i = 0; i < size; i += 7) {
        ctx.beginPath();
        ctx.moveTo(0, i + Math.sin(i * 0.05) * 3);
        ctx.lineTo(size, i + Math.sin(i * 0.05 + 2) * 3);
        ctx.stroke();
    }

    // 線
    ctx.strokeStyle = '#2a2018';
    ctx.lineWidth = 1;
    for (let i = 0; i < 19; i++) {
        const pos = padding + i * spacing;
        // 横線
        ctx.beginPath();
        ctx.moveTo(padding, pos);
        ctx.lineTo(size - padding, pos);
        ctx.stroke();
        // 縦線
        ctx.beginPath();
        ctx.moveTo(pos, padding);
        ctx.lineTo(pos, size - padding);
        ctx.stroke();
    }

    // 外枠を太く
    ctx.lineWidth = 2;
    ctx.strokeRect(padding, padding, gridSize, gridSize);

    // 星（ほし）
    const starPoints = [3, 9, 15];
    ctx.fillStyle = '#2a2018';
    for (const sx of starPoints) {
        for (const sy of starPoints) {
            ctx.beginPath();
            ctx.arc(padding + sx * spacing, padding + sy * spacing, 3.5, 0, Math.PI * 2);
            ctx.fill();
        }
    }

    // 石を描画
    const stoneRadius = spacing * 0.46;
    for (let x = 0; x < 19; x++) {
        for (let y = 0; y < 19; y++) {
            const stone = state.stones[x][y];
            if (stone === 0) continue;

            const cx = padding + x * spacing;
            const cy = padding + y * spacing;

            if (stone === 1) {
                // 黒石
                drawBlackStone(ctx, cx, cy, stoneRadius);
            } else {
                // 白石
                drawWhiteStone(ctx, cx, cy, stoneRadius);
            }
        }
    }

    // 最終手マーカー
    if (lastMove && lastMove.moveNumber > 0 && !lastMove.isPass) {
        const mcx = padding + lastMove.x * spacing;
        const mcy = padding + lastMove.y * spacing;
        const markerColor = lastMove.color === 'B' ? '#fff' : '#111';
        ctx.strokeStyle = markerColor;
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.arc(mcx, mcy, stoneRadius * 0.45, 0, Math.PI * 2);
        ctx.stroke();
    }

    // 情報表示を更新
    updateGameInfo(state);
}

function drawBlackStone(ctx, cx, cy, r) {
    // グラデーションで立体感
    const grad = ctx.createRadialGradient(cx - r * 0.3, cy - r * 0.3, r * 0.1, cx, cy, r);
    grad.addColorStop(0, '#555');
    grad.addColorStop(0.6, '#222');
    grad.addColorStop(1, '#000');
    // 影
    ctx.beginPath();
    ctx.arc(cx + 1.5, cy + 1.5, r, 0, Math.PI * 2);
    ctx.fillStyle = 'rgba(0,0,0,0.3)';
    ctx.fill();
    // 石本体
    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.fillStyle = grad;
    ctx.fill();
}

function drawWhiteStone(ctx, cx, cy, r) {
    const grad = ctx.createRadialGradient(cx - r * 0.3, cy - r * 0.3, r * 0.1, cx, cy, r);
    grad.addColorStop(0, '#fff');
    grad.addColorStop(0.8, '#e8e8e8');
    grad.addColorStop(1, '#ccc');
    // 影
    ctx.beginPath();
    ctx.arc(cx + 1.5, cy + 1.5, r, 0, Math.PI * 2);
    ctx.fillStyle = 'rgba(0,0,0,0.25)';
    ctx.fill();
    // 石本体
    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.fillStyle = grad;
    ctx.fill();
    ctx.strokeStyle = '#aaa';
    ctx.lineWidth = 0.5;
    ctx.stroke();
}

function updateGameInfo(state) {
    const playerEl = document.getElementById('current-player');
    const turnLabel = document.getElementById('turn-label');

    if (state.resigned) {
        playerEl.textContent = '🏳';
        playerEl.className = 'player-indicator';
        turnLabel.textContent = '終局';
    } else if (state.player === 1) {
        playerEl.textContent = '●';
        playerEl.className = 'player-indicator black';
        turnLabel.textContent = '黒番';
    } else {
        playerEl.textContent = '○';
        playerEl.className = 'player-indicator white';
        turnLabel.textContent = '白番';
    }

    document.getElementById('move-number').textContent = state.moveNumber;
    document.getElementById('captures-info').textContent =
        `⚫${state.capturedByBlack} ⚪${state.capturedByWhite}`;
}

// ======================
// Toast通知
// ======================
function showToast(message) {
    const existing = document.querySelector('.toast');
    if (existing) existing.remove();

    const toast = document.createElement('div');
    toast.className = 'toast';
    toast.textContent = message;
    document.body.appendChild(toast);
    setTimeout(() => toast.remove(), 3000);
}

// ======================
// 碁盤クリック → 着手
// ======================
boardCanvas.addEventListener('click', (e) => {
    if (cameraMode) return;

    const rect = boardCanvas.getBoundingClientRect();
    const scaleX = boardCanvas.width / rect.width;
    const scaleY = boardCanvas.height / rect.height;
    const px = (e.clientX - rect.left) * scaleX;
    const py = (e.clientY - rect.top) * scaleY;

    const padding = 15;
    const spacing = (boardCanvas.width - 2 * padding) / 18;
    const x = Math.round((px - padding) / spacing);
    const y = Math.round((py - padding) / spacing);

    if (x < 0 || x > 18 || y < 0 || y > 18) return;

    const result = kifu.playMove(x, y);
    if (result === 0) {
        drawBoard();
    } else {
        showToast('❌ 違法手です');
    }
});

// ======================
// ボタンイベント
// ======================
document.getElementById('btn-new-game').addEventListener('click', () => {
    kifu.newGame();
    drawBoard();
    document.getElementById('sgf-output').classList.add('hidden');
    showToast('🆕 新しい対局を開始しました');
});

document.getElementById('btn-pass').addEventListener('click', () => {
    kifu.pass();
    drawBoard();
    showToast('⏭ パスしました');
});

document.getElementById('btn-undo').addEventListener('click', () => {
    if (kifu.undo()) {
        drawBoard();
        showToast('↩ 1手戻しました');
    } else {
        showToast('⚠ これ以上戻せません');
    }
});

document.getElementById('btn-resign').addEventListener('click', () => {
    const state = kifu.getBoardState();
    if (state.resigned) {
        showToast('⚠ すでに終局しています');
        return;
    }
    kifu.resign();
    drawBoard();
    showToast('🏳 投了しました');
});

// SGF 出力
document.getElementById('btn-export').addEventListener('click', () => {
    const sgf = kifu.exportSGF();
    document.getElementById('sgf-text').value = sgf;
    document.getElementById('sgf-output').classList.remove('hidden');
});

document.getElementById('btn-close-sgf').addEventListener('click', () => {
    document.getElementById('sgf-output').classList.add('hidden');
});

document.getElementById('btn-copy-sgf').addEventListener('click', () => {
    const text = document.getElementById('sgf-text').value;
    navigator.clipboard.writeText(text).then(() => {
        showToast('📋 SGFをコピーしました');
    }).catch(() => {
        // フォールバック
        document.getElementById('sgf-text').select();
        document.execCommand('copy');
        showToast('📋 SGFをコピーしました');
    });
});

document.getElementById('btn-download-sgf').addEventListener('click', () => {
    const sgf = document.getElementById('sgf-text').value;
    const blob = new Blob([sgf], { type: 'application/x-go-sgf' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    const date = new Date().toISOString().slice(0, 10);
    const moveNum = kifu.getMoveNumber();
    a.download = `kifu_${date}_${moveNum}手.sgf`;
    a.click();
    URL.revokeObjectURL(a.href);
    showToast('💾 SGFをダウンロードしました');
});

// ======================
// ======================
// カメラ制御 & リアルタイム自動追跡
// ======================
let streamMode = false;
let streamAnimationId = null;

document.getElementById('btn-camera').addEventListener('click', async () => {
    cameraMode = !cameraMode;
    const cameraSection = document.getElementById('camera-section');
    const autoDetectBtn = document.getElementById('btn-auto-detect');
    const streamBtn = document.getElementById('btn-stream-mode');
    const captureBtn = document.getElementById('btn-capture');
    const resetBtn = document.getElementById('btn-reset-corners');
    const cameraBtn = document.getElementById('btn-camera');

    if (cameraMode) {
        imageMode = false;
        const video = document.getElementById('camera-video');
        const img = document.getElementById('camera-image');
        video.classList.remove('hidden');
        img.classList.add('hidden');

        cameraSection.classList.remove('hidden');
        autoDetectBtn.classList.remove('hidden');
        streamBtn.classList.remove('hidden');
        captureBtn.classList.remove('hidden');
        resetBtn.classList.remove('hidden');
        cameraBtn.textContent = '📷 カメラ閉じる';

        try {
            cameraStream = await detector.startCamera(video);
            cornerPoints = [];
            updateCornerDots();
            resetAllDebugStages();
        } catch (err) {
            showToast(`📷 カメラエラー: ${err.message}`);
            cameraMode = false;
            imageMode = false;
            cameraSection.classList.add('hidden');
            autoDetectBtn.classList.add('hidden');
            streamBtn.classList.add('hidden');
            captureBtn.classList.add('hidden');
            resetBtn.classList.add('hidden');
            cameraBtn.textContent = '📷 カメラ';
        }
    } else {
        closeCameraMode();
    }
});

function closeCameraMode() {
    stopStreamTracking();
    cameraMode = false;
    imageMode = false;
    resetAllDebugStages();
    document.getElementById('camera-section').classList.add('hidden');
    document.getElementById('btn-auto-detect').classList.add('hidden');
    document.getElementById('btn-stream-mode').classList.add('hidden');
    document.getElementById('btn-capture').classList.add('hidden');
    document.getElementById('btn-reset-corners').classList.add('hidden');
    document.getElementById('stream-indicator').classList.add('hidden');
    document.getElementById('btn-camera').textContent = '📷 カメラ';
    document.getElementById('camera-video').classList.remove('hidden');
    document.getElementById('camera-image').classList.add('hidden');

    if (cameraStream) {
        detector.stopCamera(cameraStream);
        cameraStream = null;
    }
}

let imageMode = false;

// サンプル画像 (test_image.jpg) 読み込み
document.getElementById('btn-load-sample').addEventListener('click', () => {
    loadImageSource('test_image.jpg');
});

// 任意の画像ファイル選択
document.getElementById('btn-upload-image').addEventListener('click', () => {
    document.getElementById('file-input').click();
});

document.getElementById('file-input').addEventListener('change', (e) => {
    const file = e.target.files[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (event) => {
        loadImageSource(event.target.result);
    };
    reader.readAsDataURL(file);
});

function loadImageSource(src) {
    if (cameraStream) {
        detector.stopCamera(cameraStream);
        cameraStream = null;
    }
    stopStreamTracking();
    resetAllDebugStages();

    cameraMode = true;
    imageMode = true;

    const cameraSection = document.getElementById('camera-section');
    const autoDetectBtn = document.getElementById('btn-auto-detect');
    const streamBtn = document.getElementById('btn-stream-mode');
    const captureBtn = document.getElementById('btn-capture');
    const resetBtn = document.getElementById('btn-reset-corners');
    const cameraBtn = document.getElementById('btn-camera');
    const video = document.getElementById('camera-video');
    const img = document.getElementById('camera-image');

    cameraSection.classList.remove('hidden');
    autoDetectBtn.classList.remove('hidden');
    streamBtn.classList.add('hidden'); // 静止画ではストリーム追跡不要
    captureBtn.classList.remove('hidden');
    resetBtn.classList.remove('hidden');
    cameraBtn.textContent = '📷 カメラ起動';

    video.classList.add('hidden');
    img.classList.remove('hidden');
    img.src = src;

    img.onload = async () => {
        cornerPoints = [];
        detector.resetCorners();
        updateCornerDots();
        drawCornerMarkers();

        // WASM のロード完了を最大3秒待機（初期化中のフォールバック防止）
        if (!kifu.isReady()) {
            showToast('⏳ WASM エンジン初期化待機中...');
            for (let i = 0; i < 30 && !kifu.isReady(); i++) {
                await new Promise((r) => setTimeout(r, 100));
            }
        }

        // 高精度エンジンによる四隅自動検出を試行
        try {
            const detected = detectCornersFromSource(img);
            if (detected && detected.length === 4) {
                cornerPoints = detected;
                detector.setCorners(cornerPoints);
                updateCornerDots();
                drawCornerMarkers();
            } else {
                showToast('🖼 画像を読み込みました。「🎯自動検出」または4隅タップで開始');
            }
        } catch (err) {
            console.warn('Auto detect notice:', err);
            showToast('🖼 画像を読み込みました。「🎯自動検出」または4隅タップで開始');
        }
    };
}

// 4隅の自動検出 (Go WASM 高精度 RANSAC / 天板ブロブ追跡 / シャドウライン自己修復エンジン)
function detectCornersFromSource(source) {
    let width, height;
    if (source instanceof HTMLImageElement) {
        width = source.naturalWidth || source.width;
        height = source.naturalHeight || source.height;
    } else if (source instanceof HTMLVideoElement) {
        width = source.videoWidth || source.width;
        height = source.videoHeight || source.height;
    } else {
        width = source.width;
        height = source.height;
    }

    if (!width || !height) {
        return null;
    }

    // オフスクリーン Canvas で RGBA データを取得
    const offscreen = document.createElement('canvas');
    offscreen.width = width;
    offscreen.height = height;
    const ctx = offscreen.getContext('2d', { willReadFrequently: true });
    ctx.drawImage(source, 0, 0, width, height);
    const imgData = ctx.getImageData(0, 0, width, height);
    console.log(`[Corners] Image source dimensions: ${width}x${height}`);

    // 1. Go WASM による高精度・ロバスト検出（動的マージン実測、RANSAC、ブロブ追跡、シャドウライン補正）
    if (kifu.isReady()) {
        try {
            console.log('[Corners] 🚀 Running Go WASM detectBoardCorners...');
            const res = kifu.detectBoardCorners(imgData.data, width, height);
            console.log('[Corners] Go WASM response:', res);
            if (res && res.corners && res.corners.length === 4) {
                console.log('[Corners] ✅ Go WASM detected corners successfully:', res.corners);
                showToast('🎯 碁盤の4隅を自動検出しました (Go WASM 動的マージン)');
                updateDebugCorners(offscreen, res);
                return res.corners;
            }
            if (res && res.error) {
                console.warn('[Corners] ❌ Go WASM returned error:', res.error);
            }
        } catch (err) {
            console.warn('[Corners] ❌ Wasm detectBoardCorners exception:', err);
        }
    } else {
        console.warn('[Corners] ⚠️ kifu is not ready yet');
    }

    // 2. フォールバック: OpenCV.js
    if (detector.isReady()) {
        try {
            console.log('[Corners] 🔄 Falling back to OpenCV.js autoDetectCorners...');
            const detected = detector.autoDetectCorners(offscreen);
            console.log('[Corners] OpenCV.js result:', detected);
            if (detected && detected.length === 4) {
                showToast('🎯 碁盤の4隅を自動検出しました (OpenCV.js)');
                updateDebugCorners(offscreen, {
                    corners: detected,
                    method: 'OpenCV.js',
                });
                return detected;
            }
        } catch (err) {
            console.warn('[Corners] OpenCV autoDetectCorners notice:', err);
        }
    } else {
        console.warn('[Corners] detector (OpenCV.js) is not ready');
    }

    return null;
}

// 4隅の自動検出ボタン
document.getElementById('btn-auto-detect').addEventListener('click', () => {
    const source = imageMode ? document.getElementById('camera-image') : document.getElementById('camera-video');
    let canvasSource = source;

    if (!imageMode) {
        const canvas = document.getElementById('camera-canvas');
        detector.captureFrame(source, canvas);
        canvasSource = canvas;
    }

    try {
        const detected = detectCornersFromSource(canvasSource);
        if (detected && detected.length === 4) {
            cornerPoints = detected;
            detector.setCorners(cornerPoints);
            updateCornerDots();
            drawCornerMarkers();
            showToast('🎯 碁盤の4隅を高精度自動検出しました！');
        } else {
            showToast('⚠ 自動検出できませんでした。手動で4隅をタップしてください');
        }
    } catch (err) {
        console.error('Auto detect error:', err);
        showToast(`⚠ 自動検出エラー: ${err.message}`);
    }
});

function handleCornerClick(e, elem, naturalWidth, naturalHeight) {
    if (!cameraMode || cornerPoints.length >= 4) return;

    const rect = elem.getBoundingClientRect();
    const scaleX = naturalWidth / rect.width;
    const scaleY = naturalHeight / rect.height;

    const point = {
        x: (e.clientX - rect.left) * scaleX,
        y: (e.clientY - rect.top) * scaleY,
    };
    cornerPoints.push(point);
    if (cornerPoints.length === 1) {
        resetStage3And4();
    }
    updateCornerDots();
    drawCornerMarkers();

    if (cornerPoints.length === 4) {
        const source = imageMode ? document.getElementById('camera-image') : document.getElementById('camera-video');
        const refined = detector.setCorners(cornerPoints, source);
        if (refined && refined.length === 4) {
            cornerPoints = refined;
            drawCornerMarkers();
        }
        showToast('✅ 4隅を設定・高精度吸着しました。「📸認識実行」で認識');

        // 手動指定四隅をデバッグ表示へ反映
        const offscreen = document.createElement('canvas');
        offscreen.width = naturalWidth;
        offscreen.height = naturalHeight;
        offscreen.getContext('2d').drawImage(source, 0, 0, naturalWidth, naturalHeight);
        updateDebugCorners(offscreen, {
            corners: cornerPoints,
            method: 'Manual Tap + Förstner',
        });
    }
}

// 4隅タップ (Video)
document.getElementById('camera-video').addEventListener('click', (e) => {
    const video = e.target;
    handleCornerClick(e, video, video.videoWidth, video.videoHeight);
});

// 4隅タップ (Image)
document.getElementById('camera-image').addEventListener('click', (e) => {
    const img = e.target;
    handleCornerClick(e, img, img.naturalWidth, img.naturalHeight);
});


function updateCornerDots() {
    const dots = document.querySelectorAll('.corner-dot');
    dots.forEach((dot, i) => {
        dot.classList.toggle('active', i < cornerPoints.length);
    });
}

function drawCornerMarkers() {
    const targetElem = imageMode ? document.getElementById('camera-image') : document.getElementById('camera-video');
    const overlay = document.getElementById('corner-overlay-canvas');
    overlay.width = targetElem.clientWidth;
    overlay.height = targetElem.clientHeight;

    const ctx = overlay.getContext('2d');
    ctx.clearRect(0, 0, overlay.width, overlay.height);

    const naturalWidth = imageMode ? targetElem.naturalWidth : targetElem.videoWidth;
    const naturalHeight = imageMode ? targetElem.naturalHeight : targetElem.videoHeight;

    const scaleX = targetElem.clientWidth / naturalWidth;
    const scaleY = targetElem.clientHeight / naturalHeight;

    // マーカーを描画
    cornerPoints.forEach((p, i) => {
        const x = p.x * scaleX;
        const y = p.y * scaleY;


        ctx.beginPath();
        ctx.arc(x, y, 8, 0, Math.PI * 2);
        ctx.fillStyle = 'rgba(233, 69, 96, 0.7)';
        ctx.fill();
        ctx.strokeStyle = '#fff';
        ctx.lineWidth = 2;
        ctx.stroke();

        // 番号
        ctx.fillStyle = '#fff';
        ctx.font = 'bold 10px Inter';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(i + 1, x, y);
    });

    // 線で結ぶ
    if (cornerPoints.length > 1) {
        ctx.strokeStyle = 'rgba(233, 69, 96, 0.5)';
        ctx.lineWidth = 1;
        ctx.setLineDash([5, 5]);
        ctx.beginPath();
        const first = cornerPoints[0];
        ctx.moveTo(first.x * scaleX, first.y * scaleY);
        for (let i = 1; i < cornerPoints.length; i++) {
            ctx.lineTo(cornerPoints[i].x * scaleX, cornerPoints[i].y * scaleY);
        }
        if (cornerPoints.length === 4) {
            ctx.closePath();
        }
        ctx.stroke();
        ctx.setLineDash([]);
    }
}

// 4隅リセット
document.getElementById('btn-reset-corners').addEventListener('click', () => {
    stopStreamTracking();
    cornerPoints = [];
    detector.resetCorners();
    updateCornerDots();
    resetAllDebugStages();
    // オーバーレイクリア
    const overlay = document.getElementById('corner-overlay-canvas');
    const ctx = overlay.getContext('2d');
    ctx.clearRect(0, 0, overlay.width, overlay.height);
    showToast('🔄 4隅をリセットしました');
});

// ======================
// リアルタイム自動追跡 (Stream Mode)
// ======================
const streamBtn = document.getElementById('btn-stream-mode');
const streamIndicator = document.getElementById('stream-indicator');
const stabilityBar = document.getElementById('stability-bar');
const stabilityPercent = document.getElementById('stability-percent');
const motionStatusBadge = document.getElementById('motion-status-badge');

streamBtn.addEventListener('click', () => {
    if (!detector.hasCornersSet()) {
        showToast('⚠ 碁盤の4隅を先に指定してください');
        return;
    }

    streamMode = !streamMode;
    if (streamMode) {
        startStreamTracking();
    } else {
        stopStreamTracking();
    }
});

function startStreamTracking() {
    streamMode = true;
    streamBtn.textContent = '⚡ 自動記録: ON';
    streamBtn.className = 'btn btn-success';
    streamIndicator.classList.remove('hidden');
    detector.initStreamTracker();
    showToast('⚡ リアルタイム自動着手検知を開始しました');
    streamLoop();
}

function stopStreamTracking() {
    streamMode = false;
    streamBtn.textContent = '⚡ 自動記録: OFF';
    streamBtn.className = 'btn btn-accent';
    streamIndicator.classList.add('hidden');
    if (streamAnimationId) {
        cancelAnimationFrame(streamAnimationId);
        streamAnimationId = null;
    }
    detector.resetStreamTracker();
}

let lastFrameTime = 0;
function streamLoop(timestamp) {
    if (!streamMode || !cameraMode) return;

    // 約 20fps にスロットリング（過剰なCPU消費を防止）
    if (!timestamp || timestamp - lastFrameTime > 50) {
        lastFrameTime = timestamp || 0;
        const video = document.getElementById('camera-video');

        if (video && video.readyState >= 2) {
            try {
                const res = detector.processStreamFrame(video);
                
                // UI インジケーター更新
                const percent = Math.round(res.stableProgress * 100);
                stabilityBar.style.width = `${percent}%`;
                stabilityPercent.textContent = `${percent}%`;

                if (res.isStable) {
                    motionStatusBadge.textContent = '🕊 盤面静止中';
                    motionStatusBadge.className = 'badge stable';
                } else {
                    motionStatusBadge.textContent = `🖐 動体検知中 (${res.motionLevel})`;
                    motionStatusBadge.className = 'badge';
                }

                // 静止安定トリガーが発火した瞬間
                if (res.isTriggered && res.features) {
                    processDetectedFeatures(res.features, true);
                }
            } catch (err) {
                console.warn('Stream processing warning:', err);
            }
        }
    }

    streamAnimationId = requestAnimationFrame(streamLoop);
}

// 多次元特徴量から着手を適用
function processDetectedFeatures(features, isAutoTriggered = false) {
    try {
        const analysis = kifu.analyzeBoardFeatures(features);

        if (analysis.moves && analysis.moves.length === 1) {
            const move = analysis.moves[0];
            const colorNum = move.color === 'B' ? 1 : 2;
            const result = kifu.applyDetectedMove(move.x, move.y, colorNum);

            if (result === 0) {
                drawBoard();
                const colorName = move.color === 'B' ? '黒' : '白';
                const prefix = isAutoTriggered ? '⚡ [自動検知]' : '✅';
                showToast(`${prefix} ${colorName} (${move.x},${move.y})`);
            } else {
                if (!isAutoTriggered) {
                    showToast('❌ 検出された着手は違法手です');
                }
            }
        } else if (analysis.moves && analysis.moves.length > 1) {
            if (!isAutoTriggered) {
                showToast(`⚠ ${analysis.moves.length}個の変化を検出。手動で確認してください`);
            }
        } else {
            if (!isAutoTriggered) {
                showToast('ℹ 変化が検出されませんでした');
            }
        }

        if (analysis.errors && analysis.errors.length > 0 && !isAutoTriggered) {
            console.warn('Detection warnings:', analysis.errors);
        }
    } catch (err) {
        console.error('Feature analysis error:', err);
        if (!isAutoTriggered) {
            showToast(`❌ 認識エラー: ${err.message}`);
        }
    }
}

// 認識実行 (撮影 / 静止画)
document.getElementById('btn-capture').addEventListener('click', () => {
    if (!detector.hasCornersSet()) {
        showToast('⚠ 碁盤の4隅を先にタップまたは「🎯自動検出」してください');
        return;
    }

    let source = document.getElementById('camera-image');
    if (!imageMode) {
        const video = document.getElementById('camera-video');
        const canvas = document.getElementById('camera-canvas');
        detector.captureFrame(video, canvas);
        source = canvas;
    }

    try {
        // 多次元特徴量抽出 (ドーナツ走査 + 距離変換スナッピング + 局所背景適応)
        const features = detector.extractFeatures(source);

        if (imageMode) {
            // 静止画モード: 全盤面 (361交点) の黒石・白石・空交点を一括高精度解析 & 復元
            let fullBoard = null;
            if (kifu.isReady()) {
                try {
                    fullBoard = kifu.analyzeFullBoard(features);
                } catch (err) {
                    console.warn('WASM analyzeFullBoard error:', err);
                }
            }

            if (fullBoard && fullBoard.grid && fullBoard.grid.length === 361) {
                kifu.setBoardFromGrid(fullBoard.grid);
                drawBoard();
                const bCount = fullBoard.blackCount ?? 0;
                const wCount = fullBoard.whiteCount ?? 0;
                const conf = fullBoard.confidence ? Math.round(fullBoard.confidence * 100) : 100;
                showToast(`✅ 全盤面を認識しました！ (黒: ${bCount}個, 白: ${wCount}個 / 信頼度: ${conf}%)`);

                // デバッグ用: 確定四隅による歪み補正正面画像と全361交点判定オーバーレイを更新
                try {
                    updateDebugWarpAndClassificationFromSource(source, fullBoard.grid);
                } catch (e) {
                    console.warn('Debug warp update notice:', e);
                }

                return;
            }

            // フォールバック: OpenCV.js 直接分類
            if (detector.isReady()) {
                const classified = detector.classifyBoard(source);
                if (classified && classified.length === 361) {
                    kifu.setBoardFromGrid(classified);
                    drawBoard();
                    let b = 0, w = 0;
                    for (const s of classified) {
                        if (s === 1) b++;
                        else if (s === 2) w++;
                    }
                    showToast(`✅ 盤面を認識しました！ (黒: ${b}個, 白: ${w}個)`);

                    // デバッグ用: 確定四隅による補正画像と判定オーバーレイを更新
                    try {
                        updateDebugWarpAndClassificationFromSource(source, classified);
                    } catch (e) {
                        console.warn('Debug opencv update notice:', e);
                    }

                    return;
                }
            }
        }

        // カメラ動画モード: 直前手からの1手差分着手を判定
        processDetectedFeatures(features, false);
        try {
            const currentGrid = kifu.getBoardState();
            if (currentGrid && currentGrid.length === 361) {
                updateDebugWarpAndClassificationFromSource(source, currentGrid);
            }
        } catch (e) {
            console.warn('Camera debug warp update notice:', e);
        }
    } catch (err) {
        showToast(`❌ 認識エラー: ${err.message}`);
        console.error('Detection error:', err);
    }
});


// ======================
// 中間生成物デバッグ表示機能 (Debug Stages Viewer)
// ======================
let debugMode = false;
const debugState = {
    sourceCanvas: null,
    cornerRes: null,
    warpedRes: null,
    classifiedGrid: null,
};

const debugSection = document.getElementById('debug-section');
const btnDebugToggle = document.getElementById('btn-debug-toggle');
const debugModal = document.getElementById('debug-modal');
const debugModalCanvas = document.getElementById('debug-modal-canvas');
const debugModalTitle = document.getElementById('debug-modal-title');

// デバッグトグルボタン
btnDebugToggle.addEventListener('click', () => {
    debugMode = !debugMode;
    if (debugMode) {
        btnDebugToggle.classList.add('btn-accent');
        btnDebugToggle.textContent = '🛠 デバッグ: ON';
        debugSection.classList.remove('hidden');
        renderAllDebugStages();
    } else {
        btnDebugToggle.classList.remove('btn-accent');
        btnDebugToggle.textContent = '🛠 デバッグ';
        debugSection.classList.add('hidden');
    }
});

// Stage 3 & 4 のデバッグ表示および保持ステートのリセット
function resetStage3And4() {
    debugState.warpedRes = null;
    debugState.classifiedGrid = null;

    const canvasWarped = document.getElementById('debug-canvas-warped');
    const metaWarped = document.getElementById('debug-warped-meta');
    const placeholderWarped = document.getElementById('debug-warped-placeholder');
    if (canvasWarped) {
        const ctx = canvasWarped.getContext('2d');
        ctx.clearRect(0, 0, canvasWarped.width, canvasWarped.height);
        canvasWarped.width = 0;
        canvasWarped.height = 0;
    }
    if (metaWarped) metaWarped.textContent = '-';
    if (placeholderWarped) placeholderWarped.style.display = 'block';

    const canvasClassified = document.getElementById('debug-canvas-classified');
    const metaClassified = document.getElementById('debug-classified-meta');
    const placeholderClassified = document.getElementById('debug-classified-placeholder');
    if (canvasClassified) {
        const ctx = canvasClassified.getContext('2d');
        ctx.clearRect(0, 0, canvasClassified.width, canvasClassified.height);
        canvasClassified.width = 0;
        canvasClassified.height = 0;
    }
    if (metaClassified) metaClassified.textContent = '-';
    if (placeholderClassified) placeholderClassified.style.display = 'block';
}

// 全ステージのデバッグリセット (画像切り替え時や4隅リセット時)
function resetAllDebugStages() {
    debugState.sourceCanvas = null;
    debugState.cornerRes = null;
    resetStage3And4();

    const canvasBlob = document.getElementById('debug-canvas-blob');
    const metaBlob = document.getElementById('debug-blob-meta');
    const placeholderBlob = document.getElementById('debug-blob-placeholder');
    if (canvasBlob) {
        const ctx = canvasBlob.getContext('2d');
        ctx.clearRect(0, 0, canvasBlob.width, canvasBlob.height);
        canvasBlob.width = 0;
        canvasBlob.height = 0;
    }
    if (metaBlob) metaBlob.textContent = '-';
    if (placeholderBlob) placeholderBlob.style.display = 'block';

    const canvasCorners = document.getElementById('debug-canvas-corners');
    const metaCorners = document.getElementById('debug-corners-meta');
    const placeholderCorners = document.getElementById('debug-corners-placeholder');
    if (canvasCorners) {
        const ctx = canvasCorners.getContext('2d');
        ctx.clearRect(0, 0, canvasCorners.width, canvasCorners.height);
        canvasCorners.width = 0;
        canvasCorners.height = 0;
    }
    if (metaCorners) metaCorners.textContent = '-';
    if (placeholderCorners) placeholderCorners.style.display = 'block';

    const methodBadge = document.getElementById('debug-method-badge');
    if (methodBadge) methodBadge.textContent = '手法: 未実行';
    const starsBadge = document.getElementById('debug-stars-badge');
    if (starsBadge) starsBadge.textContent = '星(花点): 0個';
    const confBadge = document.getElementById('debug-conf-badge');
    if (confBadge) confBadge.textContent = '信頼度: -';
}

// RGBA 配列から安全に Base64 文字列を生成するヘルパー（スタックオーバーフロー防止）
function rgbaDataToBase64(uint8Data) {
    let binary = '';
    const len = uint8Data.byteLength || uint8Data.length;
    const chunkSize = 16384;
    for (let i = 0; i < len; i += chunkSize) {
        const chunk = uint8Data.subarray ? uint8Data.subarray(i, Math.min(i + chunkSize, len)) : uint8Data.slice(i, i + chunkSize);
        binary += String.fromCharCode.apply(null, chunk);
    }
    return btoa(binary);
}

// RGBA Base64 文字列を Canvas に描画する汎用ヘルパー
function drawRgbaBase64ToCanvas(base64Data, width, height, canvas) {
    if (!base64Data || !width || !height || !canvas) return;
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext('2d');
    const binary = atob(base64Data);
    const imgData = ctx.createImageData(width, height);
    for (let i = 0; i < binary.length; i++) {
        imgData.data[i] = binary.charCodeAt(i);
    }
    ctx.putImageData(imgData, 0, 0);
}

// Stage 1: 天板ブロブ抽出マスクの描画
function renderStage1Blob(blobBase64, width, height) {
    const canvas = document.getElementById('debug-canvas-blob');
    const meta = document.getElementById('debug-blob-meta');
    const placeholder = document.getElementById('debug-blob-placeholder');

    if (!blobBase64 || !width || !height || !canvas) return;
    drawRgbaBase64ToCanvas(blobBase64, width, height, canvas);
    if (meta) meta.textContent = `${width}×${height} px`;
    if (placeholder) placeholder.style.display = 'none';
}

// Stage 2: 四隅・1線幾何検出オーバーレイの描画
function renderStage2Corners(sourceCanvas, cornerRes) {
    const canvas = document.getElementById('debug-canvas-corners');
    const meta = document.getElementById('debug-corners-meta');
    const placeholder = document.getElementById('debug-corners-placeholder');

    if (!sourceCanvas || !cornerRes || !cornerRes.corners || !canvas) return;

    canvas.width = sourceCanvas.width;
    canvas.height = sourceCanvas.height;
    const ctx = canvas.getContext('2d');

    // 1. 元画像を描画
    ctx.drawImage(sourceCanvas, 0, 0);

    // 2. 木枠外縁 (rawOuter) を青色破線で描画
    if (cornerRes.rawOuter && cornerRes.rawOuter.length === 4) {
        ctx.strokeStyle = '#3498db';
        ctx.lineWidth = Math.max(2, Math.round(canvas.width * 0.0025));
        ctx.setLineDash([8, 8]);
        ctx.beginPath();
        ctx.moveTo(cornerRes.rawOuter[0].x, cornerRes.rawOuter[0].y);
        for (let i = 1; i < 4; i++) {
            ctx.lineTo(cornerRes.rawOuter[i].x, cornerRes.rawOuter[i].y);
        }
        ctx.closePath();
        ctx.stroke();
        ctx.setLineDash([]);
    }

    // 3. 検出された星 (花点・天元) を金色星マークで描画
    if (cornerRes.stars && cornerRes.stars.length > 0) {
        cornerRes.stars.forEach(s => {
            const sx = s.imgX !== undefined ? s.imgX : (s.ImgX !== undefined ? s.ImgX : s.x);
            const sy = s.imgY !== undefined ? s.imgY : (s.ImgY !== undefined ? s.ImgY : s.y);
            if (sx !== undefined && sy !== undefined) {
                ctx.fillStyle = '#f1c40f';
                ctx.strokeStyle = '#2c3e50';
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.arc(sx, sy, Math.max(6, Math.round(canvas.width * 0.006)), 0, Math.PI * 2);
                ctx.fill();
                ctx.stroke();
            }
        });
    }

    // 4. 1線四隅 (corners) を赤色実線 ＆ 番号マーカーで描画
    const corners = cornerRes.corners;
    ctx.strokeStyle = '#e74c3c';
    ctx.lineWidth = Math.max(3, Math.round(canvas.width * 0.0035));
    ctx.beginPath();
    ctx.moveTo(corners[0].x, corners[0].y);
    for (let i = 1; i < 4; i++) {
        ctx.lineTo(corners[i].x, corners[i].y);
    }
    ctx.closePath();
    ctx.stroke();

    // コーナー番号バッジ
    const r = Math.max(12, Math.round(canvas.width * 0.012));
    corners.forEach((p, i) => {
        ctx.beginPath();
        ctx.arc(p.x, p.y, r, 0, Math.PI * 2);
        ctx.fillStyle = '#e74c3c';
        ctx.fill();
        ctx.strokeStyle = '#ffffff';
        ctx.lineWidth = 2;
        ctx.stroke();

        ctx.fillStyle = '#ffffff';
        ctx.font = `bold ${Math.round(r * 1.1)}px Inter, sans-serif`;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(i + 1, p.x, p.y);
    });

    const methodText = cornerRes.method || 'DirectGrid';
    if (meta) meta.textContent = `${methodText} (${canvas.width}×${canvas.height})`;
    if (placeholder) placeholder.style.display = 'none';

    // バッジ更新
    const methodBadge = document.getElementById('debug-method-badge');
    if (methodBadge) methodBadge.textContent = `手法: ${methodText}`;
    const starsBadge = document.getElementById('debug-stars-badge');
    if (starsBadge) starsBadge.textContent = `星: ${cornerRes.stars ? cornerRes.stars.length : 0}個`;
    const confBadge = document.getElementById('debug-conf-badge');
    if (confBadge && cornerRes.confidence) {
        confBadge.textContent = `信頼度: ${Math.round(cornerRes.confidence * 100)}%`;
    }
}

// Stage 3: 歪み補正後 正面碁盤画像の描画
function renderStage3Warped(warpedBase64, width, height) {
    const canvas = document.getElementById('debug-canvas-warped');
    const meta = document.getElementById('debug-warped-meta');
    const placeholder = document.getElementById('debug-warped-placeholder');

    if (!warpedBase64 || !width || !height || !canvas) return;
    drawRgbaBase64ToCanvas(warpedBase64, width, height, canvas);
    if (meta) meta.textContent = `${width}×${height} px (正面化)`;
    if (placeholder) placeholder.style.display = 'none';
}

// Stage 4: 全361交点 判定グリッドオーバーレイの描画
function renderStage4Classified(warpedBase64, width, height, grid, margin = 40) {
    const canvas = document.getElementById('debug-canvas-classified');
    const meta = document.getElementById('debug-classified-meta');
    const placeholder = document.getElementById('debug-classified-placeholder');

    if (!warpedBase64 || !grid || grid.length !== 361 || !canvas) return;

    // まず補正碁盤を下敷きとして描画
    drawRgbaBase64ToCanvas(warpedBase64, width, height, canvas);
    const ctx = canvas.getContext('2d');

    // 19x19 の交点座標を計算
    const gridW = width - 2 * margin;
    const gridH = height - 2 * margin;
    const pitchX = gridW / 18.0;
    const pitchY = gridH / 18.0;
    const stoneRadius = Math.min(pitchX, pitchY) * 0.44;

    let bCount = 0, wCount = 0, eCount = 0;

    for (let gy = 0; gy < 19; gy++) {
        for (let gx = 0; gx < 19; gx++) {
            const idx = gy * 19 + gx;
            const state = grid[idx];
            const cx = margin + gx * pitchX;
            const cy = margin + gy * pitchY;

            if (state === 1) {
                // 黒石 (Black): 半透明黒丸 + 白いリング
                bCount++;
                ctx.beginPath();
                ctx.arc(cx, cy, stoneRadius, 0, Math.PI * 2);
                ctx.fillStyle = 'rgba(20, 20, 25, 0.85)';
                ctx.fill();
                ctx.strokeStyle = '#ffffff';
                ctx.lineWidth = 1.5;
                ctx.stroke();
            } else if (state === 2) {
                // 白石 (White): 半透明白丸 + 黒いリング
                wCount++;
                ctx.beginPath();
                ctx.arc(cx, cy, stoneRadius, 0, Math.PI * 2);
                ctx.fillStyle = 'rgba(250, 250, 252, 0.88)';
                ctx.fill();
                ctx.strokeStyle = '#2c3e50';
                ctx.lineWidth = 1.5;
                ctx.stroke();
            } else {
                // 空点 (Empty): 半透明の緑色小ドット
                eCount++;
                ctx.beginPath();
                ctx.arc(cx, cy, 3, 0, Math.PI * 2);
                ctx.fillStyle = 'rgba(46, 204, 113, 0.7)';
                ctx.fill();
            }
        }
    }

    if (meta) meta.textContent = `黒:${bCount} 白:${wCount} 空:${eCount}`;
    if (placeholder) placeholder.style.display = 'none';
}

// デバッグ状態の更新ハンドラー (四隅検出時)
function updateDebugCorners(sourceCanvas, res) {
    debugState.sourceCanvas = sourceCanvas;
    debugState.cornerRes = res;
    if (debugMode) {
        if (res.blobMaskRgbaBase64 && res.blobWidth && res.blobHeight) {
            renderStage1Blob(res.blobMaskRgbaBase64, res.blobWidth, res.blobHeight);
        }
        renderStage2Corners(sourceCanvas, res);
    }
}

// 現在の入力画像と確定四隅から射影変換（正面碁盤）を生成し、Stage 3 と Stage 4 を確実に同期描画
function updateDebugWarpAndClassificationFromSource(source, grid) {
    debugState.classifiedGrid = grid;
    if (!detector.isReady() || !detector.hasCornersSet()) return;

    try {
        const srcMat = detector._readSourceMat(source);
        const warpedMat = detector._perspectiveTransform(srcMat);
        const warpedCanvas = document.createElement('canvas');
        cv.imshow(warpedCanvas, warpedMat);
        const ctx = warpedCanvas.getContext('2d');
        const imgData = ctx.getImageData(0, 0, warpedCanvas.width, warpedCanvas.height);

        // 安全なチャンクBase64エンコード
        const base64 = rgbaDataToBase64(imgData.data);

        const warpRes = {
            warpedRgbaBase64: base64,
            warpedWidth: warpedCanvas.width,
            warpedHeight: warpedCanvas.height,
            margin: 40,
        };
        debugState.warpedRes = warpRes;

        if (debugMode) {
            renderStage3Warped(warpRes.warpedRgbaBase64, warpRes.warpedWidth, warpRes.warpedHeight);
            renderStage4Classified(warpRes.warpedRgbaBase64, warpRes.warpedWidth, warpRes.warpedHeight, grid, warpRes.margin);
        }

        warpedMat.delete();
        srcMat.delete();
    } catch (e) {
        console.warn('Warp & classification debug render notice:', e);
    }
}

// デバッグ状態の更新ハンドラー (互換性維持用)
function updateDebugWarpAndClassification(warpRes, grid) {
    debugState.warpedRes = warpRes;
    debugState.classifiedGrid = grid;
    if (debugMode && warpRes) {
        renderStage3Warped(warpRes.warpedRgbaBase64, warpRes.warpedWidth, warpRes.warpedHeight);
        renderStage4Classified(warpRes.warpedRgbaBase64, warpRes.warpedWidth, warpRes.warpedHeight, grid, warpRes.margin || 40);
    }
}

// OpenCV.js フォールバック時の分類結果描画 (新共通関数へ委譲)
function updateDebugClassificationOpenCV(source, grid) {
    updateDebugWarpAndClassificationFromSource(source, grid);
}

// 全デバッグステージの再描画
function renderAllDebugStages() {
    if (debugState.cornerRes) {
        if (debugState.cornerRes.blobMaskRgbaBase64) {
            renderStage1Blob(
                debugState.cornerRes.blobMaskRgbaBase64,
                debugState.cornerRes.blobWidth,
                debugState.cornerRes.blobHeight
            );
        }
        if (debugState.sourceCanvas) {
            renderStage2Corners(debugState.sourceCanvas, debugState.cornerRes);
        }
    }
    if (debugState.warpedRes) {
        renderStage3Warped(
            debugState.warpedRes.warpedRgbaBase64,
            debugState.warpedRes.warpedWidth,
            debugState.warpedRes.warpedHeight
        );
        if (debugState.classifiedGrid) {
            renderStage4Classified(
                debugState.warpedRes.warpedRgbaBase64,
                debugState.warpedRes.warpedWidth,
                debugState.warpedRes.warpedHeight,
                debugState.classifiedGrid,
                debugState.warpedRes.margin || 40
            );
        }
    }
}

// カードクリックでモーダル拡大プレビュー
document.querySelectorAll('.debug-card').forEach(card => {
    card.addEventListener('click', () => {
        const stageCanvas = card.querySelector('canvas');
        const title = card.querySelector('h3').textContent;
        if (!stageCanvas || stageCanvas.width === 0) return;

        debugModalCanvas.width = stageCanvas.width;
        debugModalCanvas.height = stageCanvas.height;
        const mctx = debugModalCanvas.getContext('2d');
        mctx.drawImage(stageCanvas, 0, 0);
        debugModalTitle.textContent = title;
        debugModal.classList.remove('hidden');
    });
});

debugModal.addEventListener('click', () => {
    debugModal.classList.add('hidden');
});

// ======================
// 起動
// ======================
init();

// OpenCV.js ロード完了コールバック（グローバル）
window.onOpenCVReady = () => {
    console.log('OpenCV.js loaded');
};

