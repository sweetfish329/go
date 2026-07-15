import { createBoard, placeStone, sgfToCoords, coordsToSgf } from "./goEngine";
import { parseAIAnalysis } from "./aiAnalysis";
import { parse as sabakiParse, stringify as sabakiStringify } from "@sabaki/sgf";
import GameTree from "@sabaki/immutable-gametree";

export interface SgfNode {
  id?: any; // Added for @sabaki/immutable-gametree support
  properties: Record<string, string[]>;
  children: SgfNode[];
  parent: SgfNode | null;
  aiAnalysis?: any;
}

export interface HistoryEntry {
  board: number[][];
  lastMove: { x: number; y: number } | null;
  captives: { B: number; W: number };
  node: SgfNode | null;
}

// Convert Sabaki's SGF Node to our internal SgfNode structure (with parent pointers)
function convertSabakiNode(
  sabakiNode: any,
  parent: SgfNode | null = null,
  nodeMap?: Map<any, SgfNode>,
): SgfNode {
  const node: SgfNode = {
    id: sabakiNode.id,
    properties: sabakiNode.data || {},
    children: [],
    parent: parent,
  };

  if (nodeMap && sabakiNode.id !== undefined) {
    nodeMap.set(sabakiNode.id, node);
  }

  if (sabakiNode.children && sabakiNode.children.length > 0) {
    node.children = sabakiNode.children.map((child: any) =>
      convertSabakiNode(child, node, nodeMap),
    );
  }

  return node;
}

// Convert our SgfNode back to Sabaki Sgf node structure for serialization
function convertToSabakiNode(node: SgfNode, idCounter = { val: 1 }): any {
  const currentId = node.id !== undefined ? node.id : idCounter.val++;
  const sabakiNode: any = {
    id: currentId,
    data: node.properties,
    parentId: node.parent?.id || null,
    children: [],
  };

  sabakiNode.children = node.children.map((child) => {
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

    // Build a GameTree to ensure valid and consistent IDs
    const tree = new GameTree({ root: rootNodes[0] });
    return convertSabakiNode(tree.root);
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
          const escaped = val.replace(/\\/g, "\\\\").replace(/\]/g, "\\]");
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

// SgfPlayer class manages navigation and board state transitions using @sabaki/immutable-gametree
export class SgfPlayer {
  boardSize: number;
  root: SgfNode | null;
  history: HistoryEntry[];
  currentIndex: number;
  tree: GameTree;
  nodeMap: Map<any, SgfNode>;

  constructor(sgfText: string, boardSize: number = 19) {
    this.boardSize = boardSize;
    this.nodeMap = new Map();

    const rootNodes = sabakiParse(sgfText.trim());
    if (rootNodes && rootNodes.length > 0) {
      this.tree = new GameTree({ root: rootNodes[0] });
      this.root = convertSabakiNode(this.tree.root, null, this.nodeMap);
      this.attachAIAnalysis(this.root);
    } else {
      this.tree = new GameTree();
      this.root = null;
    }

    this.history = [];
    this.currentIndex = 0;
    this.initGame();
  }

  // Update GameTree state and rebuild the SgfNode tree and node references
  updateTree(newTree: GameTree, selectNodeId?: any): void {
    const oldCurrentNode = this.getCurrentState()?.node;
    const oldCurrentId = oldCurrentNode ? oldCurrentNode.id : null;

    this.tree = newTree;
    this.nodeMap.clear();
    this.root = convertSabakiNode(this.tree.root, null, this.nodeMap);
    if (this.root) {
      this.attachAIAnalysis(this.root);
    }

    // Update node references in history
    for (const entry of this.history) {
      if (entry.node && entry.node.id !== undefined) {
        const newNode = this.nodeMap.get(entry.node.id);
        if (newNode) {
          entry.node = newNode;
        }
      }
    }

    this.calculateMainPath();

    // Reset current index to target or previous active node
    const targetId = selectNodeId !== undefined ? selectNodeId : oldCurrentId;
    if (targetId !== null) {
      const idx = this.history.findIndex((h) => h.node && h.node.id === targetId);
      if (idx !== -1) {
        this.currentIndex = idx;
      }
    }
  }

  attachAIAnalysis(node: SgfNode): void {
    if (node.properties.C && node.properties.C.length > 0) {
      const ai = parseAIAnalysis(node.properties.C[0]);
      if (ai) {
        node.aiAnalysis = ai;
      }
    }
    for (const child of node.children) {
      this.attachAIAnalysis(child);
    }
  }

  initGame(): void {
    const initialBoard = createBoard(this.boardSize);
    let board = initialBoard;
    const captives = { B: 0, W: 0 };

    if (this.root) {
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

    this.history = [
      {
        board: board,
        lastMove: null,
        captives: captives,
        node: this.root,
      },
    ];
    this.currentIndex = 0;
    this.calculateMainPath();
  }

  calculateMainPath(): void {
    let current = this.history[this.currentIndex];
    let node = current.node;

    this.history = this.history.slice(0, this.currentIndex + 1);

    while (node && node.children.length > 0) {
      const nextNode = node.children[0];
      const nextState = this.calculateNextState(current, nextNode);
      if (!nextState) break;

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
      return {
        board: currentState.board,
        lastMove: currentState.lastMove,
        captives: { ...currentState.captives },
        node: nextNode,
      };
    }

    const { x, y, pass } = sgfToCoords(coordStr);
    if (pass) {
      return {
        board: currentState.board,
        lastMove: null,
        captives: { ...currentState.captives },
        node: nextNode,
      };
    }

    if (x === undefined || y === undefined) {
      return null;
    }

    const res = placeStone(currentState.board, x, y, color, this.boardSize);
    if (!res.success || !res.board || res.captured === undefined) {
      console.warn("Invalid SGF move:", res.error, `at [${x}, ${y}]`);
      return {
        board: currentState.board,
        lastMove: { x, y },
        captives: { ...currentState.captives },
        node: nextNode,
      };
    }

    const newCaptives = { ...currentState.captives };
    if (color === 1) {
      newCaptives.W += res.captured;
    } else {
      newCaptives.B += res.captured;
    }

    return {
      board: res.board,
      lastMove: { x, y },
      captives: newCaptives,
      node: nextNode,
    };
  }

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

  getAlternativeBranches(): SgfNode[] {
    const currentState = this.getCurrentState();
    const node = currentState.node;
    if (!node || !node.parent) return [];
    return node.parent.children.filter((child) => child !== node);
  }

  selectBranch(branchIndex: number): boolean {
    const currentState = this.getCurrentState();
    if (!currentState.node) return false;
    const parentNode = currentState.node.parent;
    if (!parentNode || branchIndex < 0 || branchIndex >= parentNode.children.length) return false;

    this.stepBackward();
    const prevAppState = this.getCurrentState();

    const targetNode = parentNode.children[branchIndex];
    const nextState = this.calculateNextState(prevAppState, targetNode);
    if (!nextState) return false;

    this.history = this.history.slice(0, this.currentIndex + 1);
    this.history.push(nextState);
    this.currentIndex++;
    this.calculateMainPath();

    return true;
  }

  addMove(
    x: number,
    y: number,
    color: number,
  ): { success: boolean; isNew?: boolean; node?: SgfNode; error?: string } {
    const currentState = this.getCurrentState();
    const currentNode = currentState.node;

    if (!currentNode) {
      return { success: false, error: "No active node to play on" };
    }

    const coordStr = coordsToSgf(x, y);
    const key = color === 1 ? "B" : "W";

    const existingChild = currentNode.children.find((child) => {
      return child.properties[key] && child.properties[key][0] === coordStr;
    });

    if (existingChild) {
      const idx = this.history.findIndex((h) => h.node && h.node.id === existingChild.id);
      if (idx !== -1) {
        this.currentIndex = idx;
      } else {
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

    const res = placeStone(currentState.board, x, y, color, this.boardSize);
    if (!res.success || !res.board || res.captured === undefined) {
      return { success: false, error: res.error };
    }

    let newNodeId: any = null;
    const newTree = this.tree.mutate((draft) => {
      newNodeId = draft.appendNode(currentNode.id, {
        [key]: [coordStr],
      });
    });

    this.updateTree(newTree, newNodeId);

    const updatedCurrentState = this.history[this.currentIndex];
    const nextCaptives = { ...updatedCurrentState.captives };
    if (color === 1) {
      nextCaptives.W += res.captured;
    } else {
      nextCaptives.B += res.captured;
    }

    const newNode = this.nodeMap.get(newNodeId);
    if (!newNode) return { success: false, error: "Failed to find new node" };

    const nextState: HistoryEntry = {
      board: res.board,
      lastMove: { x, y },
      captives: nextCaptives,
      node: newNode,
    };

    this.history = this.history.slice(0, this.currentIndex + 1);
    this.history.push(nextState);
    this.currentIndex++;
    this.calculateMainPath();

    return { success: true, isNew: true, node: newNode };
  }

  addComment(reviewerName: string, commentText: string): boolean {
    const currentState = this.getCurrentState();
    const node = currentState.node;
    if (!node || node.id === undefined) return false;

    const commentVal = `${reviewerName}: ${commentText}`;

    const newTree = this.tree.mutate((draft) => {
      const draftNode = draft.get(node.id);
      if (draftNode) {
        draftNode.data["C"] = [commentVal];
      }
    });

    this.updateTree(newTree);
    return true;
  }

  getComment(): { author: string; text: string } | null {
    const currentState = this.getCurrentState();
    const node = currentState.node;
    if (!node || !node.properties["C"]) return null;

    const rawComment = node.properties["C"][0];
    const colonIndex = rawComment.indexOf(":");
    if (colonIndex !== -1) {
      return {
        author: rawComment.substring(0, colonIndex).trim(),
        text: rawComment.substring(colonIndex + 1).trim(),
      };
    }
    return {
      author: "Unknown",
      text: rawComment,
    };
  }

  getSgfString(): string {
    try {
      return sabakiStringify([this.tree.root]);
    } catch (err) {
      console.error("Failed to stringify GameTree:", err);
      return "";
    }
  }
}
