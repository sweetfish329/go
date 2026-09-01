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
                    if (mx < width * 0.03 || mx > width * 0.97 || my < height * 0.08 || my > height * 0.92) {
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

            // 最外周の横線と縦線を選択 (外れ値除外: 1% - 99%)
            const topH = horizLines[Math.floor(horizLines.length * 0.02)];
            const botH = horizLines[Math.floor(horizLines.length * 0.98)];
            const leftV = vertLines[Math.floor(vertLines.length * 0.02)];
            const rightV = vertLines[Math.floor(vertLines.length * 0.98)];

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
                // 順序: [左上, 右上, 右下, 左下]
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
     * 射影変換で碁盤を正面から見た画像に補正
     * @param {cv.Mat} srcMat - 入力画像
     * @returns {cv.Mat} 射影変換済み画像
     * @private
     */
    _perspectiveTransform(srcMat) {
        const dstSize = 570; // 19路盤: 30px間隔 × 19 = 570

        const srcPoints = cv.matFromArray(4, 1, cv.CV_32FC2, [
            this.cornerPoints[0].x, this.cornerPoints[0].y,
            this.cornerPoints[1].x, this.cornerPoints[1].y,
            this.cornerPoints[2].x, this.cornerPoints[2].y,
            this.cornerPoints[3].x, this.cornerPoints[3].y,
        ]);

        const dstPoints = cv.matFromArray(4, 1, cv.CV_32FC2, [
            0, 0,
            dstSize, 0,
            dstSize, dstSize,
            0, dstSize,
        ]);

        const transformMatrix = cv.getPerspectiveTransform(srcPoints, dstPoints);
        const warped = new cv.Mat();
        const dsize = new cv.Size(dstSize, dstSize);
        cv.warpPerspective(srcMat, warped, transformMatrix, dsize);

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
        const spacing = warpedMat.cols / (this.boardSize - 1);
        const innerRadius = Math.max(2, Math.floor(spacing * 0.14)); // 格子線交差部を除外
        const outerRadius = Math.floor(spacing * 0.35);             // ドーナツ走査外径

        const equalized = this._applyCLAHE(warpedMat);
        const smoothed = new cv.Mat();
        cv.GaussianBlur(equalized, smoothed, new cv.Size(3, 3), 0);

        const gray = new cv.Mat();
        cv.cvtColor(smoothed, gray, cv.COLOR_RGBA2GRAY);

        const hsv = new cv.Mat();
        const bgr = new cv.Mat();
        cv.cvtColor(smoothed, bgr, cv.COLOR_RGBA2BGR);
        cv.cvtColor(bgr, hsv, cv.COLOR_BGR2HSV);

        const features = [];

        try {
            for (let y = 0; y < this.boardSize; y++) {
                for (let x = 0; x < this.boardSize; x++) {
                    const cx = (this.recordedSpacingsX && this.recordedSpacingsX.length === this.boardSize)
                        ? this.recordedSpacingsX[x]
                        : Math.round(x * spacing);
                    const cy = (this.recordedSpacingsY && this.recordedSpacingsY.length === this.boardSize)
                        ? this.recordedSpacingsY[y]
                        : Math.round(y * spacing);

                    const feat = this._sampleDonutFeatures(smoothed, gray, hsv, cx, cy, innerRadius, outerRadius);
                    features.push({
                        x,
                        y,
                        l: feat.L,
                        c: feat.C,
                        d: feat.D,
                        h: feat.H,
                        s: feat.S,
                        rmb: feat.RMB
                    });
                }
            }
            return features;
        } finally {
            equalized.delete();
            smoothed.delete();
            gray.delete();
            hsv.delete();
            bgr.delete();
        }
    }

    /**
     * 射影変換済み画像の各交点で石の色を判定
     * 【NotebookLM / imago / kaya / Dani Ferrari 統合】
     * 1. ドーナツ走査 (中心格子線除外)
     * 2. 多次元局所特徴量 (L:輝度, C:クロミナンス, D:不均一性, H:色相, S:彩度)
     * 3. 色度と不均一性による影・木目分離 ＋ 1D K-means
     *
     * @param {cv.Mat} warpedMat - 射影変換済み画像
     * @returns {Int8Array} 交点状態配列
     * @private
     */
    _classifyIntersections(warpedMat) {
        const result = new Int8Array(this.boardSize * this.boardSize);
        const features = this._extractFeaturesFromWarped(warpedMat);
        const brightnessList = features.map(f => f.l);

        // 1D K-means (k=3) で大域的なクラスタ中心 [cBlack, cBoard, cWhite] を算出
        const [cBlack, cBoard, cWhite] = this._kmeans3(brightnessList);
        const splitBlack = (cBlack + cBoard) / 2;
        const splitWhite = (cBoard + cWhite) / 2;

        // 局所背景適応 (Local Background Estimation)
        // 各交点の近傍 (半径2交点) の輝度中央値を算出し、局所的な影・照明ムラを正規化
        const localNormL = new Float32Array(features.length);
        for (let y = 0; y < this.boardSize; y++) {
            for (let x = 0; x < this.boardSize; x++) {
                const neighbors = [];
                for (let dy = -2; dy <= 2; dy++) {
                    for (let dx = -2; dx <= 2; dx++) {
                        const nx = x + dx;
                        const ny = y + dy;
                        if (nx >= 0 && nx < this.boardSize && ny >= 0 && ny < this.boardSize) {
                            neighbors.push(features[ny * this.boardSize + nx].l);
                        }
                    }
                }
                neighbors.sort((a, b) => a - b);
                const localBg = neighbors[Math.floor(neighbors.length / 2)] || 128;
                localNormL[y * this.boardSize + x] = features[y * this.boardSize + x].l / Math.max(10, localBg);
            }
        }

        for (let i = 0; i < features.length; i++) {
            const f = features[i];
            const normL = localNormL[i];
            const x = f.x;
            const y = f.y;
            const isBorder = (x === 0 || x === this.boardSize - 1 || y === 0 || y === this.boardSize - 1);

            let stone = 0;

            // 盤外・最外周境界の影保護
            if (x === 0 || (y === 0 && f.l < 100) || (y === this.boardSize - 1 && f.l < 100)) {
                result[i] = 0;
                continue;
            }

            // 白石判定: 木目度が低く、高輝度または局所背景より明確に明るい
            if (f.rmb < 30 && f.c < 25) {
                if (f.l > 160 || (normL > 1.20 && f.l > 120)) {
                    stone = 2;
                } else if (f.rmb < 10 && normL > 1.05 && f.l > 105) {
                    stone = 2;
                }
            }
            if (stone === 0 && f.l > 185 && f.s < 85 && f.rmb < 35) {
                stone = 2;
            }

            // 黒石判定: 無彩色かつ木目度が低く、低輝度または局所背景より明確に暗い
            if (stone === 0) {
                if (!isBorder && f.rmb < 20 && f.c < 16) {
                    if (f.l < 75 || normL < 0.72) {
                        stone = 1;
                    } else if (f.l < 95 && f.c < 8 && f.rmb < 10) {
                        stone = 1;
                    }
                }
                if (stone === 0 && !isBorder && normL < 0.60 && f.c < 18 && f.rmb < 25) {
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
     * ストリーム追跡（リアルタイム動画）用ステートの初期化
     */
    initStreamTracker() {
        this.streamState = {
            prevGrayMat: null,
            prevStableWarpedMat: null,
            stableFrameCount: 0,
            requiredStableFrames: 12, // 12フレーム連続で静止したら着手確定
            motionThreshold: 8.0,     // 平均差分輝度の閾値
            isHandPresent: false,
        };
    }

    /**
     * ストリームフレームの動画追跡・静止安定トリガー処理 (Phase 2)
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

        const srcMat = cv.imread(source);
        try {
            const warped = this._perspectiveTransform(srcMat);
            try {
                const curGray = new cv.Mat();
                cv.cvtColor(warped, curGray, cv.COLOR_RGBA2GRAY);

                let motionLevel = 0;
                let isTriggered = false;
                let features = null;

                if (this.streamState.prevGrayMat) {
                    const diff = new cv.Mat();
                    cv.absdiff(curGray, this.streamState.prevGrayMat, diff);
                    const meanScalar = cv.mean(diff);
                    motionLevel = meanScalar[0]; // 平均変化量
                    diff.delete();

                    // 動体判定 (手の有無)
                    if (motionLevel > this.streamState.motionThreshold) {
                        this.streamState.isHandPresent = true;
                        this.streamState.stableFrameCount = 0;
                    } else {
                        // 静止状態カウント
                        this.streamState.stableFrameCount++;
                        if (this.streamState.stableFrameCount >= this.streamState.requiredStableFrames) {
                            if (this.streamState.isHandPresent) {
                                // 手が離れて静止状態に移行した瞬間 -> トリガー発火！
                                isTriggered = true;
                                this.streamState.isHandPresent = false;
                                features = this._extractFeaturesFromWarped(warped);
                            }
                        }
                    }

                    this.streamState.prevGrayMat.delete();
                }

                this.streamState.prevGrayMat = curGray.clone();
                curGray.delete();

                const stableProgress = Math.min(1.0, this.streamState.stableFrameCount / this.streamState.requiredStableFrames);

                return {
                    motionLevel: Math.round(motionLevel * 10) / 10,
                    stableProgress,
                    isStable: this.streamState.stableFrameCount >= this.streamState.requiredStableFrames,
                    isTriggered,
                    features
                };
            } finally {
                warped.delete();
            }
        } finally {
            srcMat.delete();
        }
    }

    /**
     * ストリーム追跡のリソース解放
     */
    resetStreamTracker() {
        if (this.streamState) {
            if (this.streamState.prevGrayMat) {
                this.streamState.prevGrayMat.delete();
            }
            if (this.streamState.prevStableWarpedMat) {
                this.streamState.prevStableWarpedMat.delete();
            }
            this.streamState = null;
        }
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


