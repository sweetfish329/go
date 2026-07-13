export interface AIAnalysisMove {
  winrate: number; // 黒の勝率 (0 - 100)
  scoreLead: number; // 黒から見た目数差
  loss: number; // この手の損失目数
  isBest: boolean; // この手が最善手かどうか
  bestMove?: string; // 最善手座標 (例: "R16")
  bestMoveCoords?: { x: number; y: number };
  variation?: { x: number; y: number; color: number }[]; // 予想手順
  rawComment: string; // 生のコメント
  rank?: number; // 第一感での順位
}

// アルファベット列をインデックスに変換
function alphaToX(colStr: string): number {
  const col = colStr.toUpperCase();
  let code = col.charCodeAt(0);
  if (code >= 73) {
    // 'I' is 73
    code--; // Iを除外
  }
  return code - 65; // Aは0
}

// インデックスをアルファベット列に変換
function xToAlpha(x: number): string {
  let code = x + 65;
  if (code >= 73) {
    // 'I' is 73
    code++; // Iを除外
  }
  return String.fromCharCode(code);
}

// 囲碁座標 ("R16"など) を { x, y } に変換
export function parseBoardCoords(coordStr: string): { x: number; y: number } | null {
  if (!coordStr) return null;
  const match = coordStr.match(/^([A-T])(\d+)$/i);
  if (!match) return null;
  const x = alphaToX(match[1]);
  const y = 19 - parseInt(match[2], 10);
  if (x >= 0 && x < 19 && y >= 0 && y < 19) {
    return { x, y };
  }
  return null;
}

// { x, y } を囲碁座標 ("R16"など) に変換
export function coordsToBoard(x: number, y: number): string {
  const col = xToAlpha(x);
  const row = 19 - y;
  return `${col}${row}`;
}

// 予想手順の文字列をパースする
// 例: "BR16 D4 P16 D16 Q4 R3 Q3 R4 R6 S6 R7 R5 Q5 S7 R8 S8"
export function parseVariation(variationStr: string): { x: number; y: number; color: number }[] {
  if (!variationStr) return [];
  const tokens = variationStr.trim().split(/\s+/);
  if (tokens.length === 0 || !tokens[0]) return [];

  const result: { x: number; y: number; color: number }[] = [];
  let firstToken = tokens[0];
  const firstColorChar = firstToken.charAt(0).toUpperCase();
  let currentColor = 1; // デフォルト黒

  if (firstColorChar === "B" || firstColorChar === "W") {
    currentColor = firstColorChar === "B" ? 1 : 2;
    firstToken = firstToken.slice(1);
  }

  const firstCoords = parseBoardCoords(firstToken);
  if (firstCoords) {
    result.push({ ...firstCoords, color: currentColor });
  }

  for (let i = 1; i < tokens.length; i++) {
    const coords = parseBoardCoords(tokens[i]);
    if (coords) {
      currentColor = currentColor === 1 ? 2 : 1; // 交互
      result.push({ ...coords, color: currentColor });
    }
  }

  return result;
}

// コメントテキストをパースして AIAnalysisMove オブジェクトを返す
export function parseAIAnalysis(comment: string): AIAnalysisMove | null {
  if (!comment) return null;

  // 「勝率：黒 90.0%」または「勝率：白 50.6%」
  const winrateMatch = comment.match(/勝率：(黒|白)\s*(\d+(?:\.\d+)?)%/);
  if (!winrateMatch) return null;

  const winner = winrateMatch[1];
  const winrateVal = parseFloat(winrateMatch[2]);
  const winrate = winner === "黒" ? winrateVal : 100 - winrateVal;

  // 「目差：黒+5.3」または「目差：白+0.0」
  const scoreMatch = comment.match(/目差：(黒|白)\+?(-?\d+(?:\.\d+)?)/);
  let scoreLead = 0;
  if (scoreMatch) {
    const side = scoreMatch[1];
    const scoreVal = parseFloat(scoreMatch[2]);
    scoreLead = side === "黒" ? scoreVal : -scoreVal;
  }

  // 「推定損失目数：1.3」
  const lossMatch = comment.match(/推定損失目数：(\d+(?:\.\d+)?)/);
  const loss = lossMatch ? parseFloat(lossMatch[1]) : 0;

  // 「最善手はR16 (黒+5.3)」
  const bestMatch = comment.match(/最善手は([A-T]\d+)/i);
  const bestMove = bestMatch ? bestMatch[1] : undefined;
  const bestMoveCoords = bestMove ? parseBoardCoords(bestMove) || undefined : undefined;

  // 「予想手順：...」
  const varMatch = comment.match(/予想手順：([A-Z0-9\s]+)/i);
  const variation = varMatch ? parseVariation(varMatch[1]) : undefined;

  // 「第一感では17位  (3.43%)」
  const rankMatch = comment.match(/第一感では(\d+)位/);
  const rank = rankMatch ? parseInt(rankMatch[1], 10) : undefined;

  const isBest = loss === 0;

  return {
    winrate,
    scoreLead,
    loss,
    isBest,
    bestMove,
    bestMoveCoords,
    variation,
    rawComment: comment,
    rank,
  };
}
