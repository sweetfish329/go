// Package engine は囲碁のコアゲームエンジンを提供する。
package engine

import (
	"math/rand"
	"sync"
)

// ZobristTable は 19×19 の全交点と色に対する Zobrist ハッシュテーブル。
// 64bit XOR 演算により、定数時間 O(1) で盤面状態のハッシュを計算・更新できる。
type ZobristTable struct {
	table      [19][19][3]uint64 // [x][y][color: 0=Empty(未使用), 1=Black, 2=White]
	turnBlack  uint64            // 黒番ハッシュ
	turnWhite  uint64            // 白番ハッシュ
	koPoint    [19][19]uint64    // コウ着手禁止点ハッシュ
	initialized bool
}

var (
	globalZobrist ZobristTable
	zobristOnce   sync.Once
)

// InitZobrist は再現性のある固定シードから Zobrist テーブルを初期化する
func InitZobrist() {
	zobristOnce.Do(func() {
		r := rand.New(rand.NewSource(1844674407370955161)) // 固定シード (囲碁定数)
		for y := 0; y < 19; y++ {
			for x := 0; x < 19; x++ {
				for c := 1; c <= 2; c++ {
					globalZobrist.table[x][y][c] = r.Uint64()
				}
				globalZobrist.koPoint[x][y] = r.Uint64()
			}
		}
		globalZobrist.turnBlack = r.Uint64()
		globalZobrist.turnWhite = r.Uint64()
		globalZobrist.initialized = true
	})
}

// ComputeHashFromGrid は 19×19 配列 (0=空, 1=黒, 2=白) から盤面全体の 64bit Zobrist ハッシュを算出する
func ComputeHashFromGrid(grid [19][19]int8, turn int) uint64 {
	InitZobrist()
	var h uint64

	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			c := grid[y][x]
			if c == 1 || c == 2 {
				h ^= globalZobrist.table[x][y][c]
			}
		}
	}

	if turn == 1 {
		h ^= globalZobrist.turnBlack
	} else if turn == 2 {
		h ^= globalZobrist.turnWhite
	}

	return h
}

// UpdateHashMove は 1 手の着手/アゲハマによるハッシュの差分更新を O(1) で行う
func UpdateHashMove(currentHash uint64, x, y int, color int8) uint64 {
	InitZobrist()
	if x >= 0 && x < 19 && y >= 0 && y < 19 && (color == 1 || color == 2) {
		return currentHash ^ globalZobrist.table[x][y][color]
	}
	return currentHash
}
