import { createBoard, placeStone, sgfToCoords, coordsToSgf } from './goEngine';

export interface SgfNode {
  properties: Record<string, string[]>;
  children: SgfNode[];
  parent: SgfNode | null;
}

export interface HistoryEntry {
  board: number[][];
  lastMove: { x: number; y: number } | null;
  captives: { B: number; W: number };
  node: SgfNode | null;
}

import { parse as sabakiParse, stringify as sabakiStringify } from '@sabaki/sgf';

// Convert Sabaki's SGF Node to our internal SgfNode structure (with parent pointers)
function convertSabakiNode(sabakiNode: any, parent: SgfNode | null = null): SgfNode {
  const node: SgfNode = {
    properties: sabakiNode.data || {},
    children: [],
    parent: parent
  };

  if (sabakiNode.children && sabakiNode.children.length > 0) {
    node.children = sabakiNode.children.map((child: any) => convertSabakiNode(child, node));
  }

  return node;
}

// Convert our SgfNode back to Sabaki Sgf node structure for serialization
function convertToSabakiNode(node: SgfNode, idCounter = { val: 1 }): any {
  const currentId = idCounter.val++;
  const sabakiNode: any = {
    id: currentId,
    data: node.properties,
    parentId: null,
    children: []
  };

  sabakiNode.children = node.children.map(child => {
    const childNode = convertToSabakiNode(child, idCounter);
    childNode.parentId = currentId;
    return childNode;
  });

  return sabakiNode;
}

// Parse SGF string into a JS tree structure using @sabaki/sgf
export function parseSgf(sgfStr: string): SgfNode | null {
  sgfStr = sgfStr.trim();
  if (!sgfStr) return null;

  try {
    const rootNodes = sabakiParse(sgfStr);
    if (!rootNodes || rootNodes.length === 0) return null;
    return convertSabakiNode(rootNodes[0]);
  } catch (err) {
    console.error("Failed to parse SGF with @sabaki/sgf:", err);
    return null;
  }
}

// Generate an SGF string from a tree using @sabaki/sgf
export function stringifySgf(rootNode: SgfNode): string {
  try {
    const sabakiRoot = convertToSabakiNode(rootNode);
    return sabakiStringify([sabakiRoot]);
  } catch (err) {
    console.error("Failed to stringify SGF with @sabaki/sgf:", err);
    // Simple manual fallback
    let sgf = "";
    function traverse(node: SgfNode): void {
      sgf += ";";
      for (const [key, values] of Object.entries(node.properties)) {
        sgf += key;
        for (const val of values) {
          const escaped = val.replace(/\\/g, '\\\\').replace(/\]/g, '\\]');
          sgf += `[${escaped}]`;
        }
      }
      if (node.children.length === 0) return;
      if (node.children.length === 1) {
        traverse(node.children[0]);
      } else {
        for (const child of node.children) {
          sgf += "(";
          traverse(child);
          sgf += ")";
        }
      }
    }
    sgf += "(";
    traverse(rootNode);
    sgf += ")";
    return sgf;
  }
}

// SgfPlayer class manages navigation and board state transitions
export class SgfPlayer {
  boardSize: number;
  root: SgfNode | null;
  history: HistoryEntry[];
  currentIndex: number;

  constructor(sgfText: string, boardSize: number = 19) {
    this.boardSize = boardSize;
    this.root = parseSgf(sgfText);
    this.history = []; // Array of { board, lastMove, captives: { B, W }, node }
    this.currentIndex = 0;

    this.initGame();
  }

  initGame(): void {
    const initialBoard = createBoard(this.boardSize);
    
    // Process root properties (like handicap stones)
    let board = initialBoard;
    const captives = { B: 0, W: 0 };

    if (this.root) {
      // Add handicap/add-black stones (AB) or add-white (AW)
      if (this.root.properties.AB) {
        for (const coord of this.root.properties.AB) {
          const { x, y, pass } = sgfToCoords(coord);
          if (!pass && x !== undefined && y !== undefined) board[y][x] = 1;
        }
      }
      if (this.root.properties.AW) {
        for (const coord of this.root.properties.AW) {
          const { x, y, pass } = sgfToCoords(coord);
          if (!pass && x !== undefined && y !== undefined) board[y][x] = 2;
        }
      }
    }

    // Push base state (index 0)
    this.history = [{
      board: board,
      lastMove: null,
      captives: captives,
      node: this.root
    }];
    this.currentIndex = 0;

    // Pre-calculate history for the main path (or active path)
    this.calculateMainPath();
  }

  // Pre-calculate board states for consecutive moves
  calculateMainPath(): void {
    let current = this.history[this.currentIndex];
    let node = current.node;

    // Clear history ahead of current index
    this.history = this.history.slice(0, this.currentIndex + 1);

    while (node && node.children.length > 0) {
      // Default to first child
      const nextNode = node.children[0];
      const nextState = this.calculateNextState(current, nextNode);
      if (!nextState) break; // Invalid move, stop pre-calculation

      this.history.push(nextState);
      current = nextState;
      node = nextNode;
    }
  }

  calculateNextState(currentState: HistoryEntry, nextNode: SgfNode): HistoryEntry | null {
    let color = 0;
    let coordStr = "";

    if (nextNode.properties.B) {
      color = 1;
      coordStr = nextNode.properties.B[0];
    } else if (nextNode.properties.W) {
      color = 2;
      coordStr = nextNode.properties.W[0];
    }

    if (color === 0) {
      // Non-move node (e.g. annotations only), copy previous state
      return {
        board: currentState.board,
        lastMove: currentState.lastMove,
        captives: { ...currentState.captives },
        node: nextNode
      };
    }

    const { x, y, pass } = sgfToCoords(coordStr);
    if (pass) {
      return {
        board: currentState.board,
        lastMove: null, // Pass
        captives: { ...currentState.captives },
        node: nextNode
      };
    }

    if (x === undefined || y === undefined) {
      return null;
    }

    const res = placeStone(currentState.board, x, y, color, this.boardSize);
    if (!res.success || !res.board || res.captured === undefined) {
      // Log error but allow navigation
      console.warn("Invalid SGF move:", res.error, `at [${x}, ${y}]`);
      return {
        board: currentState.board,
        lastMove: { x, y },
        captives: { ...currentState.captives },
        node: nextNode
      };
    }

    const newCaptives = { ...currentState.captives };
    if (color === 1) {
      newCaptives.W += res.captured; // Black captured White stones
    } else {
      newCaptives.B += res.captured; // White captured Black stones
    }

    return {
      board: res.board,
      lastMove: { x, y },
      captives: newCaptives,
      node: nextNode
    };
  }

  // Navigation methods
  getCurrentState(): HistoryEntry {
    return this.history[this.currentIndex];
  }

  stepForward(): boolean {
    if (this.currentIndex < this.history.length - 1) {
      this.currentIndex++;
      return true;
    }
    return false;
  }

  stepBackward(): boolean {
    if (this.currentIndex > 0) {
      this.currentIndex--;
      return true;
    }
    return false;
  }

  goTo(index: number): boolean {
    if (index >= 0 && index < this.history.length) {
      this.currentIndex = index;
      return true;
    }
    return false;
  }

  // Get other branches at current node
  getAlternativeBranches(): SgfNode[] {
    const currentState = this.getCurrentState();
    const node = currentState.node;
    if (!node || !node.parent) return [];
    
    // Siblings of the current node
    return node.parent.children.filter(child => child !== node);
  }

  // Switch to another branch at the current move
  selectBranch(branchIndex: number): boolean {
    const currentState = this.getCurrentState();
    if (!currentState.node) return false;
    const parentNode = currentState.node.parent;
    if (!parentNode || branchIndex < 0 || branchIndex >= parentNode.children.length) return false;

    // Step back
    this.stepBackward();
    const prevAppState = this.getCurrentState();

    // Select the branch node
    const targetNode = parentNode.children[branchIndex];
    const nextState = this.calculateNextState(prevAppState, targetNode);
    if (!nextState) return false;

    // Replace the rest of history from current index + 1
    this.history = this.history.slice(0, this.currentIndex + 1);
    this.history.push(nextState);
    this.currentIndex++;
    this.calculateMainPath();

    return true;
  }

  // Add a new branch/variation at the current position
  addMove(x: number, y: number, color: number): { success: boolean; isNew?: boolean; node?: SgfNode; error?: string } {
    const currentState = this.getCurrentState();
    const currentNode = currentState.node;

    // Check if this move already exists in children
    const coordStr = coordsToSgf(x, y);
    const key = color === 1 ? "B" : "W";
    
    let existingChild: SgfNode | undefined = undefined;
    if (currentNode) {
      existingChild = currentNode.children.find(child => {
        return child.properties[key] && child.properties[key][0] === coordStr;
      });
    }

    if (existingChild) {
      // Just navigate to existing child
      const idx = this.history.findIndex(h => h.node === existingChild);
      if (idx !== -1) {
        this.currentIndex = idx;
      } else {
        // If not pre-calculated, push it
        const nextState = this.calculateNextState(currentState, existingChild);
        if (nextState) {
          this.history = this.history.slice(0, this.currentIndex + 1);
          this.history.push(nextState);
          this.currentIndex++;
          this.calculateMainPath();
        }
      }
      return { success: true, isNew: false };
    }

    // Apply go rules validation
    const res = placeStone(currentState.board, x, y, color, this.boardSize);
    if (!res.success || !res.board || res.captured === undefined) {
      return { success: false, error: res.error };
    }

    // Create new SGF node
    const newNode: SgfNode = {
      properties: {
        [key]: [coordStr]
      },
      children: [],
      parent: currentNode
    };

    if (currentNode) {
      currentNode.children.push(newNode);
    } else {
      this.root = newNode;
    }

    // Precalculate new state
    const nextCaptives = { ...currentState.captives };
    if (color === 1) {
      nextCaptives.W += res.captured;
    } else {
      nextCaptives.B += res.captured;
    }

    const nextState: HistoryEntry = {
      board: res.board,
      lastMove: { x, y },
      captives: nextCaptives,
      node: newNode
    };

    // Update history
    this.history = this.history.slice(0, this.currentIndex + 1);
    this.history.push(nextState);
    this.currentIndex++;

    return { success: true, isNew: true, node: newNode };
  }

  // Add comment to current node
  addComment(reviewerName: string, commentText: string): boolean {
    const currentState = this.getCurrentState();
    const node = currentState.node;
    if (!node) return false;

    // Standard SGF property for comment is C[...]
    node.properties["C"] = [`${reviewerName}: ${commentText}`];
    return true;
  }

  // Get comments from current node
  getComment(): { author: string; text: string } | null {
    const currentState = this.getCurrentState();
    const node = currentState.node;
    if (!node || !node.properties["C"]) return null;
    
    const rawComment = node.properties["C"][0];
    const colonIndex = rawComment.indexOf(":");
    if (colonIndex !== -1) {
      return {
        author: rawComment.substring(0, colonIndex).trim(),
        text: rawComment.substring(colonIndex + 1).trim()
      };
    }
    return {
      author: "Unknown",
      text: rawComment
    };
  }

  getSgfString(): string {
    if (!this.root) return "";
    return stringifySgf(this.root);
  }
}
