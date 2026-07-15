import GoBoard from "@sabaki/go-board";
import * as influence from "@sabaki/influence";
import * as deadstones from "@sabaki/deadstones";

export function createBoard(size: number = 19): number[][] {
  const board: number[][] = [];
  for (let i = 0; i < size; i++) {
    board.push(new Array(size).fill(0)); // 0: empty, 1: black, 2: white
  }
  return board;
}

export function copyBoard(board: number[][]): number[][] {
  return board.map((row) => [...row]);
}

// Helper to convert kifu colors (1: black, 2: white, 0: empty) to Sabaki signs (1: black, -1: white, 0: empty)
function kifuToSabakiColor(color: number): 1 | -1 | 0 {
  if (color === 1) return 1;
  if (color === 2) return -1;
  return 0;
}

// Helper to convert Sabaki signs back to kifu colors
function sabakiToKifuColor(color: number): number {
  if (color === 1) return 1;
  if (color === -1) return 2;
  return 0;
}

// Convert kifu 2D array board to Sabaki GoBoard
function toSabakiBoard(kifuBoard: number[][], size: number): GoBoard {
  const sabakiBoard = GoBoard.fromDimensions(size);
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      const kColor = kifuBoard[y][x];
      if (kColor !== 0) {
        sabakiBoard.set([x, y], kifuToSabakiColor(kColor));
      }
    }
  }
  return sabakiBoard;
}

// Convert Sabaki GoBoard back to kifu 2D array board
function toKifuBoard(sabakiBoard: GoBoard, size: number): number[][] {
  const board = createBoard(size);
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      const sColor = sabakiBoard.get([x, y]);
      board[y][x] = sabakiToKifuColor(sColor || 0);
    }
  }
  return board;
}

export interface PlaceStoneResult {
  success: boolean;
  board?: number[][];
  captured?: number;
  error?: string;
}

// Places a stone on the board using @sabaki/go-board.
export function placeStone(
  board: number[][],
  x: number,
  y: number,
  color: number,
  size: number = 19,
): PlaceStoneResult {
  if (x < 0 || x >= size || y < 0 || y >= size) {
    return { success: false, error: "Out of bounds" };
  }
  if (board[y][x] !== 0) {
    return { success: false, error: "Intersection already occupied" };
  }

  const sabakiBoard = toSabakiBoard(board, size);
  const sabakiColor = kifuToSabakiColor(color);

  if (sabakiColor === 0) {
    return { success: false, error: "Invalid stone color" };
  }

  // Check rules (suicide, ko) using analyzeMove
  const analysis = sabakiBoard.analyzeMove(sabakiColor, [x, y]);
  if (analysis.suicide) {
    return { success: false, error: "Suicide move is illegal" };
  }
  if (analysis.ko) {
    return { success: false, error: "Ko rule violation" };
  }

  try {
    const nextSabakiBoard = sabakiBoard.makeMove(sabakiColor, [x, y], {
      preventSuicide: true,
      preventKo: true,
      preventOverwrite: true,
    });

    // Count captured stones
    let captured = 0;
    const changedVertices = sabakiBoard.diff(nextSabakiBoard);
    if (changedVertices) {
      const opponentColor = -sabakiColor;
      for (const vertex of changedVertices) {
        if (sabakiBoard.get(vertex) === opponentColor && nextSabakiBoard.get(vertex) === 0) {
          captured++;
        }
      }
    }

    return {
      success: true,
      board: toKifuBoard(nextSabakiBoard, size),
      captured: captured,
    };
  } catch (err: any) {
    return { success: false, error: err.message || "Invalid move" };
  }
}

export interface CoordsResult {
  x?: number;
  y?: number;
  pass: boolean;
}

// Convert SGF coordinate (e.g. "pd") to index [x, y]
export function sgfToCoords(sgfStr: string): CoordsResult {
  if (!sgfStr || sgfStr === "" || sgfStr === "tt") {
    return { pass: true };
  }
  const x = sgfStr.charCodeAt(0) - 97;
  const y = sgfStr.charCodeAt(1) - 97;
  return { x, y, pass: false };
}

// Convert index [x, y] to SGF coordinate (e.g. "pd")
export function coordsToSgf(x: number, y: number): string {
  const xChar = String.fromCharCode(x + 97);
  const yChar = String.fromCharCode(y + 97);
  return xChar + yChar;
}

// Get influence map (-1: White influence, 1: Black influence)
export function getInfluenceMap(board: number[][], size: number = 19): number[][] {
  const sabakiBoard = toSabakiBoard(board, size);
  return influence.map(sabakiBoard.signMap);
}

// Get area map (-1: White area, 0: Neutral, 1: Black area)
export function getAreaMap(board: number[][], size: number = 19): number[][] {
  const sabakiBoard = toSabakiBoard(board, size);
  return influence.areaMap(sabakiBoard.signMap);
}

// Guess dead stones. Returns array of {x, y} coordinate objects.
export function getDeadStones(board: number[][], size: number = 19): { x: number; y: number }[] {
  const sabakiBoard = toSabakiBoard(board, size);
  const deadList = deadstones.guess(sabakiBoard.signMap);
  return deadList.map(([x, y]) => ({ x, y }));
}
