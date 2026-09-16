// Package main は wasm-kifu の WASM エントリーポイントを提供する。
// syscall/js パッケージを用いて JavaScript から呼び出し可能なグローバル関数を登録する。
package main

import (
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
	js.Global().Set("wasmGetLastMoveInfo", js.FuncOf(getLastMoveInfoWrapper))
	js.Global().Set("wasmDetectBoardCorners", js.FuncOf(detectBoardCornersWrapper))

	// WASM の実行を維持するためにチャネルで待機する
	select {}
}

