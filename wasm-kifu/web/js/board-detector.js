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
     * 画像から盤面状態を検出する
     * @param {HTMLCanvasElement|HTMLVideoElement|HTMLImageElement} source - 入力画像
     * @returns {Int8Array} 361要素の交点状態配列 (0=空, 1=黒, 2=白)
     */
    detect(source) {
        if (!this.cvReady) throw new Error('OpenCV.js not loaded');
        if (!this.cornerPoints) throw new Error('Corner points not set. Call setCorners() first.');

        // ソースからMatに変換
        const srcMat = cv.imread(source);

        try {
            // 射影変換で正面図に補正
            const warped = this._perspectiveTransform(srcMat);

            try {
                // 各交点の石を判定
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

        // ソース4点（左上, 右上, 右下, 左下）
        const srcPoints = cv.matFromArray(4, 1, cv.CV_32FC2, [
            this.cornerPoints[0].x, this.cornerPoints[0].y,
            this.cornerPoints[1].x, this.cornerPoints[1].y,
            this.cornerPoints[2].x, this.cornerPoints[2].y,
            this.cornerPoints[3].x, this.cornerPoints[3].y,
        ]);

        // デスティネーション4点（正方形）
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
     * 射影変換済み画像の各交点で石の色を判定
     * @param {cv.Mat} warpedMat - 射影変換済み画像
     * @returns {Int8Array} 交点状態配列
     * @private
     */
    _classifyIntersections(warpedMat) {
        const result = new Int8Array(this.boardSize * this.boardSize);
        const spacing = warpedMat.cols / (this.boardSize - 1); // ≈ 31.67px
        const sampleRadius = Math.floor(spacing * 0.3);

        // グレースケール変換
        const gray = new cv.Mat();
        cv.cvtColor(warpedMat, gray, cv.COLOR_RGBA2GRAY);

        // HSV変換（色情報も利用）
        const hsv = new cv.Mat();
        const bgr = new cv.Mat();
        cv.cvtColor(warpedMat, bgr, cv.COLOR_RGBA2BGR);
        cv.cvtColor(bgr, hsv, cv.COLOR_BGR2HSV);

        try {
            for (let y = 0; y < this.boardSize; y++) {
                for (let x = 0; x < this.boardSize; x++) {
                    const cx = Math.round(x * spacing);
                    const cy = Math.round(y * spacing);

                    result[y * this.boardSize + x] = this._classifyPoint(
                        gray, hsv, cx, cy, sampleRadius
                    );
                }
            }
        } finally {
            gray.delete();
            hsv.delete();
            bgr.delete();
        }

        return result;
    }

    /**
     * 1つの交点での石の判定
     * 円形サンプリング領域内のピクセル輝度・彩度を集計して判定
     *
     * @param {cv.Mat} grayMat - グレースケール画像
     * @param {cv.Mat} hsvMat - HSV画像
     * @param {number} cx - 交点のX座標（ピクセル）
     * @param {number} cy - 交点のY座標（ピクセル）
     * @param {number} radius - サンプリング半径（ピクセル）
     * @returns {number} 0=空, 1=黒, 2=白
     * @private
     */
    _classifyPoint(grayMat, hsvMat, cx, cy, radius) {
        let totalBrightness = 0;
        let totalSaturation = 0;
        let count = 0;

        const rows = grayMat.rows;
        const cols = grayMat.cols;

        for (let dy = -radius; dy <= radius; dy++) {
            for (let dx = -radius; dx <= radius; dx++) {
                // 円形領域のみサンプリング
                if (dx * dx + dy * dy > radius * radius) continue;

                const px = cx + dx;
                const py = cy + dy;
                if (px < 0 || px >= cols || py < 0 || py >= rows) continue;

                totalBrightness += grayMat.ucharAt(py, px);
                // HSV Mat は3チャンネル: [H, S, V]
                totalSaturation += hsvMat.ucharAt(py, px * 3 + 1);
                count++;
            }
        }

        if (count === 0) return 0;

        const avgBrightness = totalBrightness / count;
        const avgSaturation = totalSaturation / count;

        // 閾値による判定
        const { blackMaxBrightness, whiteMinBrightness, whiteMaxSaturation } = this.thresholds;

        // 黒石: 低輝度
        if (avgBrightness < blackMaxBrightness) return 1;
        // 白石: 高輝度 + 低彩度（碁盤の木目は高彩度のため区別可能）
        if (avgBrightness > whiteMinBrightness && avgSaturation < whiteMaxSaturation) return 2;
        // それ以外: 空（碁盤の木目）
        return 0;
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
