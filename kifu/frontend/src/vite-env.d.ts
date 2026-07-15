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
declare function wasmApplyDetectedMove(x: number, y: number, color: number): number;
declare function wasmGetLastMoveInfo(): string;

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
  }

  export interface AnalysisResult {
    moves?: DetectedMove[];
    removed?: DetectedMove[];
    errors?: string[];
    confidence: number;
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
    applyDetectedMove(x: number, y: number, color: number): number;
    getLastMoveInfo(): LastMoveInfo;
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

  export class BoardDetector {
    constructor();
    waitForOpenCV(): Promise<void>;
    isReady(): boolean;
    setCorners(corners: Point2D[]): void;
    hasCornersSet(): boolean;
    resetCorners(): void;
    setThresholds(thresholds: Partial<DetectorThresholds>): void;
    detect(source: HTMLCanvasElement | HTMLVideoElement | HTMLImageElement): Int8Array;
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
