/**
 * WasmKifu - Go WASM 棋譜記録ライブラリのJSブリッジ
 *
 * Go WASM モジュールのロードと syscall/js でグローバルに登録された関数の呼び出しを提供する。
 * ブラウザ環境で使用する。
 */
class WasmKifu {
    constructor() {
        this.wasmReady = false;
        this.instance = null;
    }

    /**
     * WASM モジュールをロードする
     * @param {string} wasmPath - .wasm ファイルのパス
     * @returns {Promise<void>}
     */
    async load(wasmPath) {
        const go = new Go(); // wasm_exec.js が提供するグローバル
        const result = await WebAssembly.instantiateStreaming(
            fetch(wasmPath),
            go.importObject
        );
        this.instance = result.instance;
        go.run(this.instance); // バックグラウンドで Go ランタイム実行
        this.wasmReady = true;
    }

    /**
     * WASM モジュールの準備ができているか
     * @returns {boolean}
     */
    isReady() {
        return this.wasmReady;
    }

    /** 新規ゲームを開始（19路盤） */
    newGame() {
        this._checkReady();
        wasmNewGame();
    }

    /**
     * プレイヤー情報付きで新規ゲーム開始
     * @param {string} playerBlack - 黒番プレイヤー名
     * @param {string} playerWhite - 白番プレイヤー名
     * @param {number} komi - コミ
     */
    newGameWithInfo(playerBlack, playerWhite, komi) {
        this._checkReady();
        wasmNewGameWithInfo(playerBlack, playerWhite, komi);
    }

    /**
     * 着手する
     * @param {number} x - X座標 (0-18)
     * @param {number} y - Y座標 (0-18)
     * @returns {number} 0=成功, 1=違法手
     */
    playMove(x, y) {
        this._checkReady();
        return wasmPlayMove(x, y);
    }

    /** パス */
    pass() {
        this._checkReady();
        wasmPass();
    }

    /** 投了 */
    resign() {
        this._checkReady();
        wasmResign();
    }

    /**
     * 直前の着手を取り消す
     * @returns {boolean} 成功したかどうか
     */
    undo() {
        this._checkReady();
        return wasmUndo() === 0;
    }

    /**
     * SGF文字列をエクスポート
     * @returns {string} SGF文字列
     */
    exportSGF() {
        this._checkReady();
        return wasmExportSGF();
    }

    /**
     * SGF文字列をインポートしてゲームを復元
     * @param {string} sgfString - SGF文字列
     * @returns {boolean} 成功したかどうか
     */
    importSGF(sgfString) {
        this._checkReady();
        return wasmImportSGF(sgfString) === 0;
    }

    /**
     * 現在の手番を取得
     * @returns {number} 1=黒, 2=白
     */
    getCurrentPlayer() {
        this._checkReady();
        return wasmGetCurrentPlayer();
    }

    /**
     * 現在の手数を取得
     * @returns {number}
     */
    getMoveNumber() {
        this._checkReady();
        return wasmGetMoveNumber();
    }

    /**
     * 盤面状態を取得
     * @returns {Object} BoardState {stones: number[][], player: number, moveNumber: number, ...}
     */
    getBoardState() {
        this._checkReady();
        const json = wasmGetBoardState();
        return JSON.parse(json);
    }

    /**
     * 画像認識の結果を解析して着手を推定
     * @param {Int8Array} intersections - 361要素の交点状態配列 (0=空, 1=黒, 2=白)
     * @returns {Object} AnalysisResult {moves: Array, removed: Array, errors: Array, confidence: number}
     */
    analyzeBoardImage(intersections) {
        this._checkReady();
        const data = JSON.stringify({ intersections: Array.from(intersections) });
        const result = wasmAnalyzeBoardImage(data, 19, 19);
        return JSON.parse(result);
    }

    /**
     * 検出された着手を適用
     * @param {number} x - X座標 (0-18)
     * @param {number} y - Y座標 (0-18)
     * @param {number} color - 1=黒, 2=白
     * @returns {number} 0=成功, 1=違法手, 2=ゲーム終了
     */
    applyDetectedMove(x, y, color) {
        this._checkReady();
        return wasmApplyDetectedMove(x, y, color);
    }

    /**
     * 直前の着手情報を取得
     * @returns {Object} {x: number, y: number, color: string, moveNumber: number, isPass: boolean}
     */
    getLastMoveInfo() {
        this._checkReady();
        const json = wasmGetLastMoveInfo();
        return JSON.parse(json);
    }

    /** @private */
    _checkReady() {
        if (!this.wasmReady) {
            throw new Error('WASM module not loaded. Call load() first.');
        }
    }
}

export { WasmKifu };
