export interface BoardState {
  stones: number[][];
  player: number;
  moveNumber: number;
}

export interface AnalysisResult {
  moves: Array<{ x: number; y: number; color: string }>;
  removed: any[];
  errors: any[];
  confidence: number;
}

export interface FullBoardResult {
  grid: number[];
  confidence: number;
  blackCount: number;
  whiteCount: number;
  emptyCount: number;
  warpedRgbaBase64?: string;
  warpedWidth?: number;
  warpedHeight?: number;
  margin?: number;
}

export interface CornerDetectionResult {
  corners: Array<{ x: number; y: number }>;
  area: number;
  confidence: number;
  method?: string;
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
  analyzeBoardFeatures(features: any[]): AnalysisResult;
  analyzeFullBoard(features: any[]): FullBoardResult;
  applyDetectedMove(x: number, y: number, color: number): number;
  setBoardFromGrid(grid: number[] | number[][]): boolean;
  detectBoardCorners(rgbaData: Uint8Array | Uint8ClampedArray, width: number, height: number): CornerDetectionResult;
  detectAndWarp(rgbaData: Uint8Array | Uint8ClampedArray, width: number, height: number): any;
  detectWarpAndAnalyze(rgbaData: Uint8Array | Uint8ClampedArray, width: number, height: number): FullBoardResult;
  warpAndAnalyzeWithCorners(
    rgbaData: Uint8Array | Uint8ClampedArray,
    width: number,
    height: number,
    corners: Array<{ x: number; y: number }>,
    refineWithStars?: boolean
  ): FullBoardResult;
  getLastMoveInfo(): any;
}
