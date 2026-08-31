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
	intersections[3*19+3] = 1   // (3,3) に黒石
	intersections[15*19+15] = 2 // (15,15) に白石

	detected := DetectedBoard{Intersections: intersections}
	data, _ := json.Marshal(detected)

	resultJSON := AnalyzeBoardDiff(ge, string(data), 19, 19)

	var result AnalysisResult
	json.Unmarshal([]byte(resultJSON), &result)

	if len(result.Moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(result.Moves))
	}
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
	intersections[3*19+3] = 1   // 既存の黒石
	intersections[15*19+15] = 2 // 新しい白石

	detected := DetectedBoard{Intersections: intersections}
	data, _ := json.Marshal(detected)

	resultJSON := AnalyzeBoardDiff(ge, string(data), 19, 19)

	var result AnalysisResult
	json.Unmarshal([]byte(resultJSON), &result)

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
	ge.PlayMove(1, 0) // B
	ge.PlayMove(0, 0) // W
	ge.PlayMove(0, 1) // B: (0,0)の白石をキャプチャ

	var intersections [361]int8
	intersections[0*19+1] = 1   // (1,0) 黒 既存
	intersections[1*19+0] = 1   // (0,1) 黒 既存
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

func TestAnalyzeBoardFeatures_DynamicMean(t *testing.T) {
	ge := engine.NewGame() // 黒番

	// 盤面全体を空交点特徴（木目: L=145, C=25, D=180, S=110）で初期化
	var data BoardFeaturesData
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			data.Features[y*19+x] = IntersectionFeatures{
				X: x, Y: y,
				L: 145.0, C: 25.0, D: 180.0, H: 35.0, S: 110.0,
			}
		}
	}

	// (3,3) に黒石を配置（滑らか・無彩色・低輝度: L=30, C=3, D=20, S=10）
	data.Features[3*19+3] = IntersectionFeatures{
		X: 3, Y: 3,
		L: 30.0, C: 3.0, D: 20.0, H: 0.0, S: 10.0,
	}

	rawJSON, _ := json.Marshal(data)
	resultJSON := AnalyzeBoardFeatures(ge, string(rawJSON))

	var result AnalysisResult
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(result.Moves) != 1 {
		t.Fatalf("expected 1 detected move, got %d (errors: %v)", len(result.Moves), result.Errors)
	}

	move := result.Moves[0]
	if move.X != 3 || move.Y != 3 || move.Color != "B" {
		t.Errorf("expected Black at (3,3), got %s at (%d,%d)", move.Color, move.X, move.Y)
	}
}

func TestAnalyzeBoardFeatures_ShadowSeparation(t *testing.T) {
	ge := engine.NewGame() // 黒番

	// 盤面全体を木目で初期化
	var data BoardFeaturesData
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			data.Features[y*19+x] = IntersectionFeatures{
				X: x, Y: y,
				L: 145.0, C: 25.0, D: 180.0, H: 35.0, S: 110.0,
			}
		}
	}

	// (5,5) に「暗い影（木目あり）」: 輝度 L=50 は低いが、木目色度 C=22、不均一性 D=150 は高い
	data.Features[5*19+5] = IntersectionFeatures{
		X: 5, Y: 5,
		L: 50.0, C: 22.0, D: 150.0, H: 35.0, S: 90.0,
	}

	// (3,3) に「本物の黒石」: 輝度 L=32、色度 C=2（無彩色）、不均一性 D=18（平滑）
	data.Features[3*19+3] = IntersectionFeatures{
		X: 3, Y: 3,
		L: 32.0, C: 2.0, D: 18.0, H: 0.0, S: 10.0,
	}

	rawJSON, _ := json.Marshal(data)
	resultJSON := AnalyzeBoardFeatures(ge, string(rawJSON))

	var result AnalysisResult
	json.Unmarshal([]byte(resultJSON), &result)

	if len(result.Moves) != 1 {
		t.Fatalf("expected exactly 1 move (real stone, not shadow), got %d (errors: %v)", len(result.Moves), result.Errors)
	}

	// 影 (5,5) ではなく 本物の黒石 (3,3) が正しく選択されるべき
	if result.Moves[0].X != 3 || result.Moves[0].Y != 3 {
		t.Errorf("expected real stone at (3,3), but detected (%d,%d)", result.Moves[0].X, result.Moves[0].Y)
	}
}
