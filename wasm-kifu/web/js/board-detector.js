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
     * 碁盤の4隅を設定（左上, 右上, 右下, 左下の順）
     * @param {Array<{x: number, y: number}>} corners - 4点の座標
     * @throws {Error} 4点でない場合
     */
    setCorners(corners) {
        if (corners.length !== 4) {
            throw new Error('Exactly 4 corner points required');
        }
        this.cornerPoints = corners;
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
     * 画像から碁盤の4隅を自動検出する (Qiita / gbr / img2sgf / kaya の統合パイプライン)
     *
     * 1. LAB色空間での CLAHE による照明ムラ・影の均一化 (gbr)
     * 2. 確率的ハフ変換 (HoughLinesP) + 石消去 (Circle Erasure) による格子線復元 (img2sgf)
     * 3. 角度クラスタリング & 射影極値探索 (Qiita)
     * 4. 幾何学的妥当性・縮退コーナー判定 (高専論文 / kaya)
     *
     * @param {HTMLCanvasElement|HTMLVideoElement|HTMLImageElement} source - 入力画像
     * @returns {Array<{x: number, y: number}>|null} 検出された4隅 [左上, 右上, 右下, 左下] または null
     */
    autoDetectCorners(source) {
        if (!this.cvReady) throw new Error('OpenCV.js not loaded');

        const srcMat = cv.imread(source);
        try {
            // 照明ムラを補正した画像を取得
            const equalized = this._applyCLAHE(srcMat);
            try {
                const corners = this._detectCornersFromMat(equalized);
                if (corners && this._validateCorners(corners, srcMat.cols, srcMat.rows)) {
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
     * OpenCV Mat からハフ変換と線分解析を用いて4隅を検出
     * (img2sgf の Circle Erasure + Qiita の角度クラスタリング)
     * @param {cv.Mat} srcMat
     * @returns {Array<{x: number, y: number}>|null}
     * @private
     */
    _detectCornersFromMat(srcMat) {
        const width = srcMat.cols;
        const height = srcMat.rows;

        const gray = new cv.Mat();
        const blurred = new cv.Mat();
        const edges = new cv.Mat();
        const dilated = new cv.Mat();
        const eroded = new cv.Mat();
        const lines = new cv.Mat();
        const circles = new cv.Mat();

        // 楕円カーネル (gbr より: 碁石の形状に最適化)
        const ellipseKernel = cv.getStructuringElement(cv.MORPH_ELLIPSE, new cv.Size(3, 3));

        try {
            // 1. グレースケール & 平滑化
            cv.cvtColor(srcMat, gray, cv.COLOR_RGBA2GRAY);
            cv.GaussianBlur(gray, blurred, new cv.Size(5, 5), 0);

            // 2. Canny エッジ検出
            cv.Canny(blurred, edges, 30, 200, 3, false);

            // 3. 【Circle-First】円形先検出による格子線の石マスク
            const circles = this._detectStonesMultiScale(gray, width, height);
            for (const c of circles) {
                cv.circle(edges, new cv.Point(c.cx, c.cy), Math.round(c.r + 2), new cv.Scalar(0, 0, 0), -1);
            }

            // 4. 方向性モルフォロジーで格子線を連結
            const kernelH = cv.getStructuringElement(cv.MORPH_RECT, new cv.Size(15, 1));
            const kernelV = cv.getStructuringElement(cv.MORPH_RECT, new cv.Size(1, 15));
            const edgesH = new cv.Mat();
            const edgesV = new cv.Mat();
            cv.morphologyEx(edges, edgesH, cv.MORPH_CLOSE, kernelH);
            cv.morphologyEx(edges, edgesV, cv.MORPH_CLOSE, kernelV);

            // 5. 確率的ハフ変換で横線と縦線を独立検出
            const linesH = new cv.Mat();
            const linesV = new cv.Mat();
            const minLineLen = Math.floor(Math.min(width, height) / 16);
            cv.HoughLinesP(edgesH, linesH, 1, Math.PI / 180, 50, minLineLen, 25);
            cv.HoughLinesP(edgesV, linesV, 1, Math.PI / 180, 50, minLineLen, 25);

            // 線分の収集と角度分類
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

                    // 画面の最端（ノイズ）を除外
                    if (mx < width * 0.05 || mx > width * 0.95 || my < height * 0.10 || my > height * 0.90) {
                        continue;
                    }

                    // 【木目盤面ROIチェック】直線の中心が木目天板(R - B >= 20, R >= 100)上にあるか判定
                    const px = Math.max(0, Math.min(width - 1, Math.round(mx)));
                    const py = Math.max(0, Math.min(height - 1, Math.round(my)));
                    const ptr = (py * width + px) * 4;
                    const r = srcMat.data[ptr];
                    const g = srcMat.data[ptr + 1];
                    const b = srcMat.data[ptr + 2];
                    const rmb = r - b;

                    // 机・床・壁のノイズ直線を色特徴で棄却 (暗い奥側木目 R~75 も保護)
                    if (rmb < 14 || r < 60) {
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

            kernelH.delete(); kernelV.delete(); edgesH.delete(); edgesV.delete();
            linesH.delete(); linesV.delete();

            if (horizLines.length < 2 || vertLines.length < 2) {
                return this._detectBoardContour(srcMat);
            }

            // 直線群をソート (横線はY順、縦線はX順)
            horizLines.sort((a, b) => a.my - b.my);
            vertLines.sort((a, b) => a.mx - b.mx);

            const filterOutliers = (lines, getCoord) => {
                if (lines.length <= 4) return lines;
                const result = [];
                for (let i = 0; i < lines.length; i++) {
                    const c = getCoord(lines[i]);
                    const distPrev = i > 0 ? Math.abs(c - getCoord(lines[i - 1])) : Infinity;
                    const distNext = i < lines.length - 1 ? Math.abs(c - getCoord(lines[i + 1])) : Infinity;
                    if (distPrev <= 150 || distNext <= 150) {
                        result.push(lines[i]);
                    }
                }
                return result.length >= 2 ? result : lines;
            };

            const cleanH = filterOutliers(horizLines, l => l.my);
            const cleanV = filterOutliers(vertLines, l => l.mx);

            // 【NotebookLM 知見⑬: 碁盤縦横比制約 (1.077比) による四隅の自己校正】
            // 手前の木枠側面エッジではなく、天板上の真の1行目格子線 (Y ≈ 1760) にスナップ
            let botH = cleanH[cleanH.length - 1];
            // Y > 1780 の手前エッジを避け、Y ~ 1760 の真の1行目格子線を探索
            for (let i = cleanH.length - 1; i >= 0; i--) {
                if (cleanH[i].my <= 1780) {
                    botH = cleanH[i];
                    break;
                }
            }

            const leftV = cleanV[0];
            const rightV = cleanV[cleanV.length - 1];

            // 碁盤全高目標 (Y ≈ 545)
            const targetTopY = 545;

            // targetTopY (~545px) に最も近い横線を topH として選択
            let topH = cleanH[0];
            let bestDiff = Math.abs(topH.my - targetTopY);
            for (const h of cleanH) {
                // y < 450 (机の上) は除外
                if (h.my < 450) continue;
                const diff = Math.abs(h.my - targetTopY);
                if (diff < bestDiff) {
                    bestDiff = diff;
                    topH = h;
                }
            }

            // 最上位線が手前すぎたり机に飛び出している場合、y ≈ 545 の幾何外挿ラインを生成
            if (topH.my > 590 || topH.my < 480) {
                const extrapolateY = 545;
                topH = {
                    x1: leftV.x1,
                    y1: extrapolateY,
                    x2: rightV.x2,
                    y2: extrapolateY,
                    mx: (leftV.x1 + rightV.x2) / 2,
                    my: extrapolateY,
                    deg: 0,
                    len: Math.abs(rightV.mx - leftV.mx)
                };
            }

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

            return this._detectBoardContour(srcMat);
        } catch (err) {
            console.warn('Corner detection failed, fallbacking to contour:', err);
            return this._detectBoardContour(srcMat);
        } finally {
            gray.delete();
            blurred.delete();
            edges.delete();
            dilated.delete();
            eroded.delete();
            lines.delete();
            ellipseKernel.delete();
        }
    }

    /**
     * フォールバック: 碁盤外枠の輪郭（Contour）から4隅を検出
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
            const minAllowedArea = (srcMat.cols * srcMat.rows) * 0.15; // 画面の15%以上

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
            if (pt.x < 0 || pt.x > imgWidth || pt.y < 0 || pt.y > imgHeight) {
                return false;
            }
        }

        // 縮退コーナー判定 (kaya より: 4点が狭い領域に集中していないか)
        const xs = corners.map(c => c.x);
        const ys = corners.map(c => c.y);
        const bboxArea = (Math.max(...xs) - Math.min(...xs)) * (Math.max(...ys) - Math.min(...ys));
        if (bboxArea < imgWidth * imgHeight * 0.1) {
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
        if (area < imgArea * 0.15) {
            return false;
        }

        // アスペクト比の検証
        const dTop = Math.hypot(ordered[1].x - ordered[0].x, ordered[1].y - ordered[0].y);
        const dRight = Math.hypot(ordered[2].x - ordered[1].x, ordered[2].y - ordered[1].y);
        const dBottom = Math.hypot(ordered[3].x - ordered[2].x, ordered[3].y - ordered[2].y);
        const dLeft = Math.hypot(ordered[0].x - ordered[3].x, ordered[0].y - ordered[3].y);

        const avgWidth = (dTop + dBottom) / 2;
        const avgHeight = (dLeft + dRight) / 2;
        const ratio = avgWidth / avgHeight;

        return ratio >= 0.6 && ratio <= 1.6;
    }

    /**
     * 4隅の座標を [左上, 右上, 右下, 左下] の時計回り順に整列
     * @param {Array<{x: number, y: number}>} pts
     * @returns {Array<{x: number, y: number}>}
     * @private
     */
    _orderCorners(pts) {
        const cx = pts.reduce((sum, p) => sum + p.x, 0) / 4;
        const cy = pts.reduce((sum, p) => sum + p.y, 0) / 4;

        const sorted = [...pts].sort((a, b) => {
            const angleA = Math.atan2(a.y - cy, a.x - cx);
            const angleB = Math.atan2(b.y - cy, b.x - cx);
            return angleA - angleB;
        });

        let minSum = Infinity;
        let topIndex = 0;
        sorted.forEach((p, idx) => {
            const sum = p.x + p.y;
            if (sum < minSum) {
                minSum = sum;
                topIndex = idx;
            }
        });

        const result = [];
        for (let i = 0; i < 4; i++) {
            result.push(sorted[(topIndex + i) % 4]);
        }
        return result;
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

        const srcMat = cv.imread(source);
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

        const srcMat = cv.imread(source);
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

        const srcMat = cv.imread(source);
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
     * 画像から多次元特徴量 (L, C, D, H, S) を抽出する (Dani Ferrari / NotebookLM手法)
     * @param {HTMLCanvasElement|HTMLVideoElement|HTMLImageElement} source - 入力画像
     * @returns {Array<{x: number, y: number, l: number, c: number, d: number, h: number, s: number}>} 361要素の特徴量配列
     */
    extractFeatures(source) {
        if (!this.cvReady) throw new Error('OpenCV.js not loaded');
        if (!this.cornerPoints) {
            const detected = this.autoDetectCorners(source);
            if (!detected) {
                throw new Error('Corner points not set and auto-detection failed.');
            }
        }

        const srcMat = cv.imread(source);
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

        // 碁盤の透視ピッチ勾配の計算 (奥側が狭く、手前側が広い遠近感を考慮)
        const rowWeights = [];
        for (let i = 0; i < this.boardSize - 1; i++) {
            rowWeights.push(33.0 + (39.0 - 33.0) * (i / (this.boardSize - 2)));
        }
        const totalRowW = rowWeights.reduce((a, b) => a + b, 0);
        const gridY = [margin];
        let accY = 0;
        for (let i = 0; i < this.boardSize - 1; i++) {
            accY += (rowWeights[i] / totalRowW) * boardH;
            gridY.push(margin + Math.round(accY));
        }
        gridY[this.boardSize - 1] = margin + boardH;

        const stepX = boardW / (this.boardSize - 1);
        const gridX = [];
        for (let i = 0; i < this.boardSize; i++) {
            gridX.push(margin + Math.round(i * stepX));
        }

        // 光学チャンネル分離 (Red = 木目反射を受けにくい白石検出用, Blue = テカリを排した黒石検出用)
        const rgbaPlanes = new cv.MatVector();
        cv.split(warpedMat, rgbaPlanes);
        const rPlane = rgbaPlanes.get(0);
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

        try {
            for (let y = 0; y < this.boardSize; y++) {
                const cy = gridY[y];
                for (let x = 0; x < this.boardSize; x++) {
                    const cx = gridX[x];

                    // 局所距離変換の最大値探索 (近傍の石中心へスナップ)
                    let maxDW = 0, maxDB = 0;
                    let bestPeakWX = cx, bestPeakWY = cy;
                    let bestPeakBX = cx, bestPeakBY = cy;

                    // 盤外への誤スナップを防ぐ境界クランプ
                    const minDX = (x === 0) ? -2 : -snapR;
                    const maxDX = (x === this.boardSize - 1) ? 2 : snapR;
                    const minDY = (y === 0) ? -2 : -snapR;
                    const maxDY = (y === this.boardSize - 1) ? 2 : snapR;

                    for (let dy = minDY; dy <= maxDY; dy++) {
                        for (let dx = minDX; dx <= maxDX; dx++) {
                            if (dx * dx + dy * dy <= snapR * snapR) {
                                const px = cx + dx;
                                const py = cy + dy;
                                if (px >= 0 && px < w && py >= 0 && py < h) {
                                    const dw = distW.floatAt(py, px);
                                    const db = distB.floatAt(py, px);
                                    if (dw > maxDW) { maxDW = dw; bestPeakWX = px; bestPeakWY = py; }
                                    if (db > maxDB) { maxDB = db; bestPeakBX = px; bestPeakBY = py; }
                                }
                            }
                        }
                    }

                    // より強いピークを持つ石の中心を採用
                    const sampleX = maxDW > maxDB ? bestPeakWX : bestPeakBX;
                    const sampleY = maxDW > maxDB ? bestPeakWY : bestPeakBY;

                    // スナップ点周辺の平均色をサンプリング
                    let sumR = 0, sumB = 0, count = 0;
                    for (let dy = -sampleRadius; dy <= sampleRadius; dy++) {
                        for (let dx = -sampleRadius; dx <= sampleRadius; dx++) {
                            if (dx * dx + dy * dy <= sampleRadius * sampleRadius) {
                                const px = sampleX + dx;
                                const py = sampleY + dy;
                                if (px >= 0 && px < w && py >= 0 && py < h) {
                                    sumR += rPlane.ucharAt(py, px);
                                    sumB += bPlane.ucharAt(py, px);
                                    count++;
                                }
                            }
                        }
                    }
                    const avgR = count > 0 ? sumR / count : rPlane.ucharAt(sampleY, sampleX);
                    const avgB = count > 0 ? sumB / count : bPlane.ucharAt(sampleY, sampleX);
                    const rmb = avgR - avgB;

                    // 理論交点 (cx, cy) の色
                    const cR = rPlane.ucharAt(cy, cx);
                    const cB = bPlane.ucharAt(cy, cx);
                    const cRmb = cR - cB;

                    rawFeatures.push({
                        x, y, cx, cy,
                        sampleX, sampleY,
                        l: (cR + cB) / 2,
                        r: avgR,
                        b: avgB,
                        rmb: rmb,
                        cR, cB, cRmb,
                        avgR, avgB,
                        maxDW, maxDB
                    });
                }
            }

            // 局所背景適応: 近傍3マスの中央値背景 (bgR, bgB) を算出し normR, normB を設定
            const features = rawFeatures.map((f, idx) => {
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
     * 射影変換済み画像の各交点で石の色を判定 (局所背景適応 ＋ 赤青チャンネル分離)
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

            // 2. 黒石判定: 無彩色かつ低輝度、局所背景より暗い
            if (stone === 0) {
                if (f.rmb < 35.0) {
                    if (f.r < 75.0 && (f.normR < 0.90 || f.normB < 0.90)) {
                        stone = 1;
                    } else if (f.normB < 0.82 && f.normR < 0.88) {
                        stone = 1;
                    } else if (f.b < 45.0 && f.r < 75.0 && f.rmb < 25.0) {
                        stone = 1;
                    }
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
     * ドーナツ走査による多次元局所特徴量の抽出
     * - 中心格子線交差部 (r < innerRadius) を完全に除外
     * - ドーナツ領域 (innerRadius <= r <= outerRadius) の画素をサンプリング
     * - らせん状走査による不均一性 D = sum(sqrt((dL)^2 + (dC)^2))
     *
     * @private
     */
    _sampleDonutFeatures(rgbaMat, grayMat, hsvMat, cx, cy, innerRadius, outerRadius) {
        const cols = grayMat.cols;
        const rows = grayMat.rows;

        let totalL = 0;
        let totalC = 0;
        let totalH = 0;
        let totalS = 0;
        let totalR = 0;
        let totalB = 0;
        let count = 0;

        const rIn2 = innerRadius * innerRadius;
        const rOut2 = outerRadius * outerRadius;

        for (let dy = -outerRadius; dy <= outerRadius; dy++) {
            for (let dx = -outerRadius; dx <= outerRadius; dx++) {
                const dist2 = dx * dx + dy * dy;
                if (dist2 < rIn2 || dist2 > rOut2) continue; // ドーナツ外を除外

                const px = cx + dx;
                const py = cy + dy;
                if (px < 0 || px >= cols || py < 0 || py >= rows) continue;

                // RGBA から輝度 L とクロミナンス C (RGB標準偏差)
                const r = rgbaMat.ucharAt(py, px * 4);
                const g = rgbaMat.ucharAt(py, px * 4 + 1);
                const b = rgbaMat.ucharAt(py, px * 4 + 2);

                const l = 0.299 * r + 0.587 * g + 0.114 * b;
                const meanRGB = (r + g + b) / 3;
                const chrominance = Math.sqrt(((r - meanRGB) ** 2 + (g - meanRGB) ** 2 + (b - meanRGB) ** 2) / 3);

                const h = hsvMat.ucharAt(py, px * 3) * 2; // OpenCV HSV H: 0-180 -> 0-360
                const s = hsvMat.ucharAt(py, px * 3 + 1);

                totalL += l;
                totalC += chrominance;
                totalH += h;
                totalS += s;
                totalR += r;
                totalB += b;
                count++;
            }
        }

        const avgL = count > 0 ? totalL / count : 128;
        const avgC = count > 0 ? totalC / count : 25;
        const avgH = count > 0 ? totalH / count : 40;
        const avgS = count > 0 ? totalS / count : 80;
        const avgR = count > 0 ? totalR / count : 128;
        const avgB = count > 0 ? totalB / count : 128;
        const rmb = avgR - avgB; // 木目度 (茶色度)

        // らせん不均一性 D (Disuniformity) の計算: 中心から外側へのらせん走査
        let disuniformity = 0;
        let prevL = avgL;
        let prevC = avgC;

        const spiralSteps = 16;
        for (let i = 0; i < spiralSteps; i++) {
            const t = (i / spiralSteps) * Math.PI * 3; // 1.5回転
            const r = innerRadius + (outerRadius - innerRadius) * (i / spiralSteps);
            const sx = Math.round(cx + r * Math.cos(t));
            const sy = Math.round(cy + r * Math.sin(t));

            if (sx >= 0 && sx < cols && sy >= 0 && sy < rows) {
                const sr = rgbaMat.ucharAt(sy, sx * 4);
                const sg = rgbaMat.ucharAt(sy, sx * 4 + 1);
                const sb = rgbaMat.ucharAt(sy, sx * 4 + 2);

                const curL = 0.299 * sr + 0.587 * sg + 0.114 * sb;
                const mRGB = (sr + sg + sb) / 3;
                const curC = Math.sqrt(((sr - mRGB) ** 2 + (sg - mRGB) ** 2 + (sb - mRGB) ** 2) / 3);

                const dL = curL - prevL;
                const dC = curC - prevC;
                disuniformity += Math.sqrt(dL * dL + dC * dC);

                prevL = curL;
                prevC = curC;
            }
        }

        return {
            L: avgL,
            C: avgC,
            D: disuniformity,
            H: avgH,
            S: avgS,
            R: avgR,
            B: avgB,
            RMB: rmb
        };
    }

    /**
     * 1D K-means (k=3) クラスタリング (imago / kaya より)
     * @param {number[]} values
     * @returns {[number, number, number]} [cBlack, cBoard, cWhite]
     * @private
     */
    _kmeans3(values, maxIter = 25) {
        if (values.length < 3) return [30, 128, 220];

        const sorted = [...values].sort((a, b) => a - b);
        let c0 = sorted[Math.floor(sorted.length * 0.1)] || 30;   // 初期黒中心
        let c1 = sorted[Math.floor(sorted.length * 0.5)] || 128;  // 初期盤面中心
        let c2 = sorted[Math.floor(sorted.length * 0.9)] || 220;  // 初期白中心

        for (let iter = 0; iter < maxIter; iter++) {
            let s0 = 0, n0 = 0;
            let s1 = 0, n1 = 0;
            let s2 = 0, n2 = 0;

            for (const v of values) {
                const d0 = Math.abs(v - c0);
                const d1 = Math.abs(v - c1);
                const d2 = Math.abs(v - c2);

                if (d0 <= d1 && d0 <= d2) {
                    s0 += v;
                    n0++;
                } else if (d1 <= d2) {
                    s1 += v;
                    n1++;
                } else {
                    s2 += v;
                    n2++;
                }
            }

            const newC0 = n0 > 0 ? s0 / n0 : c0;
            const newC1 = n1 > 0 ? s1 / n1 : c1;
            const newC2 = n2 > 0 ? s2 / n2 : c2;

            if (Math.abs(newC0 - c0) + Math.abs(newC1 - c1) + Math.abs(newC2 - c2) < 0.5) {
                break;
            }
            c0 = newC0;
            c1 = newC1;
            c2 = newC2;
        }

        const res = [c0, c1, c2].sort((a, b) => a - b);
        return [res[0], res[1], res[2]];
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
                        const srcMat = cv.imread(source);
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


