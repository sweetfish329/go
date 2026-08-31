import { describe, it, expect } from "bun:test";
import { parseBoardCoords, coordsToBoard, parseVariation, parseAIAnalysis } from "./aiAnalysis";

describe("aiAnalysis", () => {
  describe("parseBoardCoords", () => {
    it("converts standard Go board coordinates correctly (skipping I)", () => {
      // Top-left: A19 -> x: 0, y: 0
      expect(parseBoardCoords("A19")).toEqual({ x: 0, y: 0 });
      // Bottom-right: T1 -> x: 18, y: 18
      expect(parseBoardCoords("T1")).toEqual({ x: 18, y: 18 });
      // H10 (Tengen): H is 7th letter (A,B,C,D,E,F,G,H -> index 7)
      expect(parseBoardCoords("H10")).toEqual({ x: 7, y: 9 });
      // J10: J comes after I, so code 74 - 1 - 65 = index 8
      expect(parseBoardCoords("J10")).toEqual({ x: 8, y: 9 });
      // R16
      expect(parseBoardCoords("R16")).toEqual({ x: 16, y: 3 });
    });

    it("returns null for invalid coordinates", () => {
      expect(parseBoardCoords("")).toBeNull();
      expect(parseBoardCoords("Z99")).toBeNull();
      expect(parseBoardCoords("A0")).toBeNull();
      expect(parseBoardCoords("A20")).toBeNull();
      expect(parseBoardCoords("invalid")).toBeNull();
    });
  });

  describe("coordsToBoard", () => {
    it("converts {x, y} to Go coordinate string correctly", () => {
      expect(coordsToBoard(0, 0)).toBe("A19");
      expect(coordsToBoard(18, 18)).toBe("T1");
      expect(coordsToBoard(16, 3)).toBe("R16");
      expect(coordsToBoard(7, 9)).toBe("H10");
      expect(coordsToBoard(8, 9)).toBe("J10");
    });
  });

  describe("parseVariation", () => {
    it("parses variation sequence with starting color", () => {
      const variation = "BR16 D4 P16 D16";
      const parsed = parseVariation(variation);
      expect(parsed).toHaveLength(4);
      expect(parsed[0]).toEqual({ x: 16, y: 3, color: 1 }); // Black
      expect(parsed[1]).toEqual({ x: 3, y: 15, color: 2 }); // White
      expect(parsed[2]).toEqual({ x: 14, y: 3, color: 1 }); // Black
      expect(parsed[3]).toEqual({ x: 3, y: 3, color: 2 }); // White
    });

    it("returns empty array for empty string", () => {
      expect(parseVariation("")).toEqual([]);
      expect(parseVariation("   ")).toEqual([]);
    });
  });

  describe("parseAIAnalysis", () => {
    it("extracts winrate, lead, loss, bestMove, variation, and rank", () => {
      const comment = `勝率：黒 75.4%
目差：黒+3.8
推定損失目数：0.0
最善手はR16
第一感では1位 (42.5%)
予想手順：BR16 D4 P16`;

      const result = parseAIAnalysis(comment);
      expect(result).not.toBeNull();
      expect(result?.winrate).toBe(75.4);
      expect(result?.scoreLead).toBe(3.8);
      expect(result?.loss).toBe(0.0);
      expect(result?.isBest).toBe(true);
      expect(result?.bestMove).toBe("R16");
      expect(result?.bestMoveCoords).toEqual({ x: 16, y: 3 });
      expect(result?.rank).toBe(1);
      expect(result?.variation).toHaveLength(3);
    });

    it("converts white winrate to black perspective", () => {
      const comment = `勝率：白 60.0%
目差：白+2.5
推定損失目数：1.5
最善手はD4
第一感では3位 (12.0%)`;

      const result = parseAIAnalysis(comment);
      expect(result).not.toBeNull();
      expect(result?.winrate).toBe(40.0); // 100 - 60
      expect(result?.scoreLead).toBe(-2.5); // Black lead: -2.5
      expect(result?.loss).toBe(1.5);
      expect(result?.isBest).toBe(false);
    });

    it("returns null for non-AI comment", () => {
      expect(parseAIAnalysis("ただの対局コメントです")).toBeNull();
      expect(parseAIAnalysis("")).toBeNull();
    });
  });
});
