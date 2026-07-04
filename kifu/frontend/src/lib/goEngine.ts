// Simple Go rules engine for board state calculation

export function createBoard(size: number = 19): number[][] {
  const board: number[][] = [];
  for (let i = 0; i < size; i++) {
    board.push(new Array(size).fill(0)); // 0: empty, 1: black, 2: white
  }
  return board;
}

export function copyBoard(board: number[][]): number[][] {
  return board.map(row => [...row]);
}

// Find all connected stones of the same color
function getGroup(board: number[][], startX: number, startY: number, size: number): [number, number][] | null {
  const color = board[startY][startX];
  if (color === 0) return null;

  const group: [number, number][] = [];
  const visited = new Array(size).fill(0).map(() => new Array(size).fill(false));
  const queue: [number, number][] = [[startX, startY]];
  visited[startY][startX] = true;

  while (queue.length > 0) {
    const shiftResult = queue.shift();
    if (!shiftResult) break;
    const [cx, cy] = shiftResult;
    group.push([cx, cy]);

    // Check 4 directions
    const dirs = [[0, 1], [0, -1], [1, 0], [-1, 0]];
    for (const [dx, dy] of dirs) {
      const nx = cx + dx;
      const ny = cy + dy;

      if (nx >= 0 && nx < size && ny >= 0 && ny < size) {
        if (!visited[ny][nx] && board[ny][nx] === color) {
          visited[ny][nx] = true;
          queue.push([nx, ny]);
        }
      }
    }
  }

  return group;
}

// Count liberties of a group of stones
function getLiberties(board: number[][], group: [number, number][], size: number): number {
  const liberties = new Set<string>();
  const dirs = [[0, 1], [0, -1], [1, 0], [-1, 0]];

  for (const [gx, gy] of group) {
    for (const [dx, dy] of dirs) {
      const nx = gx + dx;
      const ny = gy + dy;

      if (nx >= 0 && nx < size && ny >= 0 && ny < size) {
        if (board[ny][nx] === 0) {
          liberties.add(`${nx},${ny}`);
        }
      }
    }
  }

  return liberties.size;
}

export interface PlaceStoneResult {
  success: boolean;
  board?: number[][];
  captured?: number;
  error?: string;
}

// Places a stone on the board. 
// Returns { success: true, board: nextBoard, captured: number } or { success: false, error: string }
export function placeStone(board: number[][], x: number, y: number, color: number, size: number = 19): PlaceStoneResult {
  if (x < 0 || x >= size || y < 0 || y >= size) {
    return { success: false, error: "Out of bounds" };
  }
  if (board[y][x] !== 0) {
    return { success: false, error: "Intersection already occupied" };
  }

  // Work on a copy of the board
  const nextBoard = copyBoard(board);
  nextBoard[y][x] = color;

  const opponentColor = color === 1 ? 2 : 1;
  const capturedStones: [number, number][] = [];

  // Check 4 adjacent points to see if we captured any opponent groups
  const dirs = [[0, 1], [0, -1], [1, 0], [-1, 0]];
  const checkedOpponents = new Array(size).fill(0).map(() => new Array(size).fill(false));

  for (const [dx, dy] of dirs) {
    const nx = x + dx;
    const ny = y + dy;

    if (nx >= 0 && nx < size && ny >= 0 && ny < size) {
      if (nextBoard[ny][nx] === opponentColor && !checkedOpponents[ny][nx]) {
        const group = getGroup(nextBoard, nx, ny, size);
        if (group) {
          // Mark all in group as checked
          for (const [gx, gy] of group) {
            checkedOpponents[gy][gx] = true;
          }

          // If this group has 0 liberties, capture it
          if (getLiberties(nextBoard, group, size) === 0) {
            capturedStones.push(...group);
          }
        }
      }
    }
  }

  // Remove captured stones
  for (const [cx, cy] of capturedStones) {
    nextBoard[cy][cx] = 0;
  }

  // Now check if our own move has any liberties
  const ownGroup = getGroup(nextBoard, x, y, size);
  if (!ownGroup) {
    return { success: false, error: "Invalid move state" };
  }
  const ownLiberties = getLiberties(nextBoard, ownGroup, size);

  if (ownLiberties === 0) {
    // Suicide move, illegal
    return { success: false, error: "Suicide move is illegal" };
  }

  return {
    success: true,
    board: nextBoard,
    captured: capturedStones.length
  };
}

export interface CoordsResult {
  x?: number;
  y?: number;
  pass: boolean;
}

// Convert SGF coordinate (e.g. "pd") to index [x, y]
// SGF coords: "a" = 0, "b" = 1, ..., "s" = 18. Pass "tt" or empty for pass
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
