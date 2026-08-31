import { describe, it, expect } from "bun:test";
import { parseSgf, stringifySgf, SgfPlayer } from "./sgfPlayer";

describe("sgfPlayer", () => {
  const sampleSgf = `(;GM[1]FF[4]SZ[19]KM[6.5]PB[Honinbo Shusaku]PW[Honinbo Shuwa];B[qd];W[dc];B[pq];W[cp])`;

  describe("parseSgf & stringifySgf", () => {
    it("parses SGF text into SgfNode tree", () => {
      const root = parseSgf(sampleSgf);
      expect(root).not.toBeNull();
      expect(root?.properties.PB?.[0]).toBe("Honinbo Shusaku");
      expect(root?.properties.PW?.[0]).toBe("Honinbo Shuwa");
      expect(root?.children).toHaveLength(1);
      expect(root?.children[0].properties.B?.[0]).toBe("qd");
    });

    it("handles empty or invalid SGF gracefully", () => {
      expect(parseSgf("")).toBeNull();
      expect(parseSgf("   ")).toBeNull();
    });

    it("stringifies SgfNode tree back into valid SGF format", () => {
      const root = parseSgf(sampleSgf);
      expect(root).not.toBeNull();
      const stringified = stringifySgf(root!);
      expect(stringified).toContain("SZ[19]");
      expect(stringified).toContain("PB[Honinbo Shusaku]");
      expect(stringified).toContain("B[qd]");
    });
  });

  describe("SgfPlayer navigation and manipulation", () => {
    it("initializes game history and moves forward / backward", () => {
      const player = new SgfPlayer(sampleSgf);
      expect(player.history).toHaveLength(5); // Root + 4 moves
      expect(player.currentIndex).toBe(0); // Starts at root (move 0)

      // Step forward
      expect(player.stepForward()).toBe(true);
      expect(player.currentIndex).toBe(1);
      expect(player.getCurrentState().lastMove).toEqual({ x: 16, y: 3 }); // "qd" -> x: 16, y: 3

      // Step backward
      expect(player.stepBackward()).toBe(true);
      expect(player.currentIndex).toBe(0);
      expect(player.getCurrentState().lastMove).toBeNull();

      // Go to specific index
      expect(player.goTo(4)).toBe(true);
      expect(player.currentIndex).toBe(4);
      expect(player.getCurrentState().lastMove).toEqual({ x: 2, y: 15 }); // "cp" -> x: 2, y: 15
    });

    it("adds moves and comments to tree", () => {
      const player = new SgfPlayer(sampleSgf);
      player.goTo(4); // Move 4

      // Add a move (Black at Tengen 9, 9)
      const res = player.addMove(9, 9, 1);
      expect(res.success).toBe(true);
      expect(res.isNew).toBe(true);
      expect(player.getCurrentState().board[9][9]).toBe(1);

      // Add comment
      const commentOk = player.addComment("Sensei", "Center control is important");
      expect(commentOk).toBe(true);
      const comment = player.getComment();
      expect(comment?.author).toBe("Sensei");
      expect(comment?.text).toBe("Center control is important");
    });
  });
});
