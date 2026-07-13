// Package engine は棋譜記録のコアゲームエンジンを提供する。
// rooklift/sgf をラップし、着手管理・手番追跡・SGF生成を行う。
package engine

import (
	"encoding/json"
	"fmt"

	"github.com/rooklift/sgf"
)

// GameEngine は1局の囲碁ゲームを管理する
type GameEngine struct {
	root       *sgf.Node // SGFツリーのルート
	current    *sgf.Node // 現在のノード（最新の着手）
	moveNumber int       // 現在の手数
	resigned   bool      // 投了フラグ
}

// NewGame は19路盤の新規ゲームを開始する
func NewGame() *GameEngine {
	root := sgf.NewTree(19, 19)
	return &GameEngine{
		root:    root,
		current: root,
	}
}

// NewGameWithInfo はプレイヤー情報付きで新規ゲームを開始する
func NewGameWithInfo(pb, pw string, komi float64) *GameEngine {
	ge := NewGame()
	ge.root.SetValue("PB", pb)
	ge.root.SetValue("PW", pw)
	ge.root.SetValue("KM", fmt.Sprintf("%.1f", komi))
	return ge
}

// PlayMove は指定座標に着手する (0-indexed, 左上原点)
func (ge *GameEngine) PlayMove(x, y int) error {
	if ge.resigned {
		return fmt.Errorf("game already ended by resignation")
	}
	p := sgf.Point(x, y)
	next, err := ge.current.Play(p)
	if err != nil {
		return err
	}
	ge.current = next
	ge.moveNumber++
	return nil
}

// Pass はパスする
func (ge *GameEngine) Pass() {
	ge.current = ge.current.Pass()
	ge.moveNumber++
}

// Resign は投了する
func (ge *GameEngine) Resign() {
	ge.resigned = true
	// SGF に結果を記録
	board := ge.current.Board()
	if board.Player == sgf.BLACK {
		ge.root.SetValue("RE", "W+R")
	} else {
		ge.root.SetValue("RE", "B+R")
	}
}

// Undo は直前の着手を取り消す
func (ge *GameEngine) Undo() bool {
	parent := ge.current.Parent()
	if parent == nil {
		return false
	}
	ge.current = parent
	ge.moveNumber--
	return true
}

// ExportSGF は現在のゲームをSGF文字列として返す
func (ge *GameEngine) ExportSGF() string {
	return ge.root.SGF()
}

// ImportSGF はSGF文字列からゲームを復元する
func (ge *GameEngine) ImportSGF(s string) error {
	root, err := sgf.LoadSGF(s)
	if err != nil {
		return err
	}
	ge.root = root
	// メインラインの末端に移動
	ge.current = root.GetEnd()
	// 手数を計算
	line := ge.current.GetLine()
	ge.moveNumber = len(line) - 1 // ルートノードを除く
	ge.resigned = false
	return nil
}

// CurrentPlayer は現在の手番を返す (1=Black, 2=White)
func (ge *GameEngine) CurrentPlayer() int {
	board := ge.current.Board()
	return int(board.Player)
}

// MoveNumber は現在の手数を返す
func (ge *GameEngine) MoveNumber() int {
	return ge.moveNumber
}

// BoardState は盤面の状態を表す
type BoardState struct {
	// Stones は 19x19 の盤面。0=空, 1=黒, 2=白
	Stones    [19][19]int8 `json:"stones"`
	Player    int          `json:"player"`
	MoveNum   int          `json:"moveNumber"`
	Ko        string       `json:"ko,omitempty"`
	CapturedB int          `json:"capturedByBlack"`
	CapturedW int          `json:"capturedByWhite"`
	Resigned  bool         `json:"resigned"`
}

// BoardStateJSON は盤面状態をJSON文字列で返す
func (ge *GameEngine) BoardStateJSON() string {
	board := ge.current.Board()
	state := BoardState{
		Player:    int(board.Player),
		MoveNum:   ge.moveNumber,
		CapturedB: board.CapturesBy[sgf.BLACK],
		CapturedW: board.CapturesBy[sgf.WHITE],
		Resigned:  ge.resigned,
	}
	if board.Ko != "" {
		state.Ko = board.Ko
	}

	// 盤面をコピー
	for x := 0; x < 19; x++ {
		for y := 0; y < 19; y++ {
			state.Stones[x][y] = int8(board.State[x][y])
		}
	}

	data, _ := json.Marshal(state)
	return string(data)
}

// LastMoveInfo は直前の着手情報
type LastMoveInfo struct {
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Color      string `json:"color"`
	MoveNumber int    `json:"moveNumber"`
	IsPass     bool   `json:"isPass"`
}

// LastMoveInfoJSON は直前の着手情報をJSON文字列で返す
func (ge *GameEngine) LastMoveInfoJSON() string {
	info := LastMoveInfo{
		MoveNumber: ge.moveNumber,
	}
	if ge.moveNumber == 0 {
		data, _ := json.Marshal(info)
		return string(data)
	}

	// 直前の着手を取得
	if val, ok := ge.current.GetValue("B"); ok {
		info.Color = "B"
		if val == "" || val == "tt" {
			info.IsPass = true
		} else {
			x, y, _ := sgf.ParsePoint(val, 19, 19)
			info.X = x
			info.Y = y
		}
	} else if val, ok := ge.current.GetValue("W"); ok {
		info.Color = "W"
		if val == "" || val == "tt" {
			info.IsPass = true
		} else {
			x, y, _ := sgf.ParsePoint(val, 19, 19)
			info.X = x
			info.Y = y
		}
	}

	data, _ := json.Marshal(info)
	return string(data)
}

// ApplyDetectedMove は画像認識で検出された着手を適用する
// 戻り値: 0=成功, 1=違法手, 2=ゲーム終了
func (ge *GameEngine) ApplyDetectedMove(x, y, color int) int {
	if ge.resigned {
		return 2
	}

	// 手番チェック
	board := ge.current.Board()
	expectedColor := board.Player
	if int(expectedColor) != color {
		// 手番が合わない場合、色を指定して着手する（画像認識のため）
		p := sgf.Point(x, y)
		next, err := ge.current.PlayColour(p, sgf.Colour(color))
		if err != nil {
			return 1 // 違法手
		}
		ge.current = next
		ge.moveNumber++
		return 0
	}

	return ge.playMoveInternal(x, y)
}

func (ge *GameEngine) playMoveInternal(x, y int) int {
	p := sgf.Point(x, y)
	next, err := ge.current.Play(p)
	if err != nil {
		return 1
	}
	ge.current = next
	ge.moveNumber++
	return 0
}
