/**
 * MLCornerDetector - 碁盤の4隅キーポイント超軽量機械学習検出モジュール
 *
 * onnxruntime-web を用いてブラウザ上で超軽量キーポイント検出モデル（YOLOv8n-pose / 軽量CNN）を
 * 実行し、角度・照明・影・外周の碁石乱れに左右されずに碁盤の19路格子4隅 [TL, TR, BR, BL] を高精度に検出する。
 */

export class MLCornerDetector {
    /**
     * @param {Object} [options]
     * @param {string} [options.modelPath='/model/goban-corners.onnx'] - ONNX モデルのパス
     * @param {number} [options.inputSize=640] - モデル入力画像解像度
     * @param {number} [options.confidenceThreshold=0.3] - 信頼度閾値
     */
    constructor(options = {}) {
        this.modelPath = options.modelPath || '/model/goban-corners.onnx';
        this.inputSize = options.inputSize || 640;
        this.confidenceThreshold = options.confidenceThreshold || 0.3;
        this.session = null;
        this.ort = null;
        this.isLoading = false;
        this.isReady = false;
    }

    /**
     * ONNX Runtime Web およびモデルをロード
     * @param {string} [modelPath]
     * @returns {Promise<boolean>}
     */
    async load(modelPath = null) {
        if (modelPath) this.modelPath = modelPath;
        if (this.isReady && this.session) return true;
        if (this.isLoading) return false;

        this.isLoading = true;
        try {
            // onnxruntime-web の取得 (グローバル window.ort または動的 import)
            if (typeof window !== 'undefined' && window.ort) {
                this.ort = window.ort;
            } else if (typeof globalThis !== 'undefined' && globalThis.ort) {
                this.ort = globalThis.ort;
            } else {
                try {
                    const modName = 'onnxruntime-web';
                    this.ort = await import(/* @vite-ignore */ modName);
                } catch {
                    if (typeof window !== 'undefined' && window.ort) {
                        this.ort = window.ort;
                    }
                }
            }

            if (!this.ort) {
                console.warn('[MLCornerDetector] onnxruntime-web is not available. Falling back to geometric detection.');
                this.isLoading = false;
                return false;
            }

            // WebAssembly / WebGL バックエンド設定
            if (this.ort.env && this.ort.env.wasm) {
                this.ort.env.wasm.numThreads = 1; // モバイル・ブラウザ互換性の安全値
                this.ort.env.wasm.simd = true;
            }

            const sessionOptions = {
                executionProviders: ['wasm'],
                graphOptimizationLevel: 'all',
            };

            let modelSource = this.modelPath;
            if (typeof fetch === 'function' && typeof this.modelPath === 'string') {
                const resp = await fetch(this.modelPath);
                if (!resp.ok) {
                    throw new Error(`HTTP ${resp.status} fetching ${this.modelPath}`);
                }
                const buf = await resp.arrayBuffer();
                // プレースホルダー（ダミー定数モデル < 4KB または 'kifu-init-model'）の検知とスキップ
                const headerStr = new TextDecoder('utf-8', { fatal: false }).decode(
                    new Uint8Array(buf, 0, Math.min(buf.byteLength, 512))
                );
                if (buf.byteLength < 4096 || headerStr.includes('kifu-init-model')) {
                    console.info('[MLCornerDetector] Placeholder ONNX model detected. Delegating corner detection to Go WASM engine.');
                    this.session = null;
                    this.isReady = false;
                    this.isLoading = false;
                    return false;
                }
                modelSource = buf;
            }

            this.session = await this.ort.InferenceSession.create(modelSource, sessionOptions);
            this.isReady = true;
            this.isLoading = false;
            return true;
        } catch (err) {
            console.warn('[MLCornerDetector] Failed to load ONNX model:', err.message || err);
            this.session = null;
            this.isReady = false;
            this.isLoading = false;
            return false;
        }
    }

    /**
     * 入力画像から4隅キーポイント [TL, TR, BR, BL] を検出
     * @param {HTMLImageElement|HTMLCanvasElement|HTMLVideoElement|ImageData} imageSource
     * @param {Object} [fallbackWasmKifu=null] - モデル未読み込み時の Go WASM フォールバックインスタンス
     * @returns {Promise<{corners: Array<{x: number, y: number}>, confidence: number, method: string}>}
     */
    async detect(imageSource, fallbackWasmKifu = null) {
        const { srcW, srcH, canvas } = this._getCanvasFromSource(imageSource);

        if (!this.isReady || !this.session) {
            // WASM または幾何学フォールバック
            return this._fallbackDetection(canvas, srcW, srcH, fallbackWasmKifu);
        }

        try {
            // Step 1: 640x640 レターボックス変換および CHW テンソル作成
            const { tensor, scale, padX, padY } = this._preprocess(canvas, srcW, srcH);

            // Step 2: ONNX 推論実行
            const inputName = this.session.inputNames[0];
            const feeds = {};
            feeds[inputName] = tensor;
            const results = await this.session.run(feeds);

            // Step 3: 出力テンソルのパース (直接4点回帰またはYOLOv8-pose)
            const outputTensor = results[this.session.outputNames[0]];
            const parsed = this._postprocess(outputTensor, scale, padX, padY, srcW, srcH);

            if (parsed && parsed.confidence >= this.confidenceThreshold) {
                return {
                    corners: this._orderCorners(parsed.corners),
                    confidence: parsed.confidence,
                    method: 'ML-Keypoints',
                };
            }
        } catch (err) {
            console.warn('[MLCornerDetector] Inference error, falling back:', err);
        }

        return this._fallbackDetection(canvas, srcW, srcH, fallbackWasmKifu);
    }

    /**
     * 画像ソースから Canvas を抽出
     * @private
     */
    _getCanvasFromSource(source) {
        let srcW = 0, srcH = 0;
        let canvas = null;

        if (source instanceof HTMLCanvasElement) {
            srcW = source.width;
            srcH = source.height;
            canvas = source;
        } else if (source instanceof HTMLImageElement || source instanceof HTMLVideoElement) {
            srcW = source.videoWidth || source.naturalWidth || source.width;
            srcH = source.videoHeight || source.naturalHeight || source.height;
            canvas = document.createElement('canvas');
            canvas.width = srcW;
            canvas.height = srcH;
            const ctx = canvas.getContext('2d');
            ctx.drawImage(source, 0, 0, srcW, srcH);
        } else if (typeof ImageData !== 'undefined' && source instanceof ImageData) {
            srcW = source.width;
            srcH = source.height;
            canvas = document.createElement('canvas');
            canvas.width = srcW;
            canvas.height = srcH;
            const ctx = canvas.getContext('2d');
            ctx.putImageData(source, 0, 0);
        } else {
            throw new Error('Unsupported image source type for MLCornerDetector');
        }

        return { srcW, srcH, canvas };
    }

    /**
     * レターボックスリサイズおよび CHW テンソルの生成
     * @private
     */
    _preprocess(srcCanvas, srcW, srcH) {
        const inSize = this.inputSize;
        const scale = Math.min(inSize / srcW, inSize / srcH);
        const newW = Math.round(srcW * scale);
        const newH = Math.round(srcH * scale);
        const padX = Math.floor((inSize - newW) / 2);
        const padY = Math.floor((inSize - newH) / 2);

        const tempCanvas = document.createElement('canvas');
        tempCanvas.width = inSize;
        tempCanvas.height = inSize;
        const ctx = tempCanvas.getContext('2d');

        // 背景を 114 (YOLO 標準グレー) で塗りつぶし
        ctx.fillStyle = '#727272';
        ctx.fillRect(0, 0, inSize, inSize);
        ctx.drawImage(srcCanvas, 0, 0, srcW, srcH, padX, padY, newW, newH);

        const imgData = ctx.getImageData(0, 0, inSize, inSize);
        const data = imgData.data;

        // Float32Array [1, 3, inSize, inSize] (RGB 順, 0.0〜1.0)
        const floatData = new Float32Array(3 * inSize * inSize);
        const channelSize = inSize * inSize;

        for (let i = 0; i < channelSize; i++) {
            const ptr = i * 4;
            floatData[i] = data[ptr] / 255.0;                         // R
            floatData[channelSize + i] = data[ptr + 1] / 255.0;       // G
            floatData[2 * channelSize + i] = data[ptr + 2] / 255.0;   // B
        }

        const tensor = new this.ort.Tensor('float32', floatData, [1, 3, inSize, inSize]);
        return { tensor, scale, padX, padY };
    }

    /**
     * 出力テンソルのデコード
     * 形式1: 直接回帰 [1, 8] または [1, 4, 2] (TL, TR, BR, BL)
     * 形式2: YOLOv8-pose [1, 17, 8400] (4キーポイント x, y, conf)
     * @private
     */
    _postprocess(tensor, scale, padX, padY, origW, origH) {
        if (!tensor || !tensor.data) return null;
        const dims = tensor.dims;
        const data = tensor.data;

        // 形式 1: 直接回帰モデル [1, 8] または [1, 4, 2] (正規化座標 0.0〜1.0)
        if (dims.length === 2 && dims[1] === 8) {
            // 定数ダミー出力 [0.1, 0.1, 0.9, 0.1, 0.9, 0.9, 0.1, 0.9] の場合は無効化
            const dummyExpected = [0.1, 0.1, 0.9, 0.1, 0.9, 0.9, 0.1, 0.9];
            const isDummy = dummyExpected.every((v, idx) => Math.abs((data[idx] ?? 0) - v) < 1e-3);
            if (isDummy) {
                return null;
            }

            const corners = [];
            for (let i = 0; i < 4; i++) {
                const normX = data[i * 2];
                const normY = data[i * 2 + 1];
                // レターボックス補正
                const lx = normX * this.inputSize;
                const ly = normY * this.inputSize;
                const ox = Math.max(0, Math.min(origW, (lx - padX) / scale));
                const oy = Math.max(0, Math.min(origH, (ly - padY) / scale));
                corners.push({ x: ox, y: oy });
            }
            return { corners, confidence: 0.95 };
        }

        // 形式 2: YOLOv8-pose 形式 [1, 5 + 4*3, numAnchors]
        // [cx, cy, w, h, conf, k1_x, k1_y, k1_conf, k2_x, k2_y, k2_conf, ...]
        if (dims.length === 3) {
            const channels = dims[1];
            const numAnchors = dims[2];

            let bestScore = -1;
            let bestAnchor = -1;

            // クラススコアまたはボックス信頼度が最大のアンカーを探索
            for (let a = 0; a < numAnchors; a++) {
                const conf = data[4 * numAnchors + a];
                if (conf > bestScore) {
                    bestScore = conf;
                    bestAnchor = a;
                }
            }

            if (bestAnchor !== -1 && bestScore >= this.confidenceThreshold) {
                const corners = [];
                // 4つのキーポイント (0: TL, 1: TR, 2: BR, 3: BL)
                for (let k = 0; k < 4; k++) {
                    const kxOffset = (5 + k * 3) * numAnchors + bestAnchor;
                    const kyOffset = (5 + k * 3 + 1) * numAnchors + bestAnchor;
                    const lx = data[kxOffset];
                    const ly = data[kyOffset];
                    const ox = Math.max(0, Math.min(origW, (lx - padX) / scale));
                    const oy = Math.max(0, Math.min(origH, (ly - padY) / scale));
                    corners.push({ x: ox, y: oy });
                }
                return { corners, confidence: bestScore };
            }
        }

        return null;
    }

    /**
     * 4隅座標を時計回り [TL, TR, BR, BL] に正準化
     * @private
     */
    _orderCorners(pts) {
        if (!pts || pts.length !== 4) return pts;
        const sortedY = [...pts].sort((a, b) => a.y - b.y);
        const top1 = sortedY[0], top2 = sortedY[1];
        const bot1 = sortedY[2], bot2 = sortedY[3];

        let tl, tr, br, bl;
        if (top1.x < top2.x) {
            tl = top1; tr = top2;
        } else {
            tl = top2; tr = top1;
        }
        if (bot1.x > bot2.x) {
            br = bot1; bl = bot2;
        } else {
            br = bot2; bl = bot1;
        }
        return [tl, tr, br, bl];
    }

    /**
     * モデル未初期化または失敗時の安全なフォールバック
     * @private
     */
    _fallbackDetection(canvas, width, height, fallbackWasmKifu) {
        if (fallbackWasmKifu && fallbackWasmKifu.isReady && fallbackWasmKifu.isReady()) {
            try {
                const ctx = canvas.getContext('2d');
                const imgData = ctx.getImageData(0, 0, width, height);
                const res = fallbackWasmKifu.detectBoardCorners(imgData.data, width, height);
                if (res && res.corners && res.corners.length === 4) {
                    return {
                        corners: this._orderCorners(res.corners),
                        confidence: res.confidence || 0.7,
                        method: res.method || 'GoWASM-Hough',
                    };
                }
            } catch (err) {
                console.warn('[MLCornerDetector] Go WASM fallback error:', err);
            }
        }

        // デフォルト矩形フォールバック（画像内寸 80%）
        const insetX = width * 0.10;
        const insetY = height * 0.10;
        return {
            corners: [
                { x: insetX, y: insetY },
                { x: width - insetX, y: insetY },
                { x: width - insetX, y: height - insetY },
                { x: insetX, y: height - insetY },
            ],
            confidence: 0.5,
            method: 'DefaultRect',
        };
    }
}
