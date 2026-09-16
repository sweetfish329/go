// Package engine は囲碁のコアゲームエンジンを提供する。
package engine

import (
	"math/bits"
)

// BoardArea は 19×19 の全交点数 (361)
const BoardArea = 361

// Bitboard は 19×19 盤面の 361 交点を表す 6 ワード (64bit × 6 = 384bit) のビット配列。
// ナノ秒単位のビット並列演算により、呼吸点計算や死石判定を超高速に実行する。
type Bitboard [6]uint64

// PointIndex は (x, y) 座標から 0〜360 のビットインデックスを取得する
func PointIndex(x, y int) int {
	return y*19 + x
}

// IndexToPoint はビットインデックスから (x, y) 座標を復元する
func IndexToPoint(idx int) (x, y int) {
	return idx % 19, idx / 19
}

// Set は指定した交点のビットを立てる (1)
func (b *Bitboard) Set(x, y int) {
	idx := PointIndex(x, y)
	if idx >= 0 && idx < BoardArea {
		b[idx/64] |= (1 << (idx % 64))
	}
}

// Clear は指定した交点のビットを倒す (0)
func (b *Bitboard) Clear(x, y int) {
	idx := PointIndex(x, y)
	if idx >= 0 && idx < BoardArea {
		b[idx/64] &^= (1 << (idx % 64))
	}
}

// Test は指定した交点のビットが立っているかを返す
func (b *Bitboard) Test(x, y int) bool {
	idx := PointIndex(x, y)
	if idx < 0 || idx >= BoardArea {
		return false
	}
	return (b[idx/64] & (1 << (idx % 64))) != 0
}

// Count は立っているビットの総数 (石数) を CPU POPCNT 命令で高速集計する
func (b *Bitboard) Count() int {
	cnt := 0
	for i := 0; i < 6; i++ {
		cnt += bits.OnesCount64(b[i])
	}
	return cnt
}

// Or は 2 つのビットボードの論理和を計算する
// Or は 2 つのビットボードの論理和を計算する
func (b Bitboard) Or(o Bitboard) Bitboard {
	var res Bitboard
	for i := 0; i < 6; i++ {
		res[i] = b[i] | o[i]
	}
	return res
}

// And は 2 つのビットボードの論理積を計算する
func (b Bitboard) And(o Bitboard) Bitboard {
	var res Bitboard
	for i := 0; i < 6; i++ {
		res[i] = b[i] & o[i]
	}
	return res
}

// AndNot は b AND (NOT o) を計算する
func (b Bitboard) AndNot(o Bitboard) Bitboard {
	var res Bitboard
	for i := 0; i < 6; i++ {
		res[i] = b[i] &^ o[i]
	}
	return res
}

// IsEmpty はビットボードが空 (すべてのビットが 0) かを判定する
func (b Bitboard) IsEmpty() bool {
	for i := 0; i < 6; i++ {
		if b[i] != 0 {
			return false
		}
	}
	return true
}

// Adjacent は指定ビットボードのすべての石の上下左右隣接交点ビットマップを生成する
func (b Bitboard) Adjacent() Bitboard {
	var adj Bitboard
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			if b.Test(x, y) {
				if x > 0 {
					adj.Set(x-1, y)
				}
				if x < 18 {
					adj.Set(x+1, y)
				}
				if y > 0 {
					adj.Set(x, y-1)
				}
				if y < 18 {
					adj.Set(x, y+1)
				}
			}
		}
	}
	return adj
}

// BoardBitboards は盤面全体の白黒石状態をビットボードで管理する
type BoardBitboards struct {
	Black Bitboard
	White Bitboard
}

// NewBoardBitboardsFromGrid は 19×19 配列からビットボード表現を構築する
func NewBoardBitboardsFromGrid(grid [19][19]int8) BoardBitboards {
	var bb BoardBitboards
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			switch grid[y][x] {
			case 1:
				bb.Black.Set(x, y)
			case 2:
				bb.White.Set(x, y)
			}
		}
	}
	return bb
}

// Occupied は石が存在する全交点 (Black OR White) のビットボードを返す
func (bb *BoardBitboards) Occupied() Bitboard {
	return bb.Black.Or(bb.White)
}

// Empty は空交点のビットボードを返す (NOT Occupied かつ 盤内361ビット)
func (bb *BoardBitboards) Empty() Bitboard {
	var full Bitboard
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			full.Set(x, y)
		}
	}
	return full.AndNot(bb.Occupied())
}

// GetGroup は (x, y) にある石を含む連結した「連 (Group)」のビットマップをフラッドフィルで抽出する
func (bb *BoardBitboards) GetGroup(x, y int) (group Bitboard, color int8) {
	if bb.Black.Test(x, y) {
		color = 1
	} else if bb.White.Test(x, y) {
		color = 2
	} else {
		return group, 0
	}

	stones := bb.Black
	if color == 2 {
		stones = bb.White
	}

	// 連結成分探索
	group.Set(x, y)
	for {
		// 現在の連の隣接交点のうち、同色の石を連に追加
		expanded := group.Or(group.Adjacent().And(stones))
		if expanded == group {
			break
		}
		group = expanded
	}

	return group, color
}

// GetLiberties は (x, y) にある石の連の「呼吸点 (Liberties)」数およびそのビットボードを返す
func (bb *BoardBitboards) GetLiberties(x, y int) (libertyCount int, liberties Bitboard) {
	group, color := bb.GetGroup(x, y)
	if color == 0 {
		return 0, liberties
	}

	// 連の周囲交点のうち、石が置かれていない交点 (Empty) が呼吸点
	liberties = group.Adjacent().And(bb.Empty())
	return liberties.Count(), liberties
}
