// Package vision は碁盤画像の多次元特徴量解析および幾何認識ロジックを提供する。
package vision

import (
	"math"
)

// RationalProjectivePitch は物理空間で等間隔に並ぶ19本の格子線に対し、
// 透視投影（射影不変量・複比 Cross-Ratio）に基づく厳密な1次元射影有理変換モデル
// f(t) = (H * r * t) / ((r - 1) * t + 18)  (r = sqrt(pStart / pEnd))
// を用いて、各格子行の非線形座標 baseY[0..18] を算出する。
func RationalProjectivePitch(boardH float64, pStart, pEnd float64) []float64 {
	baseY := make([]float64, BoardSize)
	if boardH <= 0 {
		return baseY
	}

	// ピッチ比率が指定されていない、または等比の場合は均等配分
	if pStart <= 0 || pEnd <= 0 || math.Abs(pStart-pEnd) < 1e-4 {
		step := boardH / float64(BoardSize-1)
		for i := 0; i < BoardSize; i++ {
			baseY[i] = float64(i) * step
		}
		baseY[BoardSize-1] = boardH
		return baseY
	}

	// 有理射影モデル: r = sqrt(pStart / pEnd)
	// 微分 f'(0) = a = p0, f'(18) = a / (18c + 1)^2 = p18 より導出
	r := math.Sqrt(pStart / pEnd)
	numDenomCoeff := r - 1.0

	for y := 0; y < BoardSize; y++ {
		t := float64(y)
		denom := numDenomCoeff*t + float64(BoardSize-1)
		if math.Abs(denom) < 1e-6 {
			denom = 1e-6
		}
		baseY[y] = (boardH * r * t) / denom
	}

	baseY[0] = 0.0
	baseY[BoardSize-1] = boardH
	return baseY
}
