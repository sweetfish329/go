// Package main は wasm-kifu の WASM エントリーポイントを提供する。
// syscall/js パッケージを用いて JavaScript から呼び出し可能なグローバル関数を登録する。
package main

import (
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

func applyDetectedMoveWrapper(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return 1
	}
	x := args[0].Int()
	y := args[1].Int()
	color := args[2].Int()
	return gameEngine.ApplyDetectedMove(x, y, color)
}

func getLastMoveInfoWrapper(this js.Value, args []js.Value) any {
	return gameEngine.LastMoveInfoJSON()
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
	js.Global().Set("wasmApplyDetectedMove", js.FuncOf(applyDetectedMoveWrapper))
	js.Global().Set("wasmGetLastMoveInfo", js.FuncOf(getLastMoveInfoWrapper))

	// WASM の実行を維持するためにチャネルで待機する
	select {}
}
