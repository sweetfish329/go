// Package vision は盤面差分解析ロジックを提供する。
// OpenCV.js側から受け取った交点状態を分析し、着手を推定する。
package vision

import (
	"encoding/json"
	"fmt"

	"github.com/sweetfish329/go/wasm-kifu/pkg/engine"
)

// BoardSize は碁盤のサイズ（19路盤固定）
const BoardSize = 19

// IntersectionState は各交点の状態
type IntersectionState int8

const (
	Empty IntersectionState = 0
	Black IntersectionState = 1
	White IntersectionState = 2
)

// DetectedBoard はOpenCV.jsから受け取る盤面データ
type DetectedBoard struct {
	// Intersections は19×19の交点状態配列 (row-major: [y*19+x])
	Intersections [BoardSize * BoardSize]int8 `json:"intersections"`
}

// DetectedMove は検出された着手
type DetectedMove struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Color string `json:"color"` // "B" or "W"
}

// AnalysisResult は盤面解析の結果
type AnalysisResult struct {
	Moves      []DetectedMove `json:"moves"`
	Removed    []DetectedMove `json:"removed,omitempty"` // 取られた石
	Errors     []string       `json:"errors,omitempty"`
	Confidence float64        `json:"confidence"`
}

// AnalyzeBoardDiff は現在のゲーム状態と検出された盤面を比較し、
// 新しい着手を推定する
func AnalyzeBoardDiff(ge *engine.GameEngine, imageData string, width, height int) string {
	var detected DetectedBoard
	if err := json.Unmarshal([]byte(imageData), &detected); err != nil {
		result := AnalysisResult{
			Errors: []string{fmt.Sprintf("JSON parse error: %v", err)},
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	// 現在のゲーム盤面を取得
	currentJSON := ge.BoardStateJSON()
	var currentState engine.BoardState
	json.Unmarshal([]byte(currentJSON), &currentState)

	result := AnalysisResult{
		Confidence: 1.0,
	}

	// 差分を計算
	var newStones []DetectedMove
	var removedStones []DetectedMove

	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			detectedState := detected.Intersections[y*BoardSize+x]
			currentStone := currentState.Stones[x][y]

			if detectedState != int8(currentStone) {
				if detectedState != 0 && currentStone == 0 {
					// 新しい石が置かれた
					color := "B"
					if detectedState == 2 {
						color = "W"
					}
					newStones = append(newStones, DetectedMove{
						X: x, Y: y, Color: color,
					})
				} else if detectedState == 0 && currentStone != 0 {
					// 石が取り除かれた（アゲハマ）
					color := "B"
					if currentStone == 2 {
						color = "W"
					}
					removedStones = append(removedStones, DetectedMove{
						X: x, Y: y, Color: color,
					})
				} else {
					// 石の色が変わった（誤検出の可能性）
					result.Errors = append(result.Errors,
						fmt.Sprintf("stone color changed at (%d,%d): %d -> %d", x, y, currentStone, detectedState))
					result.Confidence *= 0.5
				}
			}
		}
	}

	// 着手の妥当性チェック
	if len(newStones) == 0 && len(removedStones) == 0 {
		// 変化なし
		result.Errors = append(result.Errors, "no changes detected")
	} else if len(newStones) == 1 && len(removedStones) == 0 {
		// 理想的: 1つの新しい石、取られた石なし
		result.Moves = newStones
	} else if len(newStones) == 1 && len(removedStones) > 0 {
		// 1つの新しい石 + 取られた石（正常なキャプチャ）
		result.Moves = newStones
		result.Removed = removedStones
	} else if len(newStones) > 1 {
		// 複数の新しい石（複数手が進んだか、誤検出）
		result.Moves = newStones
		result.Errors = append(result.Errors,
			fmt.Sprintf("multiple new stones detected (%d), possible multi-move or detection error", len(newStones)))
		result.Confidence *= 0.3
	}

	data, _ := json.Marshal(result)
	return string(data)
}
