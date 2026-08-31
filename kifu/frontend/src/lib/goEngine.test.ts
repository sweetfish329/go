import { describe, it, expect } from "bun:test";
import {
  createBoard,
  copyBoard,
  placeStone,
  sgfToCoords,
  coordsToSgf,
  getInfluenceMap,
  getAreaMap,
} from "./goEngine";

describe("goEngine", () => {
  describe("createBoard & copyBoard", () => {
    it("creates a 19x19 empty board by default", () => {
      const board = createBoard(19);
      expect(board).toHaveLength(19);
      expect(board[0]).toHaveLength(19);
      expect(board[0][0]).toBe(0);
    });

    it("creates independent copies with copyBoard", () => {
      const board = createBoard(19);
      board[3][3] = 1;
      const copy = copyBoard(board);
      copy[3][3] = 2;
      expect(board[3][3]).toBe(1);
      expect(copy[3][3]).toBe(2);
    });
  });

  describe("sgfToCoords & coordsToSgf", () => {
    it("converts SGF coordinates to 0-indexed [x, y]", () => {
      expect(sgfToCoords("aa")).toEqual({ x: 0, y: 0, pass: false });
      expect(sgfToCoords("pd")).toEqual({ x: 15, y: 3, pass: false });
      expect(sgfToCoords("ss")).toEqual({ x: 18, y: 18, pass: false });
    });

    it("handles pass coordinates", () => {
      expect(sgfToCoords("")).toEqual({ pass: true });
      expect(sgfToCoords("tt")).toEqual({ pass: true });
    });

    it("converts index [x, y] to SGF coordinate", () => {
      expect(coordsToSgf(0, 0)).toBe("aa");
      expect(coordsToSgf(15, 3)).toBe("pd");
      expect(coordsToSgf(18, 18)).toBe("ss");
    });
  });

  describe("placeStone", () => {
    it("places stone on empty intersection", () => {
      const board = createBoard(19);
      const res = placeStone(board, 3, 3, 1, 19);
      expect(res.success).toBe(true);
      expect(res.board?.[3][3]).toBe(1);
    });

    it("rejects placement on occupied intersection", () => {
      const board = createBoard(19);
      board[3][3] = 1;
      const res = placeStone(board, 3, 3, 2, 19);
      expect(res.success).toBe(false);
      expect(res.error).toBe("Intersection already occupied");
    });

    it("rejects out of bounds placement", () => {
      const board = createBoard(19);
      expect(placeStone(board, -1, 0, 1, 19).success).toBe(false);
      expect(placeStone(board, 19, 0, 1, 19).success).toBe(false);
    });

    it("captures surrounded stones", () => {
      let board = createBoard(19);
      // Place White at (0,0)
      const w1 = placeStone(board, 0, 0, 2, 19);
      expect(w1.success).toBe(true);
      board = w1.board!;

      // Black at (1,0)
      const b1 = placeStone(board, 1, 0, 1, 19);
      expect(b1.success).toBe(true);
      board = b1.board!;

      // Black at (0,1) captures (0,0)
      const b2 = placeStone(board, 0, 1, 1, 19);
      expect(b2.success).toBe(true);
      expect(b2.captured).toBe(1);
      expect(b2.board?.[0][0]).toBe(0); // Captured white stone removed
      expect(b2.board?.[0][1]).toBe(1); // Black stone present
      expect(b2.board?.[1][0]).toBe(1); // Black stone present
    });

    it("rejects suicide moves", () => {
      let board = createBoard(19);
      // Surround (0,0) with black
      board[0][1] = 1;
      board[1][0] = 1;

      // White plays at (0,0) with 0 liberties
      const res = placeStone(board, 0, 0, 2, 19);
      expect(res.success).toBe(false);
      expect(res.error).toContain("Suicide");
    });
  });

  describe("influence and area maps", () => {
    it("computes influence and area maps", () => {
      const board = createBoard(19);
      board[3][3] = 1; // Black hoshi
      board[15][15] = 2; // White hoshi

      const inf = getInfluenceMap(board, 19);
      expect(inf).toHaveLength(19);
      expect(inf[3][3]).toBeGreaterThan(0); // Positive for Black
      expect(inf[15][15]).toBeLessThan(0); // Negative for White

      const area = getAreaMap(board, 19);
      expect(area).toHaveLength(19);
    });
  });
});
