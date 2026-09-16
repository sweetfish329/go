/// <reference types="vite/client" />

declare class Go {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

declare const cv: any;

// wasm-kifu が登録するグローバル関数
declare function wasmNewGame(): void;
declare function wasmNewGameWithInfo(playerBlack: string, playerWhite: string, komi: number): void;
declare function wasmPlayMove(x: number, y: number): number;
declare function wasmPass(): void;
declare function wasmResign(): void;
declare function wasmUndo(): number;
declare function wasmExportSGF(): string;
declare function wasmImportSGF(sgfString: string): number;
declare function wasmGetCurrentPlayer(): number;
declare function wasmGetMoveNumber(): number;
declare function wasmGetBoardState(): string;
declare function wasmAnalyzeBoardImage(imageData: string, width: number, height: number): string;
declare function wasmAnalyzeBoardFeatures(featuresJSON: string): string;
declare function wasmAnalyzeFullBoard(featuresJSON: string): string;
declare function wasmApplyDetectedMove(x: number, y: number, color: number): number;
declare function wasmSetBoardFromGrid(grid: number[] | number[][] | string): number;
declare function wasmGetLastMoveInfo(): string;
declare function wasmDetectBoardCorners(
  rgbaData: Uint8Array,
  width: number,
  height: number,
): string;

declare module "@wasm-kifu/web/js/wasm-bridge.js" {
  export interface BoardState {
    stones: number[][]; // 19x19
    player: number; // 1=Black, 2=White
    moveNumber: number;
    ko?: string;
    capturedByBlack: number;
    capturedByWhite: number;
    resigned: boolean;
  }

  export interface LastMoveInfo {
    x: number;
    y: number;
    color: string;
    moveNumber: number;
    isPass: boolean;
  }

  export interface DetectedMove {
    x: number;
    y: number;
    color: string;
    score?: number;
  }

  export interface AnalysisResult {
    moves?: DetectedMove[];
    removed?: DetectedMove[];
    errors?: string[];
    confidence: number;
  }

  export interface FullBoardResult {
    grid: number[]; // 361 elements: 0=Empty, 1=Black, 2=White (row-major: y*19+x)
    confidence: number;
    blackCount: number;
    whiteCount: number;
    emptyCount: number;
  }

  export interface IntersectionFeature {
    x: number;
    y: number;
    l: number;
    c: number;
    d: number;
    h: number;
    s: number;
    rmb?: number;
  }

  export class WasmKifu {
    constructor();
    load(wasmPath: string): Promise<void>;
    isReady(): boolean;
    newGame(): void;
    newGameWithInfo(playerBlack: string, playerWhite: string, komi: number): void;
    playMove(x: number, y: number): number;
    pass(): void;
    resign(): void;
    undo(): boolean;
    exportSGF(): string;
    importSGF(sgfString: string): boolean;
    getCurrentPlayer(): number;
    getMoveNumber(): number;
    getBoardState(): BoardState;
    analyzeBoardImage(intersections: Int8Array): AnalysisResult;
    analyzeBoardFeatures(features: IntersectionFeature[]): AnalysisResult;
    analyzeFullBoard(features: IntersectionFeature[]): FullBoardResult;
    applyDetectedMove(x: number, y: number, color: number): number;
    setBoardFromGrid(grid: number[] | number[][]): boolean;
    getLastMoveInfo(): LastMoveInfo;
    detectBoardCorners(
      rgbaData: Uint8ClampedArray | Uint8Array,
      width: number,
      height: number,
    ): {
      corners: Array<{ x: number; y: number }>;
      area: number;
      confidence: number;
    };
  }
}

declare module "@wasm-kifu/web/js/board-detector.js" {
  export interface Point2D {
    x: number;
    y: number;
  }

  export interface DetectorThresholds {
    blackMaxBrightness: number;
    whiteMinBrightness: number;
    whiteMaxSaturation: number;
  }

  export interface StreamFrameResult {
    motionLevel: number;
    stableProgress: number;
    isStable: boolean;
    isTriggered: boolean;
    features: import("@wasm-kifu/web/js/wasm-bridge.js").IntersectionFeature[] | null;
  }

  export class BoardDetector {
    constructor();
    waitForOpenCV(): Promise<void>;
    isReady(): boolean;
    setCorners(corners: Point2D[]): void;
    hasCornersSet(): boolean;
    resetCorners(): void;
    setThresholds(thresholds: Partial<DetectorThresholds>): void;
    autoDetectCorners(
      source: HTMLCanvasElement | HTMLVideoElement | HTMLImageElement,
    ): Point2D[] | null;
    detect(source: HTMLCanvasElement | HTMLVideoElement | HTMLImageElement): Int8Array;
    extractFeatures(
      source: HTMLCanvasElement | HTMLVideoElement | HTMLImageElement,
    ): import("@wasm-kifu/web/js/wasm-bridge.js").IntersectionFeature[];
    recordedSpacingsX: number[] | null;
    recordedSpacingsY: number[] | null;
    calibrateRecordedSpacings(source: HTMLCanvasElement | HTMLVideoElement | HTMLImageElement): {
      spacingsX: number[];
      spacingsY: number[];
    };
    initStreamTracker(): void;
    processStreamFrame(source: HTMLCanvasElement | HTMLVideoElement): StreamFrameResult;
    resetStreamTracker(): void;
    startCamera(
      videoElement: HTMLVideoElement,
      facingMode?: "user" | "environment",
    ): Promise<MediaStream>;
    stopCamera(stream: MediaStream | null): void;
    captureFrame(videoElement: HTMLVideoElement, canvasElement: HTMLCanvasElement): void;
  }
}

declare module "@sabaki/influence" {
  export interface InfluenceOptions {
    discrete?: boolean;
    maxDistance?: number;
    minRadiance?: number;
  }
  export function map(data: number[][], options?: InfluenceOptions): number[][];
  export function areaMap(data: number[][]): number[][];
  export function distanceMap(data: number[][], sign: number): number[][];
  export function radianceMap(
    data: number[][],
    sign: number,
    p1?: number,
    p2?: number,
    p3?: number,
  ): number[][];
}

declare module "@sabaki/deadstones" {
  export interface DeadstonesOptions {
    playthroughs?: number;
    randomness?: number;
  }
  export function guess(data: number[][], options?: DeadstonesOptions): Promise<[number, number][]>;
}

declare module "@sabaki/immutable-gametree" {
  export interface NodeObject {
    id: any;
    data: Record<string, string[]>;
    parentId: any | null;
    children: NodeObject[];
  }

  export interface GameTreeOptions {
    getId?: () => any;
    merger?: (node: NodeObject, data: Record<string, string[]>) => Record<string, string[]> | null;
    root?: NodeObject;
  }

  export interface Draft {
    root: NodeObject;
    appendNode(parentId: any, data: Record<string, string[]>, options?: any): any;
    removeNode(id: any): any;
    addToProperty(id: any, property: string, value: string): any;
    removeFromProperty(id: any, property: string, value: string): any;
    get(id: any): NodeObject | null;
  }

  export default class GameTree {
    constructor(options?: GameTreeOptions);
    root: NodeObject;
    get(id: any): NodeObject | null;
    mutate(mutator: (draft: Draft) => void): GameTree;
  }
}
