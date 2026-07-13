package engine

import (
	"encoding/json"
	"testing"
)

func TestNewGame(t *testing.T) {
	ge := NewGame()
	if ge == nil {
		t.Fatal("NewGame returned nil")
	}
	if ge.MoveNumber() != 0 {
		t.Errorf("expected move 0, got %d", ge.MoveNumber())
	}
	if ge.CurrentPlayer() != 1 { // BLACK
		t.Errorf("expected black (1), got %d", ge.CurrentPlayer())
	}
}

func TestNewGameWithInfo(t *testing.T) {
	ge := NewGameWithInfo("黒太郎", "白花子", 6.5)
	sgf := ge.ExportSGF()
	if sgf == "" {
		t.Fatal("SGF should not be empty")
	}
	// SGFにプレイヤー情報が含まれているか
	if !containsSubstring(sgf, "PB[黒太郎]") {
		t.Errorf("SGF should contain PB, got: %s", sgf)
	}
	if !containsSubstring(sgf, "PW[白花子]") {
		t.Errorf("SGF should contain PW, got: %s", sgf)
	}
	if !containsSubstring(sgf, "KM[6.5]") {
		t.Errorf("SGF should contain KM, got: %s", sgf)
	}
}

func TestPlayMove(t *testing.T) {
	ge := NewGame()

	// 黒: (3,3)
	err := ge.PlayMove(3, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ge.MoveNumber() != 1 {
		t.Errorf("expected move 1, got %d", ge.MoveNumber())
	}
	if ge.CurrentPlayer() != 2 { // WHITE
		t.Errorf("expected white (2), got %d", ge.CurrentPlayer())
	}

	// 白: (15,15)
	err = ge.PlayMove(15, 15)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ge.MoveNumber() != 2 {
		t.Errorf("expected move 2, got %d", ge.MoveNumber())
	}
	if ge.CurrentPlayer() != 1 { // BLACK
		t.Errorf("expected black (1), got %d", ge.CurrentPlayer())
	}
}

func TestIllegalMove_SamePosition(t *testing.T) {
	ge := NewGame()
	ge.PlayMove(9, 9) // 黒が天元に着手

	// 同じ場所に打てない
	err := ge.PlayMove(9, 9)
	if err == nil {
		t.Error("expected error for illegal move on occupied point")
	}
}

func TestIllegalMove_OutOfBounds(t *testing.T) {
	ge := NewGame()

	// 盤外に打てない
	err := ge.PlayMove(19, 0)
	if err == nil {
		t.Error("expected error for out-of-bounds move")
	}
}

func TestPass(t *testing.T) {
	ge := NewGame()
	ge.Pass()
	if ge.MoveNumber() != 1 {
		t.Errorf("expected move 1 after pass, got %d", ge.MoveNumber())
	}
	if ge.CurrentPlayer() != 2 { // WHITE
		t.Errorf("expected white (2) after black pass, got %d", ge.CurrentPlayer())
	}
}

func TestResign(t *testing.T) {
	ge := NewGame()
	ge.PlayMove(3, 3) // 黒

	ge.Resign() // 白番で投了 → 黒の勝ち
	if !ge.resigned {
		t.Error("expected resigned to be true")
	}

	sgf := ge.ExportSGF()
	if !containsSubstring(sgf, "RE[B+R]") {
		t.Errorf("SGF should contain RE[B+R], got: %s", sgf)
	}

	// 投了後は着手できない
	err := ge.PlayMove(15, 15)
	if err == nil {
		t.Error("expected error when playing after resignation")
	}
}

func TestUndo(t *testing.T) {
	ge := NewGame()
	ge.PlayMove(3, 3)
	ge.PlayMove(15, 15)

	if !ge.Undo() {
		t.Error("undo should succeed")
	}
	if ge.MoveNumber() != 1 {
		t.Errorf("expected move 1 after undo, got %d", ge.MoveNumber())
	}
	if ge.CurrentPlayer() != 2 { // WHITE
		t.Errorf("expected white (2) after undo, got %d", ge.CurrentPlayer())
	}

	if !ge.Undo() {
		t.Error("second undo should succeed")
	}
	if ge.MoveNumber() != 0 {
		t.Errorf("expected move 0 after second undo, got %d", ge.MoveNumber())
	}

	// ルートノードでは Undo できない
	if ge.Undo() {
		t.Error("undo at root should fail")
	}
}

func TestExportImportSGF(t *testing.T) {
	ge := NewGameWithInfo("黒太郎", "白花子", 6.5)
	ge.PlayMove(3, 3)   // 黒: dd
	ge.PlayMove(15, 15) // 白: pp
	ge.PlayMove(15, 3)  // 黒: pd

	sgf := ge.ExportSGF()
	if sgf == "" {
		t.Fatal("exported SGF is empty")
	}

	// インポートテスト
	ge2 := NewGame()
	err := ge2.ImportSGF(sgf)
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	if ge2.MoveNumber() != 3 {
		t.Errorf("expected 3 moves after import, got %d", ge2.MoveNumber())
	}

	// 再エクスポートして一致確認
	sgf2 := ge2.ExportSGF()
	if sgf != sgf2 {
		t.Errorf("re-exported SGF differs:\n  original: %s\n  reimport: %s", sgf, sgf2)
	}
}

func TestImportSGF_Invalid(t *testing.T) {
	ge := NewGame()
	err := ge.ImportSGF("not valid sgf content")
	if err == nil {
		t.Error("expected error for invalid SGF")
	}
}

func TestBoardStateJSON(t *testing.T) {
	ge := NewGame()
	ge.PlayMove(3, 3)

	jsonStr := ge.BoardStateJSON()
	var state BoardState
	if err := json.Unmarshal([]byte(jsonStr), &state); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	if state.Stones[3][3] != 1 { // 黒石
		t.Errorf("expected black stone (1) at (3,3), got %d", state.Stones[3][3])
	}
	if state.Player != 2 { // 次は白番
		t.Errorf("expected white (2), got %d", state.Player)
	}
	if state.MoveNum != 1 {
		t.Errorf("expected moveNumber 1, got %d", state.MoveNum)
	}
}

func TestBoardStateJSON_EmptyBoard(t *testing.T) {
	ge := NewGame()
	jsonStr := ge.BoardStateJSON()
	var state BoardState
	if err := json.Unmarshal([]byte(jsonStr), &state); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	// 空盤面: 全て0
	for x := 0; x < 19; x++ {
		for y := 0; y < 19; y++ {
			if state.Stones[x][y] != 0 {
				t.Errorf("expected empty (0) at (%d,%d), got %d", x, y, state.Stones[x][y])
			}
		}
	}
}

func TestLastMoveInfoJSON(t *testing.T) {
	ge := NewGame()
	ge.PlayMove(9, 9)

	jsonStr := ge.LastMoveInfoJSON()
	var info LastMoveInfo
	if err := json.Unmarshal([]byte(jsonStr), &info); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	if info.X != 9 || info.Y != 9 {
		t.Errorf("expected (9,9), got (%d,%d)", info.X, info.Y)
	}
	if info.Color != "B" {
		t.Errorf("expected B, got %s", info.Color)
	}
	if info.MoveNumber != 1 {
		t.Errorf("expected moveNumber 1, got %d", info.MoveNumber)
	}
	if info.IsPass {
		t.Error("expected IsPass to be false")
	}
}

func TestLastMoveInfoJSON_Pass(t *testing.T) {
	ge := NewGame()
	ge.Pass()

	jsonStr := ge.LastMoveInfoJSON()
	var info LastMoveInfo
	if err := json.Unmarshal([]byte(jsonStr), &info); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	if info.Color != "B" {
		t.Errorf("expected B, got %s", info.Color)
	}
	if !info.IsPass {
		t.Error("expected IsPass to be true")
	}
}

func TestLastMoveInfoJSON_NoMoves(t *testing.T) {
	ge := NewGame()
	jsonStr := ge.LastMoveInfoJSON()
	var info LastMoveInfo
	if err := json.Unmarshal([]byte(jsonStr), &info); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if info.MoveNumber != 0 {
		t.Errorf("expected moveNumber 0, got %d", info.MoveNumber)
	}
}

func TestApplyDetectedMove(t *testing.T) {
	ge := NewGame()

	// 黒の手番で黒石を検出して適用
	result := ge.ApplyDetectedMove(3, 3, 1) // BLACK
	if result != 0 {
		t.Errorf("expected success (0), got %d", result)
	}
	if ge.MoveNumber() != 1 {
		t.Errorf("expected move 1, got %d", ge.MoveNumber())
	}
}

func TestApplyDetectedMove_WrongTurn(t *testing.T) {
	ge := NewGame()

	// 黒の手番で白石を検出（手番が合わない）→ 強制着手
	result := ge.ApplyDetectedMove(3, 3, 2) // WHITE
	if result != 0 {
		t.Errorf("expected success (0) for forced color, got %d", result)
	}
}

func TestApplyDetectedMove_AfterResign(t *testing.T) {
	ge := NewGame()
	ge.Resign()

	result := ge.ApplyDetectedMove(3, 3, 1)
	if result != 2 {
		t.Errorf("expected game-ended (2), got %d", result)
	}
}

func TestCapture(t *testing.T) {
	ge := NewGame()

	// 黒で白を囲んでキャプチャするシナリオ
	// 白: (0,0)
	// 黒: (1,0), (0,1)
	ge.PlayMove(1, 0) // B
	ge.PlayMove(0, 0) // W
	ge.PlayMove(0, 1) // B → (0,0)の白石をキャプチャ

	jsonStr := ge.BoardStateJSON()
	var state BoardState
	json.Unmarshal([]byte(jsonStr), &state)

	if state.Stones[0][0] != 0 {
		t.Errorf("expected captured stone removed at (0,0), got %d", state.Stones[0][0])
	}
	if state.CapturedB != 1 {
		t.Errorf("expected capturedByBlack=1, got %d", state.CapturedB)
	}
}

func TestMultipleMoves(t *testing.T) {
	ge := NewGame()

	// 30手打つ
	moves := [][2]int{
		{3, 3}, {15, 15}, {15, 3}, {3, 15},
		{9, 3}, {9, 15}, {3, 9}, {15, 9},
		{9, 9}, {6, 3}, {6, 15}, {12, 3},
		{12, 15}, {3, 6}, {15, 6}, {3, 12},
		{15, 12}, {6, 9}, {12, 9}, {9, 6},
		{9, 12}, {6, 6}, {12, 12}, {6, 12},
		{12, 6}, {7, 7}, {11, 11}, {7, 11},
		{11, 7}, {8, 8},
	}

	for i, m := range moves {
		err := ge.PlayMove(m[0], m[1])
		if err != nil {
			t.Fatalf("move %d (%d,%d) failed: %v", i+1, m[0], m[1], err)
		}
	}

	if ge.MoveNumber() != 30 {
		t.Errorf("expected 30 moves, got %d", ge.MoveNumber())
	}

	sgf := ge.ExportSGF()
	if sgf == "" {
		t.Fatal("SGF should not be empty after 30 moves")
	}
}

// helper
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
