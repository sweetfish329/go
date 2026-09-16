// Package engine は囲碁のコアゲームエンジンを提供する。
package engine

import (
	"fmt"
)

// BoardAnomaly は盤面認識におけるルール違反・異常箇所を表す
type BoardAnomaly struct {
	X, Y        int
	Color       int8
	Reason      string
	RepairedTo  int8
}

// FindZeroLibertyGroups は盤面上の呼吸点が 0 の連（死石/アゲハマ残り/誤認識）をすべて検出する
func (bb *BoardBitboards) FindZeroLibertyGroups() (deadBlack, deadWhite Bitboard) {
	visited := Bitboard{}

	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			if visited.Test(x, y) {
				continue
			}

			if bb.Black.Test(x, y) {
				group, _ := bb.GetGroup(x, y)
				visited = visited.Or(group)
				liberties, _ := bb.GetLiberties(x, y)
				if liberties == 0 {
					deadBlack = deadBlack.Or(group)
				}
			} else if bb.White.Test(x, y) {
				group, _ := bb.GetGroup(x, y)
				visited = visited.Or(group)
				liberties, _ := bb.GetLiberties(x, y)
				if liberties == 0 {
					deadWhite = deadWhite.Or(group)
				}
			}
		}
	}

	return deadBlack, deadWhite
}

// RepairBoardWithRules は画像認識結果に対し、囲碁の合法ルール（呼吸点0の禁止など）を制約として
// ノイズやアゲハマ未回収を自己修復（Auto-Repair）した盤面を返す
func RepairBoardWithRules(grid [19][19]int8) ([19][19]int8, []BoardAnomaly) {
	repaired := grid
	var anomalies []BoardAnomaly

	// 最大 3 回の反復修復 (連鎖的な呼吸点回復に対応)
	for iter := 0; iter < 3; iter++ {
		bb := NewBoardBitboardsFromGrid(repaired)
		deadBlack, deadWhite := bb.FindZeroLibertyGroups()

		if deadBlack.IsEmpty() && deadWhite.IsEmpty() {
			break // ルール違反のない安定状態
		}

		// 黒の呼吸点ゼロの石の修復
		for y := 0; y < 19; y++ {
			for x := 0; x < 19; x++ {
				if deadBlack.Test(x, y) {
					repaired[y][x] = 0 // アゲハマまたは誤検出として空に修復
					anomalies = append(anomalies, BoardAnomaly{
						X:          x,
						Y:          y,
						Color:      1,
						Reason:     fmt.Sprintf("呼吸点 0 の黒石連を検知 (アゲハマ未回収またはノイズ) -> 空交点に自己修復"),
						RepairedTo: 0,
					})
				}
			}
		}

		// 白の呼吸点ゼロの石の修復
		for y := 0; y < 19; y++ {
			for x := 0; x < 19; x++ {
				if deadWhite.Test(x, y) {
					repaired[y][x] = 0 // アゲハマまたは誤検出として空に修復
					anomalies = append(anomalies, BoardAnomaly{
						X:          x,
						Y:          y,
						Color:      2,
						Reason:     fmt.Sprintf("呼吸点 0 の白石連を検知 (アゲハマ未回収またはノイズ) -> 空交点に自己修復"),
						RepairedTo: 0,
					})
				}
			}
		}
	}

	return repaired, anomalies
}
