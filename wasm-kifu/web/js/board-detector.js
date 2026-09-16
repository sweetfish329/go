/**
 * BoardDetector - OpenCV.js を使用した碁盤画像認識
 *
 * Phase 1: 4隅タップ方式
 *  - ユーザーが碁盤の4隅を指定
 *  - 射影変換で正面図に補正
 *  - 各交点の色をサンプリングして黒/白/空を判定
 */
class BoardDetector {
    constructor() {
        this.cvReady = false;
        this.cornerPoints = null; // 4隅の座標 [{x,y}, ...]
        this.boardSize = 19;
        this.recordedSpacingsX = null; // 実測間隔 X [x0, x1, ... x18]
        this.recordedSpacingsY = null; // 実測間隔 Y [y0, y1, ... y18]

        // 石判定の閾値（チューニング可能）
        this.thresholds = {
            blackMaxBrightness: 80,
            whiteMinBrightness: 180,
            whiteMaxSaturation: 50,
        };
    }

    /**
     * OpenCV.js のロードを待機
     * @returns {Promise<void>}
     */
    async waitForOpenCV() {
        if (typeof cv !== 'undefined' && cv.Mat) {
            this.cvReady = true;
            return;
        }
        return new Promise((resolve, reject) => {
            const timeout = setTimeout(() => {
                clearInterval(checkInterval);
                reject(new Error('OpenCV.js load timeout (30s)'));
            }, 30000);

            const checkInterval = setInterval(() => {
                if (typeof cv !== 'undefined' && cv.Mat) {
                    this.cvReady = true;
                    clearInterval(checkInterval);
                    clearTimeout(timeout);
                    resolve();
                }
            }, 100);
        });
    }

    /**
     * OpenCV.js がロード済みかどうか
     * @returns {boolean}
     */
    isReady() {
        return this.cvReady;
    }

    /**
     * 碁盤の4隅を設定（左上, 右上, 右下, 左下の順に自動正準化）
     * @param {Array<{x: number, y: number}>} corners - 4点の座標
     * @throws {Error} 4点でない場合
     */
    setCorners(corners) {
        if (corners.length !== 4) {
            throw new Error('Exactly 4 corner points required');
        }
        this.cornerPoints = this._orderCorners(corners);
    }

    /**
     * 入力ソース (img, video, canvas) から真のピクセル解像度 (naturalWidth/Height) で cv.Mat を生成
     * HTMLImageElement を直接 cv.imread に渡すと CSS表示サイズ (img.width) で
     * 縮小されてしまい、naturalWidth との座標ズレを起こすブラウザ固有問題を防止する。
     * @param {HTMLCanvasElement|HTMLVideoElement|HTMLImageElement|cv.Mat} source
     * @returns {cv.Mat}
     * @private
     */
    _readSourceMat(source) {
        if (!source) throw new Error('Source element is null or undefined');
        if (typeof cv !== 'undefined' && source instanceof cv.Mat) {
            return source.clone();
        }
        if (typeof HTMLImageElement !== 'undefined' && source instanceof HTMLImageElement) {
            const width = source.naturalWidth || source.width;
            const height = source.naturalHeight || source.height;
            const canvas = document.createElement('canvas');
            canvas.width = width;
            canvas.height = height;
            const ctx = canvas.getContext('2d', { willReadFrequently: true });
            ctx.drawImage(source, 0, 0, width, height);
            return cv.imread(canvas);
        }
        if (typeof HTMLVideoElement !== 'undefined' && source instanceof HTMLVideoElement) {
            const width = source.videoWidth || source.width;
            const height = source.videoHeight || source.height;
            const canvas = document.createElement('canvas');
            canvas.width = width;
            canvas.height = height;
            const ctx = canvas.getContext('2d', { willReadFrequently: true });
            ctx.drawImage(source, 0, 0, width, height);
            return cv.imread(canvas);
        }
        return cv.imread(source);
    }

    /**
     * 4隅がセットされているか
     * @returns {boolean}
     */
    hasCornersSet() {
        return this.cornerPoints !== null && this.cornerPoints.length === 4;
    }

    /**
     * 4隅をリセット
     */
    resetCorners() {
        this.cornerPoints = null;
    }

    /**
     * 石判定の閾値を更新する（照明条件によるチューニング用）
     * @param {Object} thresholds
     */
    setThresholds(thresholds) {
        Object.assign(this.thresholds, thresholds);
    }

    /**
     * 画像から碁盤の4隅を自動検出する (多段ロバストパイプライン)
     *
     * ステージ 1: 木目天板の色域セグメンテーション ＆ 多角形近似輪郭 (Wood Color & Contour)
     * ステージ 2: 確率的ハフ変換 (HoughLinesP) ＋ 外枠格子線交点探索 (Grid Lines Hough Intersections)
     * ステージ 3: 適応的二値化 (Adaptive Threshold) ＋ 最大凸四角形輪郭 (Adaptive Contour Quad)
     *
     * @param {HTMLCanvasElement|HTMLVideoElement|HTMLImageElement} source - 入力画像
     * @returns {Array<{x: number, y: number}>|null} 検出された4隅 [左上, 右上, 右下, 左下] または null
     */
    autoDetectCorners(source) {
        if (!this.cvReady) throw new Error('OpenCV.js not loaded');

        const srcMat = this._readSourceMat(source);
        try {
            const width = srcMat.cols;
            const height = srcMat.rows;

            // ステージ 1: 木目天板の色域セグメンテーションと輪郭近似
            let corners = this._detectBoardByWoodColor(srcMat);
            if (corners && this._validateCorners(corners, width, height)) {
                const ordered = this._orderCorners(corners);
                // 木枠マージン (左右・上下 約3.5%) を逆算して真の19路格子角へインセット (NotebookLM 幾何モデル)
                const gridCorners = this._applyWoodMarginInset(ordered, 0.035, 0.035);
                this.setCorners(gridCorners);
                return gridCorners;
            }

            // 照明ムラを補正した画像を取得 (ステージ 2, 3用)
            const equalized = this._applyCLAHE(srcMat);
            try {
                // ステージ 2: 格子線群のハフ変換 ＆ 境界線交点
                corners = this._detectCornersFromHough(equalized);
                if (corners && this._validateCorners(corners, width, height)) {
                    const ordered = this._orderCorners(corners);
                    this.setCorners(ordered);
                    return ordered;
                }

                // ステージ 3: 適応的二値化による最大輪郭四角形
                corners = this._detectBoardContour(equalized);
                if (corners && this._validateCorners(corners, width, height)) {
                    const ordered = this._orderCorners(corners);
                    this.setCorners(ordered);
                    return ordered;
                }

                return null;
            } finally {
                equalized.delete();
            }
        } finally {
            srcMat.delete();
        }
    }

    /**
     * CLAHE (Contrast Limited Adaptive Histogram Equalization)
     * LAB色空間の L (明度) チャンネルのみに適用し、部屋の影やスポット光のムラを均一化 (gbr より)
     * @param {cv.Mat} srcMat - BGRまたはRGBA画像
     * @returns {cv.Mat} 照明補正後の画像
     * @private
     */
    _applyCLAHE(srcMat) {
        const bgr = new cv.Mat();
        const lab = new cv.Mat();
        const channels = new cv.MatVector();
        const lChannel = new cv.Mat();
        const claheL = new cv.Mat();
        const mergedLab = new cv.Mat();
        const result = new cv.Mat();

        try {
            // RGBA -> BGR -> Lab
            cv.cvtColor(srcMat, bgr, cv.COLOR_RGBA2BGR);
            cv.cvtColor(bgr, lab, cv.COLOR_BGR2Lab);

            cv.split(lab, channels);
            channels.get(0).copyTo(lChannel);

            // CLAHE を L チャンネルに適用 (OpenCV.js で CLAHE が利用可能な場合は適用、なければ equalizeHist)
            try {
                if (typeof cv.createCLAHE === 'function') {
                    const clahe = cv.createCLAHE(3.0, new cv.Size(8, 8));
                    clahe.apply(lChannel, claheL);
                    clahe.delete();
                } else {
                    cv.equalizeHist(lChannel, claheL);
                }
                channels.set(0, claheL);
                cv.merge(channels, mergedLab);
                cv.cvtColor(mergedLab, bgr, cv.COLOR_Lab2BGR);
                cv.cvtColor(bgr, result, cv.COLOR_BGR2RGBA);
                return result.clone();
            } catch {
                // フォールバック: 元の画像をそのまま返す
                return srcMat.clone();
            }
        } finally {
            bgr.delete();
            lab.delete();
            channels.delete();
            lChannel.delete();
            claheL.delete();
            mergedLab.delete();
            result.delete();
        }
    }

    /**
     * ステージ 1: 木目天板色域セグメンテーション ＆ 輪郭近似による4隅検出
     * 碁盤の木目色（黄色〜薄茶色）ブロブを抽出し、石や格子線を閉処理で埋めて最大4角形輪郭を探索
     * @param {cv.Mat} srcMat
     * @returns {Array<{x: number, y: number}>|null}
     * @private
     */
    _detectBoardByWoodColor(srcMat) {
        const width = srcMat.cols;
        const height = srcMat.rows;
        const mask = new cv.Mat(height, width, cv.CV_8UC1);
        const contours = new cv.MatVector();
        const hierarchy = new cv.Mat();

        try {
            const srcData = srcMat.data;
            const maskData = mask.data;

            // 木目の色特徴: R > B + 14, R >= 50, G >= 30, R >= G - 25 (木製天板)
            for (let i = 0, p = 0; i < srcData.length; i += 4, p++) {
                const r = srcData[i];
                const g = srcData[i + 1];
                const b = srcData[i + 2];
                if (r - b >= 14 && r >= 50 && g >= 30 && r >= g - 25) {
                    maskData[p] = 255;
                } else {
                    maskData[p] = 0;
                }
            }

            // モルフォロジー演算 (碁石や格子線を埋めて巨大ブロブ化)
            const kSize = Math.max(7, Math.min(31, Math.round(Math.min(width, height) * 0.025) | 1));
            const kernel = cv.getStructuringElement(cv.MORPH_RECT, new cv.Size(kSize, kSize));
            cv.morphologyEx(mask, mask, cv.MORPH_CLOSE, kernel);
            cv.morphologyEx(mask, mask, cv.MORPH_OPEN, kernel);
            kernel.delete();

            cv.findContours(mask, contours, hierarchy, cv.RETR_EXTERNAL, cv.CHAIN_APPROX_SIMPLE);

            let maxArea = 0;
            let bestCntIndex = -1;
            const minAllowedArea = width * height * 0.12;

            for (let i = 0; i < contours.size(); i++) {
                const cnt = contours.get(i);
                const area = cv.contourArea(cnt);
                if (area > minAllowedArea && area > maxArea) {
                    maxArea = area;
                    bestCntIndex = i;
                }
            }

            if (bestCntIndex === -1) {
                return null;
            }

            const targetCnt = contours.get(bestCntIndex);
            const peri = cv.arcLength(targetCnt, true);

            // 多角形近似 (epsilon を段階的に変化させて4頂点を探す)
            for (let epsRatio = 0.015; epsRatio <= 0.06; epsRatio += 0.008) {
                const approx = new cv.Mat();
                cv.approxPolyDP(targetCnt, approx, epsRatio * peri, true);
                if (approx.rows === 4 && cv.isContourConvex(approx)) {
                    const pts = [];
                    for (let k = 0; k < 4; k++) {
                        pts.push({
                            x: approx.data32S[k * 2],
                            y: approx.data32S[k * 2 + 1]
                        });
                    }
                    approx.delete();
                    return pts;
                }
                approx.delete();
            }

            // 4頂点に収束しない場合: 最小外接回転矩形 (minAreaRect)
            const rotatedRect = cv.minAreaRect(targetCnt);
            const vertices = cv.RotatedRect.points(rotatedRect);
            if (vertices && vertices.length === 4) {
                return [
                    { x: Math.round(vertices[0].x), y: Math.round(vertices[0].y) },
                    { x: Math.round(vertices[1].x), y: Math.round(vertices[1].y) },
                    { x: Math.round(vertices[2].x), y: Math.round(vertices[2].y) },
                    { x: Math.round(vertices[3].x), y: Math.round(vertices[3].y) },
                ];
            }

            return null;
        } catch (err) {
            console.warn('Wood color detection warning:', err);
            return null;
        } finally {
            mask.delete();
            contours.delete();
            hierarchy.delete();
        }
    }

    /**
     * ステージ 2: 確率的ハフ変換と格子線幾何解析による4隅検出
     * (特定ピクセル値のハードコードを完全撤廃し、画像比率と線分クラスタリングで動的に境界交点を計算)
     * @param {cv.Mat} srcMat
     * @returns {Array<{x: number, y: number}>|null}
     * @private
     */
    _detectCornersFromHough(srcMat) {
        const width = srcMat.cols;
        const height = srcMat.rows;

        const gray = new cv.Mat();
        const blurred = new cv.Mat();
        const edges = new cv.Mat();
        const linesH = new cv.Mat();
        const linesV = new cv.Mat();
        const kernelH = cv.getStructuringElement(cv.MORPH_RECT, new cv.Size(15, 1));
        const kernelV = cv.getStructuringElement(cv.MORPH_RECT, new cv.Size(1, 15));
        const edgesH = new cv.Mat();
        const edgesV = new cv.Mat();

        try {
            cv.cvtColor(srcMat, gray, cv.COLOR_RGBA2GRAY);
            cv.GaussianBlur(gray, blurred, new cv.Size(5, 5), 0);
            cv.Canny(blurred, edges, 30, 200, 3, false);

            // 【Circle-First】円形先検出による格子線の石マスク
            const circles = this._detectStonesMultiScale(gray, width, height);
            for (const c of circles) {
                cv.circle(edges, new cv.Point(c.cx, c.cy), Math.round(c.r + 2), new cv.Scalar(0, 0, 0), -1);
            }

            cv.morphologyEx(edges, edgesH, cv.MORPH_CLOSE, kernelH);
            cv.morphologyEx(edges, edgesV, cv.MORPH_CLOSE, kernelV);

            const minLineLen = Math.floor(Math.min(width, height) / 16);
            cv.HoughLinesP(edgesH, linesH, 1, Math.PI / 180, 45, minLineLen, 25);
            cv.HoughLinesP(edgesV, linesV, 1, Math.PI / 180, 45, minLineLen, 25);

            const horizLines = [];
            const vertLines = [];

            const collectLines = (mat, isHorizTarget) => {
                for (let i = 0; i < mat.rows; i++) {
                    const x1 = mat.data32S[i * 4];
                    const y1 = mat.data32S[i * 4 + 1];
                    const x2 = mat.data32S[i * 4 + 2];
                    const y2 = mat.data32S[i * 4 + 3];
                    const dx = x2 - x1, dy = y2 - y1;
                    const len = Math.hypot(dx, dy);
                    let deg = Math.atan2(dy, dx) * 180 / Math.PI;
                    if (deg < -90) deg += 180;
                    if (deg > 90) deg -= 180;

                    const mx = (x1 + x2) / 2;
                    const my = (y1 + y2) / 2;

                    // 画面の最端 4% のノイズを除外
                    if (mx < width * 0.04 || mx > width * 0.96 || my < height * 0.04 || my > height * 0.96) {
                        continue;
                    }

                    // 木目色度チェック
                    const px = Math.max(0, Math.min(width - 1, Math.round(mx)));
                    const py = Math.max(0, Math.min(height - 1, Math.round(my)));
                    const ptr = (py * width + px) * 4;
                    const r = srcMat.data[ptr];
                    const b = srcMat.data[ptr + 2];
                    if (r - b < 12 || r < 50) {
                        continue;
                    }

                    if (isHorizTarget && Math.abs(deg) <= 25) {
                        horizLines.push({ x1, y1, x2, y2, mx, my, deg, len });
                    } else if (!isHorizTarget && (deg <= -65 || deg >= 65)) {
                        vertLines.push({ x1, y1, x2, y2, mx, my, deg, len });
                    }
                }
            };

            collectLines(linesH, true);
            collectLines(linesV, false);

            if (horizLines.length < 2 || vertLines.length < 2) {
                return null;
            }

            horizLines.sort((a, b) => a.my - b.my);
            vertLines.sort((a, b) => a.mx - b.mx);

            // 外れ値除去 (画像サイズ比での許容間隔)
            const maxAllowedGap = Math.min(width, height) * 0.20;
            const filterOutliers = (lines, getCoord) => {
                if (lines.length <= 4) return lines;
                const result = [];
                for (let i = 0; i < lines.length; i++) {
                    const c = getCoord(lines[i]);
                    const distPrev = i > 0 ? Math.abs(c - getCoord(lines[i - 1])) : Infinity;
                    const distNext = i < lines.length - 1 ? Math.abs(c - getCoord(lines[i + 1])) : Infinity;
                    if (distPrev <= maxAllowedGap || distNext <= maxAllowedGap) {
                        result.push(lines[i]);
                    }
                }
                return result.length >= 2 ? result : lines;
            };

            const cleanH = filterOutliers(horizLines, l => l.my);
            const cleanV = filterOutliers(vertLines, l => l.mx);

            const topH = cleanH[0];
            const botH = cleanH[cleanH.length - 1];
            const leftV = cleanV[0];
            const rightV = cleanV[cleanV.length - 1];

            // 2直線の交点を計算
            const intersect = (l1, l2) => {
                const x1 = l1.x1, y1 = l1.y1, x2 = l1.x2, y2 = l1.y2;
                const x3 = l2.x1, y3 = l2.y1, x4 = l2.x2, y4 = l2.y2;
                const denom = (x1 - x2) * (y3 - y4) - (y1 - y2) * (x3 - x4);
                if (Math.abs(denom) < 1e-4) return null;
                const px = ((x1 * y2 - y1 * x2) * (x3 - x4) - (x1 - x2) * (x3 * y4 - y3 * x4)) / denom;
                const py = ((x1 * y2 - y1 * x2) * (y3 - y4) - (y1 - y2) * (x3 * y4 - y3 * x4)) / denom;
                return { x: Math.round(px), y: Math.round(py) };
            };

            const tl = intersect(topH, leftV);
            const tr = intersect(topH, rightV);
            const br = intersect(botH, rightV);
            const bl = intersect(botH, leftV);

            if (tl && tr && br && bl) {
                return [tl, tr, br, bl];
            }
            return null;
        } catch (err) {
            console.warn('Hough corner detection warning:', err);
            return null;
        } finally {
            gray.delete();
            blurred.delete();
            edges.delete();
            linesH.delete();
            linesV.delete();
            kernelH.delete();
            kernelV.delete();
            edgesH.delete();
            edgesV.delete();
        }
    }

    /**
     * ステージ 3: 碁盤外枠の適応的二値化輪郭（Contour）から4隅を検出
     * @param {cv.Mat} srcMat
     * @returns {Array<{x: number, y: number}>|null}
     * @private
     */
    _detectBoardContour(srcMat) {
        const gray = new cv.Mat();
        const blurred = new cv.Mat();
        const thresh = new cv.Mat();
        const contours = new cv.MatVector();
        const hierarchy = new cv.Mat();

        try {
            cv.cvtColor(srcMat, gray, cv.COLOR_RGBA2GRAY);
            cv.GaussianBlur(gray, blurred, new cv.Size(7, 7), 0);
            cv.adaptiveThreshold(blurred, thresh, 255, cv.ADAPTIVE_THRESH_GAUSSIAN_C, cv.THRESH_BINARY_INV, 15, 3);

            cv.findContours(thresh, contours, hierarchy, cv.RETR_EXTERNAL, cv.CHAIN_APPROX_SIMPLE);

            let maxArea = 0;
            let bestApprox = null;
            const minAllowedArea = (srcMat.cols * srcMat.rows) * 0.12;

            for (let i = 0; i < contours.size(); i++) {
                const cnt = contours.get(i);
                const area = cv.contourArea(cnt);
                if (area < minAllowedArea || area <= maxArea) {
                    cnt.delete();
                    continue;
                }

                const peri = cv.arcLength(cnt, true);
                const approx = new cv.Mat();
                cv.approxPolyDP(cnt, approx, 0.03 * peri, true);

                if (approx.rows === 4 && cv.isContourConvex(approx)) {
                    maxArea = area;
                    if (bestApprox) bestApprox.delete();
                    bestApprox = approx;
                } else {
                    approx.delete();
                }
                cnt.delete();
            }

            if (bestApprox && bestApprox.rows === 4) {
                const points = [];
                for (let i = 0; i < 4; i++) {
                    points.push({
                        x: bestApprox.data32S[i * 2],
                        y: bestApprox.data32S[i * 2 + 1]
                    });
                }
                bestApprox.delete();
                return points;
            }

            return null;
        } catch (err) {
            console.warn('Contour corner detection warning:', err);
            return null;
        } finally {
            gray.delete();
            blurred.delete();
            thresh.delete();
            contours.delete();
            hierarchy.delete();
        }
    }

    /**
     * 検出された4隅の幾何学的妥当性を検証 (高専論文 / kaya の縮退判定)
     * @private
     */
    _validateCorners(corners, imgWidth, imgHeight) {
        if (!corners || corners.length !== 4) return false;

        for (const pt of corners) {
            if (pt.x < -imgWidth * 0.05 || pt.x > imgWidth * 1.05 ||
                pt.y < -imgHeight * 0.05 || pt.y > imgHeight * 1.05) {
                return false;
            }
        }

        // 縮退コーナー判定 (4点が狭い領域に集中していないか)
        const xs = corners.map(c => c.x);
        const ys = corners.map(c => c.y);
        const bboxArea = (Math.max(...xs) - Math.min(...xs)) * (Math.max(...ys) - Math.min(...ys));
        if (bboxArea < imgWidth * imgHeight * 0.10) {
            return false;
        }

        // 4点のポリゴン面積を靴紐の公式で計算
        const ordered = this._orderCorners(corners);
        let area = 0;
        for (let i = 0; i < 4; i++) {
            const j = (i + 1) % 4;
            area += ordered[i].x * ordered[j].y;
            area -= ordered[j].x * ordered[i].y;
        }
        area = Math.abs(area) / 2;

        const imgArea = imgWidth * imgHeight;
        if (area < imgArea * 0.10) {
            return false;
        }

        // アスペクト比の検証 (0.5 〜 2.0)
        const dTop = Math.hypot(ordered[1].x - ordered[0].x, ordered[1].y - ordered[0].y);
        const dRight = Math.hypot(ordered[2].x - ordered[1].x, ordered[2].y - ordered[1].y);
        const dBottom = Math.hypot(ordered[3].x - ordered[2].x, ordered[3].y - ordered[2].y);
        const dLeft = Math.hypot(ordered[0].x - ordered[3].x, ordered[0].y - ordered[3].y);

        const avgWidth = (dTop + dBottom) / 2;
        const avgHeight = (dLeft + dRight) / 2;
        if (avgHeight <= 0) return false;
        const ratio = avgWidth / avgHeight;

        return ratio >= 0.5 && ratio <= 2.0;
    }

    /**
     * 4隅の座標を [左上, 右上, 右下, 左下] の時計回り順に整列
     * @param {Array<{x: number, y: number}>} pts
     * @returns {Array<{x: number, y: number}>}
     * @private
     */
    _orderCorners(pts) {
        if (!pts || pts.length !== 4) return pts;

        // x + y 最小 -> 左上(TL), x + y 最大 -> 右下(BR)
        // x - y 最大 -> 右上(TR), x - y 最小 -> 左下(BL)
        const sum = pts.map(p => p.x + p.y);
        const diff = pts.map(p => p.x - p.y);

        let tlIdx = 0, brIdx = 0, trIdx = 0, blIdx = 0;
        for (let i = 1; i < 4; i++) {
            if (sum[i] < sum[tlIdx]) tlIdx = i;
            if (sum[i] > sum[brIdx]) brIdx = i;
            if (diff[i] > diff[trIdx]) trIdx = i;
            if (diff[i] < diff[blIdx]) blIdx = i;
        }

        const indices = new Set([tlIdx, trIdx, brIdx, blIdx]);
        if (indices.size === 4) {
            return [pts[tlIdx], pts[trIdx], pts[brIdx], pts[blIdx]];
        }

        // 重複時フォールバック: 重心からの角度で整列
        const cx = pts.reduce((s, p) => s + p.x, 0) / 4;
        const cy = pts.reduce((s, p) => s + p.y, 0) / 4;
        const sorted = [...pts].sort((a, b) => Math.atan2(a.y - cy, a.x - cx) - Math.atan2(b.y - cy, b.x - cx));
        let minSumIdx = 0;
        sorted.forEach((p, idx) => {
            if (p.x + p.y < sorted[minSumIdx].x + sorted[minSumIdx].y) {
                minSumIdx = idx;
            }
        });
        return [
            sorted[minSumIdx],
            sorted[(minSumIdx + 1) % 4],
            sorted[(minSumIdx + 2) % 4],
            sorted[(minSumIdx + 3) % 4],
        ];
    }

    /**
     * 木枠マージン (m_x, m_y) のインセット逆算補正
     * 天板外形輪郭が検出された際、外枠マージン（日本・韓国規格: 約3.5%）を内側へ線形インセットし、
     * 真の19路格子角 (A19, T19, T1, A1) に収束させる (NotebookLM 幾何モデル)
     * @param {Array<{x: number, y: number}>} corners - [TL, TR, BR, BL]
     * @param {number} [insetX=0.035] - 横方向インセット比率
     * @param {number} [insetY=0.035] - 縦方向インセット比率
     * @returns {Array<{x: number, y: number}>} 補正後の格子4隅
     * @private
     */
    _applyWoodMarginInset(corners, insetX = 0.035, insetY = 0.035) {
        if (!corners || corners.length !== 4) return corners;
        const [tl, tr, br, bl] = corners;

        const vTop = { x: tr.x - tl.x, y: tr.y - tl.y };
        const vBot = { x: br.x - bl.x, y: br.y - bl.y };
        const vLeft = { x: bl.x - tl.x, y: bl.y - tl.y };
        const vRight = { x: br.x - tr.x, y: br.y - tr.y };

        return [
            {
                x: Math.round(tl.x + insetX * vTop.x + insetY * vLeft.x),
                y: Math.round(tl.y + insetX * vTop.y + insetY * vLeft.y),
            },
            {
                x: Math.round(tr.x - insetX * vTop.x + insetY * vRight.x),
                y: Math.round(tr.y - insetX * vTop.y + insetY * vRight.y),
            },
            {
                x: Math.round(br.x - insetX * vBot.x - insetY * vRight.x),
                y: Math.round(br.y - insetX * vBot.y - insetY * vRight.y),
            },
            {
                x: Math.round(bl.x + insetX * vBot.x - insetY * vLeft.x),
                y: Math.round(bl.y + insetX * vBot.y - insetY * vLeft.y),
            },
        ];
    }

    /**
     * 画像から盤面状態を検出する
     * @param {HTMLCanvasElement|HTMLVideoElement|HTMLImageElement} source - 入力画像
     * @returns {Int8Array} 361要素の交点状態配列 (0=空, 1=黒, 2=白)
     */
    detect(source) {
        if (!this.cvReady) throw new Error('OpenCV.js not loaded');
        if (!this.cornerPoints) {
            const detected = this.autoDetectCorners(source);
            if (!detected) {
                throw new Error('Corner points not set and auto-detection failed. Call setCorners() or tap 4 corners.');
            }
        }

        const srcMat = this._readSourceMat(source);
        try {
            // 射影変換で正面図に補正
            const warped = this._perspectiveTransform(srcMat);
            try {
                // 各交点の石を判定 (CLAHE + 1D K-means + 局所分散判定)
                return this._classifyIntersections(warped);
            } finally {
                warped.delete();
            }
        } finally {
            srcMat.delete();
        }
    }

    /**
     * 盤面の交点状態配列 (0=空, 1=黒, 2=白) を取得する (detect のエイリアス)
     * @param {HTMLCanvasElement|HTMLVideoElement|HTMLImageElement} source - 入力画像
     * @returns {Int8Array} 361要素の交点状態配列
     */
    classifyBoard(source) {
        return this.detect(source);
    }

    /**
     * 碁盤画像から全361交点の多次元特徴量（L, C, D, H, S, R, B, RMB）を抽出
     * @param {HTMLCanvasElement|HTMLVideoElement|HTMLImageElement} source
     * @returns {Array<Object>}
     */
    extractFeatures(source) {
        if (!this.cvReady) throw new Error('OpenCV.js not loaded');
        if (!this.hasCornersSet()) {
            const detected = this.autoDetectCorners(source);
            if (!detected) {
                throw new Error('Corner points not set and auto-detection failed.');
            }
        }

        const srcMat = this._readSourceMat(source);
        try {
            const warped = this._perspectiveTransform(srcMat);
            try {
                return this._extractFeaturesFromWarped(warped);
            } finally {
                warped.delete();
            }
        } finally {
            srcMat.delete();
        }
    }

    /**
     * 射影変換で碁盤を正面から見た画像に補正
     * @param {cv.Mat} srcMat - 入力画像
     * @returns {cv.Mat} 射影変換済み画像
     * @private
     */
    _perspectiveTransform(srcMat) {
        // 日本・韓国規格の碁盤縦横比（約 1.077: 42.4cm x 45.5cm）
        // 四隅の形状から縦長か横長かを判定
        const p = this.cornerPoints;
        const wTop = Math.hypot(p[1].x - p[0].x, p[1].y - p[0].y);
        const wBot = Math.hypot(p[2].x - p[3].x, p[2].y - p[3].y);
        const hLeft = Math.hypot(p[3].x - p[0].x, p[3].y - p[0].y);
        const hRight = Math.hypot(p[2].x - p[1].x, p[2].y - p[1].y);
        const avgW = (wTop + wBot) / 2;
        const avgH = (hLeft + hRight) / 2;

        const margin = 40;
        let dstW, dstH;
        if (avgH >= avgW) {
            // 縦長（通常の対局者視点: 1.077 比）
            dstW = 600 + 2 * margin;
            dstH = Math.round(600 * 1.077) + 2 * margin; // 約726
        } else {
            // 横長（横からの撮影視点）
            dstH = 600 + 2 * margin;
            dstW = Math.round(600 * 1.077) + 2 * margin;
        }

        const srcPoints = cv.matFromArray(4, 1, cv.CV_32FC2, [
            p[0].x, p[0].y,
            p[1].x, p[1].y,
            p[2].x, p[2].y,
            p[3].x, p[3].y,
        ]);

        const dstPoints = cv.matFromArray(4, 1, cv.CV_32FC2, [
            margin, margin,
            dstW - margin, margin,
            dstW - margin, dstH - margin,
            margin, dstH - margin,
        ]);

        const transformMatrix = cv.getPerspectiveTransform(srcPoints, dstPoints);
        const warped = new cv.Mat();
        const dsize = new cv.Size(dstW, dstH);
        cv.warpPerspective(srcMat, warped, transformMatrix, dsize, cv.INTER_LINEAR, cv.BORDER_CONSTANT, new cv.Scalar());

        srcPoints.delete();
        dstPoints.delete();
        transformMatrix.delete();

        return warped;
    }

    /**
     * 4層マルチスケール・ガウシアンフィルタによる碁石（円形）の相補的検出
     * 鮮明な石（小スケール）とボケた石・密集した黒石クラスタ（大スケール）を統合
     * @param {cv.Mat} grayMat - グレースケール画像
     * @param {number} width - 幅
     * @param {number} height - 高さ
     * @returns {Array<{cx: number, cy: number, r: number}>} 重複排除された円のリスト
     * @private
     */
    _detectStonesMultiScale(grayMat, width, height) {
        const minRadius = Math.floor(Math.min(width, height) / 45);
        const maxRadius = Math.floor(Math.min(width, height) / 18);
        const scales = [
            new cv.Size(3, 3),
            new cv.Size(5, 5),
            new cv.Size(7, 7),
            new cv.Size(9, 9),
        ];

        const detectedCircles = [];
        const circlesMat = new cv.Mat();
        const blurMat = new cv.Mat();

        try {
            for (const ksize of scales) {
                cv.GaussianBlur(grayMat, blurMat, ksize, 0);
                try {
                    cv.HoughCircles(blurMat, circlesMat, cv.HOUGH_GRADIENT, 1, minRadius * 1.6, 90, 25, minRadius, maxRadius);
                    if (circlesMat.cols > 0) {
                        for (let i = 0; i < circlesMat.cols; i++) {
                            const cx = Math.round(circlesMat.data32F[i * 3]);
                            const cy = Math.round(circlesMat.data32F[i * 3 + 1]);
                            const r = Math.round(circlesMat.data32F[i * 3 + 2]);

                            // 既存の円と近接している場合は重複排除（距離が minRadius * 0.8 未満）
                            const isDuplicate = detectedCircles.some(c => {
                                const dist2 = (c.cx - cx) ** 2 + (c.cy - cy) ** 2;
                                return dist2 < (minRadius * 0.8) ** 2;
                            });

                            if (!isDuplicate) {
                                detectedCircles.push({ cx, cy, r });
                            }
                        }
                    }
                } catch {
                    // スキップ
                }
            }
            return detectedCircles;
        } finally {
            circlesMat.delete();
            blurMat.delete();
        }
    }

    /**
     * 実測間隔（Recorded Spacings）モデルのキャリブレーション
     * 単純な19等分ではなく、検出された格子線間の実測ピクセル間隔を保持して歪みに高精度追従
     * @param {HTMLCanvasElement|HTMLVideoElement|HTMLImageElement} source
     * @returns {{spacingsX: number[], spacingsY: number[]}}
     */
    calibrateRecordedSpacings(source) {
        if (!this.cvReady) throw new Error('OpenCV.js not loaded');
        if (!this.cornerPoints) {
            this.autoDetectCorners(source);
        }
        if (!this.cornerPoints) {
            throw new Error('Corners not set');
        }

        const srcMat = this._readSourceMat(source);
        try {
            const warped = this._perspectiveTransform(srcMat);
            try {
                const gray = new cv.Mat();
                cv.cvtColor(warped, gray, cv.COLOR_RGBA2GRAY);

                const equalized = new cv.Mat();
                try {
                    if (typeof cv.createCLAHE === 'function') {
                        const clahe = cv.createCLAHE(2.0, new cv.Size(8, 8));
                        clahe.apply(gray, equalized);
                        clahe.delete();
                    } else {
                        cv.equalizeHist(gray, equalized);
                    }
                } catch {
                    gray.copyTo(equalized);
                }

                const dstSize = warped.cols;
                const expectedSpacing = dstSize / (this.boardSize - 1);

                // 縦・横の1D Sobelプロファイルから19本のピークを検出
                const spacingsX = this._findLinePeaks1D(equalized, 'horizontal', dstSize, expectedSpacing);
                const spacingsY = this._findLinePeaks1D(equalized, 'vertical', dstSize, expectedSpacing);

                this.recordedSpacingsX = spacingsX;
                this.recordedSpacingsY = spacingsY;

                equalized.delete();
                gray.delete();

                return {
                    spacingsX: this.recordedSpacingsX,
                    spacingsY: this.recordedSpacingsY,
                };
            } finally {
                warped.delete();
            }
        } finally {
            srcMat.delete();
        }
    }

    /**
     * 1D 射影プロファイルから19本の格子線ピークを検出
     * @private
     */
    _findLinePeaks1D(grayMat, orientation, size, expectedSpacing) {
        const peaks = [];
        const sobel = new cv.Mat();

        try {
            if (orientation === 'horizontal') {
                cv.Sobel(grayMat, sobel, cv.CV_32F, 1, 0, 3);
            } else {
                cv.Sobel(grayMat, sobel, cv.CV_32F, 0, 1, 3);
            }

            // 1D プロファイルを計算 (絶対値の平均)
            const profile = new Float32Array(size);
            const cols = sobel.cols;
            const rows = sobel.rows;

            for (let r = 0; r < rows; r++) {
                for (let c = 0; c < cols; c++) {
                    const val = Math.abs(sobel.floatAt(r, c));
                    if (orientation === 'horizontal') {
                        profile[c] += val;
                    } else {
                        profile[r] += val;
                    }
                }
            }

            const normalizer = orientation === 'horizontal' ? rows : cols;
            for (let i = 0; i < size; i++) {
                profile[i] /= normalizer;
            }

            // 各理論交点付近 (± expectedSpacing * 0.35) で局所最大値を探索
            const searchWindow = Math.round(expectedSpacing * 0.35);
            for (let i = 0; i < this.boardSize; i++) {
                const theoreticalPos = Math.round(i * expectedSpacing);
                let maxVal = -1;
                let bestPos = theoreticalPos;

                const start = Math.max(0, theoreticalPos - searchWindow);
                const end = Math.min(size - 1, theoreticalPos + searchWindow);

                for (let pos = start; pos <= end; pos++) {
                    if (profile[pos] > maxVal) {
                        maxVal = profile[pos];
                        bestPos = pos;
                    }
                }
                peaks.push(bestPos);
            }

            return peaks;
        } finally {
            sobel.delete();
        }
    }

    /**
     * 射影変換済み画像の全交点から多次元特徴量 (ドーナツ走査) を抽出
     * @param {cv.Mat} warpedMat
     * @private
     */
    _extractFeaturesFromWarped(warpedMat) {
        const w = warpedMat.cols;
        const h = warpedMat.rows;
        const margin = 40;
        const boardW = w - 2 * margin;
        const boardH = h - 2 * margin;

        // 消失点・複比（Cross-Ratio）に基づく有理関数透視ピッチモデル (NotebookLM 学術数理モデル)
        // f(t) = (H * r * t) / ((r - 1) * t + 18) (r = sqrt(pStart / pEnd))
        const pStart = 33.0;
        const pEnd = 39.0;
        const rRatio = Math.sqrt(pStart / pEnd);
        const numDenomCoeff = rRatio - 1.0;
        const gridY = [];
        for (let i = 0; i < this.boardSize; i++) {
            const t = i;
            const denom = numDenomCoeff * t + (this.boardSize - 1);
            const yOffset = (denom !== 0) ? (boardH * rRatio * t) / denom : (boardH * (t / (this.boardSize - 1)));
            gridY.push(margin + Math.round(yOffset));
        }
        gridY[0] = margin;
        gridY[this.boardSize - 1] = margin + boardH;

        const stepX = boardW / (this.boardSize - 1);
        const gridX = [];
        for (let i = 0; i < this.boardSize; i++) {
            gridX.push(margin + Math.round(i * stepX));
        }

        // 光学チャンネル分離 (Red = 木目反射を受けにくい白石検出用, Blue = テカリを排した黒石検出用, Green = YCrCb/Chroma用)
        const rgbaPlanes = new cv.MatVector();
        cv.split(warpedMat, rgbaPlanes);
        const rPlane = rgbaPlanes.get(0);
        const gPlane = rgbaPlanes.get(1);
        const bPlane = rgbaPlanes.get(2);
        const rmbPlane = new cv.Mat();
        cv.subtract(rPlane, bPlane, rmbPlane);

        // 白石マスク: Red高輝度 かつ (Red - Blue) が小さい(無彩色)
        const whiteMask = new cv.Mat();
        const rThresh = new cv.Mat();
        const rmbThresh = new cv.Mat();
        cv.threshold(rPlane, rThresh, 140, 255, cv.THRESH_BINARY);
        cv.threshold(rmbPlane, rmbThresh, 25, 255, cv.THRESH_BINARY_INV);
        cv.bitwise_and(rThresh, rmbThresh, whiteMask);

        // 黒石マスク: Blue低輝度 かつ Red低輝度 かつ (Red - Blue) が小さい
        const blackMask = new cv.Mat();
        const bThresh = new cv.Mat();
        const rThreshLow = new cv.Mat();
        const rmbThreshLow = new cv.Mat();
        cv.threshold(bPlane, bThresh, 90, 255, cv.THRESH_BINARY_INV);
        cv.threshold(rPlane, rThreshLow, 120, 255, cv.THRESH_BINARY_INV);
        cv.threshold(rmbPlane, rmbThreshLow, 28, 255, cv.THRESH_BINARY_INV);
        const temp = new cv.Mat();
        cv.bitwise_and(bThresh, rThreshLow, temp);
        cv.bitwise_and(temp, rmbThreshLow, blackMask);

        // 距離変換による石中心(ピーク)の特定
        const distW = new cv.Mat();
        const distB = new cv.Mat();
        cv.distanceTransform(whiteMask, distW, cv.DIST_L2, 5);
        cv.distanceTransform(blackMask, distB, cv.DIST_L2, 5);

        const rawFeatures = [];
        const sampleRadius = Math.floor(stepX * 0.30);
        const snapR = 9;
        const hMaximaThresh = 2.5; // H-maxima 抑制閾値: 微小ノイズピークによる誤スナップを抑制

        try {
            for (let y = 0; y < this.boardSize; y++) {
                const cy = gridY[y];
                // 碁石の厚み(約9mm)と俯角による視差(Parallax)幾何補正 (NotebookLM モデル)
                // 奥側(y小)ほど視線が浅く、石上面の重心が手前(+Y方向)に浮き上がるため探索中心をオフセット
                const parallaxShiftY = (1.0 - y / (this.boardSize - 1)) * 3.0;
                const searchCY = cy + parallaxShiftY;

                for (let x = 0; x < this.boardSize; x++) {
                    const cx = gridX[x];

                    // 局所距離変換の最大値探索 (近傍の石中心へスナップ)
                    let maxDW = 0, maxDB = 0;
                    let bestPeakWX = cx, bestPeakWY = searchCY;
                    let bestPeakBX = cx, bestPeakBY = searchCY;

                    // 盤外への誤スナップを防ぐ境界クランプ
                    const minDX = (x === 0) ? -2 : -snapR;
                    const maxDX = (x === this.boardSize - 1) ? 2 : snapR;
                    const minDY = (y === 0) ? -2 : -snapR;
                    const maxDY = (y === this.boardSize - 1) ? 2 : snapR;

                    for (let dy = minDY; dy <= maxDY; dy++) {
                        for (let dx = minDX; dx <= maxDX; dx++) {
                            if (dx * dx + dy * dy <= snapR * snapR) {
                                const px = cx + dx;
                                const py = Math.round(searchCY + dy);
                                if (px >= 0 && px < w && py >= 0 && py < h) {
                                    const dw = distW.floatAt(py, px);
                                    const db = distB.floatAt(py, px);
                                    if (dw > maxDW) { maxDW = dw; bestPeakWX = px; bestPeakWY = py; }
                                    if (db > maxDB) { maxDB = db; bestPeakBX = px; bestPeakBY = py; }
                                }
                            }
                        }
                    }

                    // H-maxima 抑制: ピーク強度が閾値未満の場合は中心スナップを行わず理論交点を保持
                    let sampleX = cx;
                    let sampleY = searchCY;
                    if (Math.max(maxDW, maxDB) >= hMaximaThresh) {
                        sampleX = maxDW > maxDB ? bestPeakWX : bestPeakBX;
                        sampleY = maxDW > maxDB ? bestPeakWY : bestPeakBY;
                    }

                    // スナップ点周辺の平均色をサンプリング (RGB 3チャンネル)
                    let sumR = 0, sumG = 0, sumB = 0, count = 0;
                    for (let dy = -sampleRadius; dy <= sampleRadius; dy++) {
                        for (let dx = -sampleRadius; dx <= sampleRadius; dx++) {
                            if (dx * dx + dy * dy <= sampleRadius * sampleRadius) {
                                const px = sampleX + dx;
                                const py = sampleY + dy;
                                if (px >= 0 && px < w && py >= 0 && py < h) {
                                    sumR += rPlane.ucharAt(py, px);
                                    sumG += gPlane.ucharAt(py, px);
                                    sumB += bPlane.ucharAt(py, px);
                                    count++;
                                }
                            }
                        }
                    }
                    const avgR = count > 0 ? sumR / count : rPlane.ucharAt(sampleY, sampleX);
                    const avgG = count > 0 ? sumG / count : gPlane.ucharAt(sampleY, sampleX);
                    const avgB = count > 0 ? sumB / count : bPlane.ucharAt(sampleY, sampleX);
                    const rmb = avgR - avgB;

                    // YCrCb 色差と無彩色度 (Chroma) の計算 (NotebookLM 色彩モデル: 影と黒石の分離)
                    const cr = 0.500 * avgR - 0.418688 * avgG - 0.081312 * avgB + 128.0;
                    const cb = -0.168736 * avgR - 0.331264 * avgG + 0.500 * avgB + 128.0;
                    const chroma = Math.hypot(cr - 128.0, cb - 128.0);

                    // 二色性反射モデル (Dichromatic Reflection Model: NotebookLM 学術モデル)
                    const sumRGB = avgR + avgG + avgB;
                    const maxRGB = Math.max(avgR, avgG, avgB);
                    const minRGB = Math.min(avgR, avgG, avgB);
                    const maxChroma = sumRGB > 0 ? maxRGB / sumRGB : 1.0 / 3.0;
                    const specWeight = minRGB;
                    const diffuseR = avgR - minRGB;

                    // 理論交点 (cx, cy) の色
                    const cR = rPlane.ucharAt(cy, cx);
                    const cB = bPlane.ucharAt(cy, cx);
                    const cRmb = cR - cB;

                    rawFeatures.push({
                        x, y, cx, cy,
                        sampleX, sampleY,
                        l: (cR + cB) / 2,
                        r: avgR,
                        g: avgG,
                        b: avgB,
                        cr, cb, chroma,
                        maxChroma, specWeight, diffuseR,
                        rmb: rmb,
                        cR, cB, cRmb,
                        avgR, avgB,
                        maxDW, maxDB
                    });
                }
            }

            // 局所背景適応: 近傍3マスの中央値背景 (bgR, bgB) を算出し normR, normB を設定
            const features = rawFeatures.map((f) => {
                const x = f.x;
                const y = f.y;
                const rList = [];
                const bList = [];

                for (let dy = -3; dy <= 3; dy++) {
                    for (let dx = -3; dx <= 3; dx++) {
                        const nx = x + dx;
                        const ny = y + dy;
                        if (nx >= 0 && nx < this.boardSize && ny >= 0 && ny < this.boardSize) {
                            const neighbor = rawFeatures[ny * this.boardSize + nx];
                            rList.push(neighbor.r);
                            bList.push(neighbor.b);
                        }
                    }
                }

                rList.sort((a, b) => a - b);
                bList.sort((a, b) => a - b);
                const bgR = Math.max(15, rList[Math.floor(rList.length / 2)] || 80);
                const bgB = Math.max(15, bList[Math.floor(bList.length / 2)] || 40);

                return {
                    ...f,
                    localBgR: bgR,
                    localBgB: bgB,
                    normR: f.r / bgR,
                    normB: f.b / bgB
                };
            });

            return features;
        } finally {
            rgbaPlanes.delete();
            rPlane.delete();
            gPlane.delete();
            bPlane.delete();
            rmbPlane.delete();
            whiteMask.delete();
            blackMask.delete();
            rThresh.delete();
            rmbThresh.delete();
            bThresh.delete();
            rThreshLow.delete();
            rmbThreshLow.delete();
            temp.delete();
            distW.delete();
            distB.delete();
        }
    }

    /**
     * 射影変換済み画像の各交点で石の色を判定 (局所背景適応 ＋ 赤青チャンネル分離 ＋ YCrCb/Chroma影除外)
     * @param {cv.Mat} warpedMat - 射影変換済み画像
     * @returns {Int8Array} 交点状態配列
     * @private
     */
    _classifyIntersections(warpedMat) {
        const result = new Int8Array(this.boardSize * this.boardSize);
        const features = this._extractFeaturesFromWarped(warpedMat);

        for (let i = 0; i < features.length; i++) {
            const f = features[i];
            let stone = 0;

            // 1. 白石判定: 無彩色かつ高輝度、局所背景青成分より高い
            if (f.rmb < 32.0 && f.r >= 85.0 && (f.normB > 1.12 || (f.normR > 1.04 && f.rmb < 20.0))) {
                stone = 2;
            } else if (f.r >= 135 && f.rmb <= 22) {
                stone = 2;
            }

            // 2. 黒石判定: 無彩色かつ低輝度、局所背景より暗い、かつ無彩色(Chroma低)で暗い木目の影を除外
            if (stone === 0 && f.rmb < 35.0) {
                if (f.r < 75.0 && (f.normR < 0.90 || f.normB < 0.90) && (!f.chroma || f.chroma < 22.0)) {
                    stone = 1;
                } else if (f.normB < 0.82 && f.normR < 0.88 && (!f.chroma || f.chroma < 18.0)) {
                    stone = 1;
                } else if (f.b < 45.0 && f.r < 75.0 && f.rmb < 25.0) {
                    stone = 1;
                }
            }

            // 3. 距離変換ピークによるサルベージ（密集石）
            if (stone === 0) {
                if (f.maxDW >= 4.0 && f.maxDW > f.maxDB && f.rmb <= 25.0) {
                    stone = 2;
                } else if (f.maxDB >= 3.5 && f.maxDB > f.maxDW && f.rmb < 30.0 && (f.r < 90.0 || f.normB < 0.92)) {
                    stone = 1;
                }
            }

            result[i] = stone;
        }

        return result;
    }

    /**
     * ストリーム追跡（リアルタイム動画）用ステートの初期化（省電力2段階パイプライン）
     */
    initStreamTracker() {
        if (!this.streamCanvas) {
            this.streamCanvas = document.createElement('canvas');
            this.streamCanvas.width = 160;
            this.streamCanvas.height = 120;
            this.streamCtx = this.streamCanvas.getContext('2d', { willReadFrequently: true });
        }

        this.streamState = {
            prevMiniGray: null,
            stableFrameCount: 0,
            requiredStableFrames: 14, // 約0.8〜1.0秒間静止で着手確定
            motionThreshold: 2.0,     // 160x120縮小空間での動体差分輝度閾値（手の出入りを確実に捉える）
            isHandPresent: false,
            lastTriggerTime: 0,
            cooldownMs: 1500,         // 発火後の二重トリガー防止クールダウン
        };
    }

    /**
     * ストリームフレームの動画追跡・静止安定トリガー処理 (省電力2段階パイプライン)
     * @param {HTMLCanvasElement|HTMLVideoElement} source
     * @returns {{
     *   motionLevel: number,
     *   stableProgress: number,
     *   isStable: boolean,
     *   isTriggered: boolean,
     *   features: Array|null
     * }}
     */
    processStreamFrame(source) {
        if (!this.cvReady || !this.hasCornersSet()) {
            return { motionLevel: 0, stableProgress: 0, isStable: false, isTriggered: false, features: null };
        }

        if (!this.streamState) {
            this.initStreamTracker();
        }

        // =========================================================
        // ステージ 1: 超軽量・低負荷動体監視 (160x120 グレースケール差分)
        // =========================================================
        this.streamCtx.drawImage(source, 0, 0, 160, 120);
        const miniMat = cv.imread(this.streamCanvas);
        const miniGray = new cv.Mat();
        cv.cvtColor(miniMat, miniGray, cv.COLOR_RGBA2GRAY);
        miniMat.delete();

        let motionLevel = 0;
        let isTriggered = false;
        let features = null;
        const now = Date.now();

        if (this.streamState.prevMiniGray) {
            const diff = new cv.Mat();
            cv.absdiff(miniGray, this.streamState.prevMiniGray, diff);
            const meanScalar = cv.mean(diff);
            motionLevel = meanScalar[0]; // 平均変化量
            diff.delete();

            // クールダウン中はトリガー発火を一時抑止
            const inCooldown = (now - this.streamState.lastTriggerTime) < this.streamState.cooldownMs;

            if (motionLevel > this.streamState.motionThreshold) {
                // 手や腕の動きを検知
                this.streamState.isHandPresent = true;
                this.streamState.stableFrameCount = 0;
            } else {
                // 静止カウント
                this.streamState.stableFrameCount++;

                if (!inCooldown && this.streamState.stableFrameCount >= this.streamState.requiredStableFrames) {
                    if (this.streamState.isHandPresent) {
                        // =========================================================
                        // ステージ 2: 静止確定時のみ高解像度解析を1回だけ発火！
                        // =========================================================
                        const srcMat = this._readSourceMat(source);
                        try {
                            const warped = this._perspectiveTransform(srcMat);
                            try {
                                features = this._extractFeaturesFromWarped(warped);
                                isTriggered = true;
                                this.streamState.isHandPresent = false;
                                this.streamState.lastTriggerTime = now;
                            } finally {
                                warped.delete();
                            }
                        } finally {
                            srcMat.delete();
                        }
                    }
                }
            }

            this.streamState.prevMiniGray.delete();
        }

        this.streamState.prevMiniGray = miniGray;

        const stableProgress = Math.min(1.0, this.streamState.stableFrameCount / this.streamState.requiredStableFrames);

        return {
            motionLevel: Math.round(motionLevel * 10) / 10,
            stableProgress,
            isStable: this.streamState.stableFrameCount >= this.streamState.requiredStableFrames,
            isTriggered,
            features
        };
    }

    /**
     * ストリーム追跡のリソース解放
     */
    resetStreamTracker() {
        if (this.streamState) {
            if (this.streamState.prevMiniGray) {
                this.streamState.prevMiniGray.delete();
            }
            this.streamState = null;
        }
        this.streamCanvas = null;
        this.streamCtx = null;
    }

    /**
     * カメラストリームを開始
     * @param {HTMLVideoElement} videoElement - ビデオ要素
     * @param {'user'|'environment'} facingMode - カメラの向き（デフォルト: 背面）
     * @returns {Promise<MediaStream>}
     */
    async startCamera(videoElement, facingMode = 'environment') {
        const constraints = {
            video: {
                facingMode: facingMode,
                width: { ideal: 1280 },
                height: { ideal: 720 },
            },
            audio: false,
        };

        const stream = await navigator.mediaDevices.getUserMedia(constraints);
        videoElement.srcObject = stream;
        videoElement.setAttribute('playsinline', ''); // iOS Safari 必須
        await videoElement.play();
        return stream;
    }

    /**
     * カメラストリームを停止
     * @param {MediaStream} stream
     */
    stopCamera(stream) {
        if (stream) {
            stream.getTracks().forEach(track => track.stop());
        }
        this.resetStreamTracker();
    }

    /**
     * ビデオフレームをキャプチャしてCanvasに描画
     * @param {HTMLVideoElement} videoElement
     * @param {HTMLCanvasElement} canvasElement
     */
    captureFrame(videoElement, canvasElement) {
        canvasElement.width = videoElement.videoWidth;
        canvasElement.height = videoElement.videoHeight;
        const ctx = canvasElement.getContext('2d');
        ctx.drawImage(videoElement, 0, 0);
    }
}

export { BoardDetector };


