package vision

import (
	"encoding/json"
	"testing"

	"github.com/sweetfish329/go/wasm-kifu/pkg/engine"
)

func TestAnalyzeBoardDiff_NewStone(t *testing.T) {
	ge := engine.NewGame()

	// 空盤面に黒石を1つ検出
	var intersections [361]int8
	intersections[9*19+9] = 1 // (9,9) に黒石

	detected := DetectedBoard{Intersections: intersections}
	data, _ := json.Marshal(detected)

	resultJSON := AnalyzeBoardDiff(ge, string(data), 19, 19)

	var result AnalysisResult
	json.Unmarshal([]byte(resultJSON), &result)

	if len(result.Moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(result.Moves))
	}
	if result.Moves[0].X != 9 || result.Moves[0].Y != 9 {
		t.Errorf("expected move at (9,9), got (%d,%d)", result.Moves[0].X, result.Moves[0].Y)
	}
	if result.Moves[0].Color != "B" {
		t.Errorf("expected color B, got %s", result.Moves[0].Color)
	}
	if result.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", result.Confidence)
	}
}

func TestAnalyzeBoardDiff_WhiteStone(t *testing.T) {
	ge := engine.NewGame()

	var intersections [361]int8
	intersections[3*19+3] = 2 // (3,3) に白石

	detected := DetectedBoard{Intersections: intersections}
	data, _ := json.Marshal(detected)

	resultJSON := AnalyzeBoardDiff(ge, string(data), 19, 19)

	var result AnalysisResult
	json.Unmarshal([]byte(resultJSON), &result)

	if len(result.Moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(result.Moves))
	}
	if result.Moves[0].Color != "W" {
		t.Errorf("expected color W, got %s", result.Moves[0].Color)
	}
}

func TestAnalyzeBoardDiff_NoChange(t *testing.T) {
	ge := engine.NewGame()

	var intersections [361]int8 // 全部空
	detected := DetectedBoard{Intersections: intersections}
	data, _ := json.Marshal(detected)

	resultJSON := AnalyzeBoardDiff(ge, string(data), 19, 19)

	var result AnalysisResult
	json.Unmarshal([]byte(resultJSON), &result)

	if len(result.Moves) != 0 {
		t.Errorf("expected no moves, got %d", len(result.Moves))
	}
	if len(result.Errors) == 0 {
		t.Error("expected 'no changes' error message")
	}
}

func TestAnalyzeBoardDiff_MultipleStones(t *testing.T) {
	ge := engine.NewGame()

	var intersections [361]int8
	intersections[3*19+3] = 1 // (3,3) に黒石
	intersections[15*19+15] = 2 // (15,15) に白石

	detected := DetectedBoard{Intersections: intersections}
	data, _ := json.Marshal(detected)

	resultJSON := AnalyzeBoardDiff(ge, string(data), 19, 19)

	var result AnalysisResult
	json.Unmarshal([]byte(resultJSON), &result)

	if len(result.Moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(result.Moves))
	}
	// 複数石検出時はconfidenceが低下
	if result.Confidence >= 1.0 {
		t.Errorf("expected reduced confidence, got %f", result.Confidence)
	}
	if len(result.Errors) == 0 {
		t.Error("expected error message for multiple stones")
	}
}

func TestAnalyzeBoardDiff_AfterMove(t *testing.T) {
	ge := engine.NewGame()
	ge.PlayMove(3, 3) // 黒が(3,3)に着手済み

	// (3,3)に黒石 + (15,15)に白石を検出
	var intersections [361]int8
	intersections[3*19+3] = 1  // 既存の黒石
	intersections[15*19+15] = 2 // 新しい白石

	detected := DetectedBoard{Intersections: intersections}
	data, _ := json.Marshal(detected)

	resultJSON := AnalyzeBoardDiff(ge, string(data), 19, 19)

	var result AnalysisResult
	json.Unmarshal([]byte(resultJSON), &result)

	// 差分は白石1つだけ
	if len(result.Moves) != 1 {
		t.Fatalf("expected 1 new move, got %d", len(result.Moves))
	}
	if result.Moves[0].X != 15 || result.Moves[0].Y != 15 {
		t.Errorf("expected move at (15,15), got (%d,%d)", result.Moves[0].X, result.Moves[0].Y)
	}
	if result.Moves[0].Color != "W" {
		t.Errorf("expected color W, got %s", result.Moves[0].Color)
	}
}

func TestAnalyzeBoardDiff_CaptureDetected(t *testing.T) {
	ge := engine.NewGame()
	// 黒: (1,0), 白: (0,0), 黒: (0,1) → 白(0,0)がキャプチャされる
	ge.PlayMove(1, 0) // B
	ge.PlayMove(0, 0) // W
	ge.PlayMove(0, 1) // B: (0,0)の白石をキャプチャ

	// 現在盤面: (1,0)黒, (0,1)黒, (0,0)空
	// 次の手を検出: (15,15)に白石
	var intersections [361]int8
	intersections[0*19+1] = 1  // (1,0) 黒 既存
	intersections[1*19+0] = 1  // (0,1) 黒 既存
	intersections[15*19+15] = 2 // (15,15) 白 新着手

	detected := DetectedBoard{Intersections: intersections}
	data, _ := json.Marshal(detected)

	resultJSON := AnalyzeBoardDiff(ge, string(data), 19, 19)

	var result AnalysisResult
	json.Unmarshal([]byte(resultJSON), &result)

	if len(result.Moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(result.Moves))
	}
}

func TestAnalyzeBoardDiff_InvalidJSON(t *testing.T) {
	ge := engine.NewGame()

	resultJSON := AnalyzeBoardDiff(ge, "not json", 19, 19)

	var result AnalysisResult
	json.Unmarshal([]byte(resultJSON), &result)

	if len(result.Errors) == 0 {
		t.Error("expected error for invalid JSON")
	}
}
