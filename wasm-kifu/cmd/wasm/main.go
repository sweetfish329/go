// Package main は wasm-kifu の WASM エントリーポイントを提供する。
// syscall/js パッケージを用いて JavaScript から呼び出し可能なグローバル関数を登録する。
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"syscall/js"

	"github.com/sweetfish329/go/wasm-kifu/pkg/engine"
	"github.com/sweetfish329/go/wasm-kifu/pkg/vision"
)

// グローバルなゲームエンジンインスタンス
var gameEngine *engine.GameEngine

func newGameWrapper(this js.Value, args []js.Value) any {
	gameEngine = engine.NewGame()
	return nil
}

func newGameWithInfoWrapper(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return "invalid arguments"
	}
	pb := args[0].String()
	pw := args[1].String()
	komi := args[2].Float()
	gameEngine = engine.NewGameWithInfo(pb, pw, komi)
	return nil
}

func playMoveWrapper(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return 1 // エラー
	}
	x := args[0].Int()
	y := args[1].Int()
	err := gameEngine.PlayMove(x, y)
	if err != nil {
		return 1 // 違法手またはエラー
	}
	return 0 // 成功
}

func passWrapper(this js.Value, args []js.Value) any {
	gameEngine.Pass()
	return nil
}

func resignWrapper(this js.Value, args []js.Value) any {
	gameEngine.Resign()
	return nil
}

func undoWrapper(this js.Value, args []js.Value) any {
	if gameEngine.Undo() {
		return 0 // 成功
	}
	return 1 // 失敗
}

func exportSGFWrapper(this js.Value, args []js.Value) any {
	return gameEngine.ExportSGF()
}

func importSGFWrapper(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return 1
	}
	sgfString := args[0].String()
	err := gameEngine.ImportSGF(sgfString)
	if err != nil {
		return 1 // エラー
	}
	return 0 // 成功
}

func getCurrentPlayerWrapper(this js.Value, args []js.Value) any {
	return gameEngine.CurrentPlayer()
}

func getMoveNumberWrapper(this js.Value, args []js.Value) any {
	return gameEngine.MoveNumber()
}

func getBoardStateWrapper(this js.Value, args []js.Value) any {
	return gameEngine.BoardStateJSON()
}

func analyzeBoardImageWrapper(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return "{}"
	}
	imageData := args[0].String()
	width := args[1].Int()
	height := args[2].Int()
	return vision.AnalyzeBoardDiff(gameEngine, imageData, width, height)
}

func analyzeBoardFeaturesWrapper(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return "{}"
	}
	featuresJSON := args[0].String()
	return vision.AnalyzeBoardFeatures(gameEngine, featuresJSON)
}

func analyzeFullBoardWrapper(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return "{}"
	}
	featuresJSON := args[0].String()
	res, err := vision.AnalyzeFullBoard(featuresJSON)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return res
}

func applyDetectedMoveWrapper(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return 1
	}
	x := args[0].Int()
	y := args[1].Int()
	color := args[2].Int()
	return int(gameEngine.ApplyDetectedMove(x, y, color))
}

func setBoardFromGridWrapper(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return 1
	}
	var grid [19][19]int8
	arg := args[0]
	if arg.Type() == js.TypeString {
		var flatGrid []int8
		if err := json.Unmarshal([]byte(arg.String()), &flatGrid); err == nil && len(flatGrid) == 361 {
			for y := 0; y < 19; y++ {
				for x := 0; x < 19; x++ {
					grid[y][x] = flatGrid[y*19+x]
				}
			}
		} else {
			return 1
		}
	} else if arg.Type() == js.TypeObject {
		length := arg.Length()
		if length == 361 {
			for i := 0; i < 361; i++ {
				x := i % 19
				y := i / 19
				grid[y][x] = int8(arg.Index(i).Int())
			}
		} else if length == 19 {
			for y := 0; y < 19; y++ {
				row := arg.Index(y)
				for x := 0; x < 19; x++ {
					grid[y][x] = int8(row.Index(x).Int())
				}
			}
		} else {
			return 1
		}
	} else {
		return 1
	}

	if err := gameEngine.SetBoardFromGrid(grid); err != nil {
		return 1
	}
	return 0
}

func getLastMoveInfoWrapper(this js.Value, args []js.Value) any {
	return gameEngine.LastMoveInfoJSON()
}

func detectBoardCornersWrapper(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return `{"error": "invalid arguments"}`
	}
	uint8Array := args[0]
	w := args[1].Int()
	h := args[2].Int()

	length := w * h * 4
	pix := make([]byte, length)
	js.CopyBytesToGo(pix, uint8Array)

	img := &image.RGBA{
		Pix:    pix,
		Stride: w * 4,
		Rect:   image.Rect(0, 0, w, h),
	}

	res, err := vision.DetectBoardCorners(img)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	return fmt.Sprintf(`{"corners":[{"x":%.1f,"y":%.1f},{"x":%.1f,"y":%.1f},{"x":%.1f,"y":%.1f},{"x":%.1f,"y":%.1f}],"area":%.1f,"confidence":%.2f}`,
		res.Corners[0].X, res.Corners[0].Y,
		res.Corners[1].X, res.Corners[1].Y,
		res.Corners[2].X, res.Corners[2].Y,
		res.Corners[3].X, res.Corners[3].Y,
		res.Area, res.Confidence)
}

// detectAndWarpWrapper は四隅検出 + 射影変換をGo WASM側で一括実行する。
// 返却JSON: {"corners":[...], "area":..., "confidence":...,
//
//	"warpedWidth":680, "warpedHeight":732,
//	"warpedRgbaBase64":"..."}
func detectAndWarpWrapper(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return `{"error": "invalid arguments"}`
	}
	uint8Array := args[0]
	w := args[1].Int()
	h := args[2].Int()

	length := w * h * 4
	pix := make([]byte, length)
	js.CopyBytesToGo(pix, uint8Array)

	img := &image.RGBA{
		Pix:    pix,
		Stride: w * 4,
		Rect:   image.Rect(0, 0, w, h),
	}

	// Step 1: 四隅検出
	res, err := vision.DetectBoardCorners(img)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	// Step 2: 縦横比から最適出力サイズを決定 → 射影変換
	dstW, dstH, margin := vision.WarpSize(res.Corners)
	warped := vision.WarpPerspectiveWithMargin(img, res.Corners, dstW, dstH, margin)

	// RGBA バイト列を Base64 エンコードして返す
	encoded := base64.StdEncoding.EncodeToString(warped.Pix)

	return fmt.Sprintf(
		`{"corners":[{"x":%.1f,"y":%.1f},{"x":%.1f,"y":%.1f},{"x":%.1f,"y":%.1f},{"x":%.1f,"y":%.1f}],"area":%.1f,"confidence":%.2f,"warpedWidth":%d,"warpedHeight":%d,"warpedRgbaBase64":"%s"}`,
		res.Corners[0].X, res.Corners[0].Y,
		res.Corners[1].X, res.Corners[1].Y,
		res.Corners[2].X, res.Corners[2].Y,
		res.Corners[3].X, res.Corners[3].Y,
		res.Area, res.Confidence,
		dstW, dstH, encoded,
	)
}

// detectWarpAndAnalyzeWrapper は四隅検出 → 射影変換 → 特徴量抽出 → 盤面認識まで
// 全パイプラインを Go WASM 側で完結させる。
// 返却JSON: FullBoardResult {grid:[...361], confidence, blackCount, whiteCount, emptyCount}
func detectWarpAndAnalyzeWrapper(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return `{"error": "invalid arguments"}`
	}
	uint8Array := args[0]
	w := args[1].Int()
	h := args[2].Int()

	length := w * h * 4
	pix := make([]byte, length)
	js.CopyBytesToGo(pix, uint8Array)

	img := &image.RGBA{
		Pix:    pix,
		Stride: w * 4,
		Rect:   image.Rect(0, 0, w, h),
	}

	// Step 1: 四隅検出
	cornersRes, err := vision.DetectBoardCorners(img)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	// Step 2: 射影変換（平面化）
	dstW, dstH, margin := vision.WarpSize(cornersRes.Corners)
	warped := vision.WarpPerspectiveWithMargin(img, cornersRes.Corners, dstW, dstH, margin)

	// Step 3: 全361交点の特徴量抽出
	featData := vision.ExtractBoardFeatures(warped, dstW, dstH)

	// Step 4: 盤面認識（AnalyzeFullBoard は JSON 文字列を受け取る）
	featJSON, jsonErr := json.Marshal(featData)
	if jsonErr != nil {
		return fmt.Sprintf(`{"error": "json marshal failed: %s"}`, jsonErr.Error())
	}

	result, analyzeErr := vision.AnalyzeFullBoard(string(featJSON))
	if analyzeErr != nil {
		return fmt.Sprintf(`{"error": "%s"}`, analyzeErr.Error())
	}

	return result
}

func main() {
	// グローバルオブジェクトに関数を登録する
	js.Global().Set("wasmNewGame", js.FuncOf(newGameWrapper))
	js.Global().Set("wasmNewGameWithInfo", js.FuncOf(newGameWithInfoWrapper))
	js.Global().Set("wasmPlayMove", js.FuncOf(playMoveWrapper))
	js.Global().Set("wasmPass", js.FuncOf(passWrapper))
	js.Global().Set("wasmResign", js.FuncOf(resignWrapper))
	js.Global().Set("wasmUndo", js.FuncOf(undoWrapper))
	js.Global().Set("wasmExportSGF", js.FuncOf(exportSGFWrapper))
	js.Global().Set("wasmImportSGF", js.FuncOf(importSGFWrapper))
	js.Global().Set("wasmGetCurrentPlayer", js.FuncOf(getCurrentPlayerWrapper))
	js.Global().Set("wasmGetMoveNumber", js.FuncOf(getMoveNumberWrapper))
	js.Global().Set("wasmGetBoardState", js.FuncOf(getBoardStateWrapper))
	js.Global().Set("wasmAnalyzeBoardImage", js.FuncOf(analyzeBoardImageWrapper))
	js.Global().Set("wasmAnalyzeBoardFeatures", js.FuncOf(analyzeBoardFeaturesWrapper))
	js.Global().Set("wasmAnalyzeFullBoard", js.FuncOf(analyzeFullBoardWrapper))
	js.Global().Set("wasmApplyDetectedMove", js.FuncOf(applyDetectedMoveWrapper))
	js.Global().Set("wasmSetBoardFromGrid", js.FuncOf(setBoardFromGridWrapper))
	js.Global().Set("wasmGetLastMoveInfo", js.FuncOf(getLastMoveInfoWrapper))
	js.Global().Set("wasmDetectBoardCorners", js.FuncOf(detectBoardCornersWrapper))
	js.Global().Set("wasmDetectAndWarp", js.FuncOf(detectAndWarpWrapper))
	js.Global().Set("wasmDetectWarpAndAnalyze", js.FuncOf(detectWarpAndAnalyzeWrapper))

	// WASM の実行を維持するためにチャネルで待機する
	select {}
}

