package vision

import (
	"image"
	"math"
)

// FörstnerRefineParams はサブピクセル交点精細化のパラメータ
type FörstnerRefineParams struct {
	WindowRadius int     // 探索ウィンドウ半径 (例: 5 => 11x11)
	MaxShift     float64 // 許容最大シフト量 (ピクセル, 例: 5.0)
	MinCornerDet float64 // 構造テンソル最小行列式閾値
	MinStrength  float64 // 最小エッジ強度
}

// DefaultFörstnerParams は標準パラメータを返す
func DefaultFörstnerParams() FörstnerRefineParams {
	return FörstnerRefineParams{
		WindowRadius: 6,
		MaxShift:     5.0,
		MinCornerDet: 100.0,
		MinStrength:  5.0,
	}
}

// RefineCornerSubPixFörstner はFörstner演算子 (構造テンソル) を用いて、
// 初期交点座標を0.1ピクセル以下のサブピクセル精度に精細化する。
// 輝度勾配の直交エッジ交点に対する重み付き二乗距離を反復最小化する。
func RefineCornerSubPixFörstner(img image.Image, initPt Point, params FörstnerRefineParams) Point {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	r := params.WindowRadius

	getGray := func(x, y int) float64 {
		r32, g32, b32, _ := img.At(x, y).RGBA()
		return 0.299*float64(r32>>8) + 0.587*float64(g32>>8) + 0.114*float64(b32>>8)
	}

	curPt := initPt
	maxIter := 5
	sigma := float64(r) / 2.0
	twoSigmaSq := 2.0 * sigma * sigma

	for iter := 0; iter < maxIter; iter++ {
		ix := int(math.Round(curPt.X))
		iy := int(math.Round(curPt.Y))

		// 画像境界チェック
		if ix-r-1 < 0 || ix+r+1 >= w || iy-r-1 < 0 || iy+r+1 >= h {
			break
		}

		var m00, m01, m11 float64
		var b0, b1 float64

		for dy := -r; dy <= r; dy++ {
			py := iy + dy
			for dx := -r; dx <= r; dx++ {
				px := ix + dx

				gx := (getGray(px+1, py) - getGray(px-1, py)) * 0.5
				gy := (getGray(px, py+1) - getGray(px, py-1)) * 0.5

				// ガウシアン空間重み (現在のサブピクセル中心からの距離)
				distSq := math.Pow(float64(px)-curPt.X, 2) + math.Pow(float64(py)-curPt.Y, 2)
				weight := math.Exp(-distSq / twoSigmaSq)

				gx2 := gx * gx * weight
				gy2 := gy * gy * weight
				gxy := gx * gy * weight

				m00 += gx2
				m01 += gxy
				m11 += gy2

				fx := float64(px)
				fy := float64(py)
				b0 += gx2*fx + gxy*fy
				b1 += gxy*fx + gy2*fy
			}
		}

		det := m00*m11 - m01*m01
		trace := m00 + m11

		if det <= params.MinCornerDet || trace <= params.MinStrength {
			break
		}

		cornerStrength := det / trace
		if cornerStrength < params.MinStrength {
			break
		}

		nextX := (m11*b0 - m01*b1) / det
		nextY := (m00*b1 - m01*b0) / det

		if math.IsNaN(nextX) || math.IsNaN(nextY) {
			break
		}

		shiftFromInit := math.Hypot(nextX-initPt.X, nextY-initPt.Y)
		if shiftFromInit > params.MaxShift {
			break
		}

		stepShift := math.Hypot(nextX-curPt.X, nextY-curPt.Y)
		curPt = Point{X: nextX, Y: nextY}

		if stepShift < 0.01 {
			// 0.01px以下に収束
			break
		}
	}

	return curPt
}

// RefineCornersSubPix は碁盤の4隅 [TL, TR, BR, BL] をサブピクセル精度で精細化する
func RefineCornersSubPix(img image.Image, corners [4]Point) [4]Point {
	params := DefaultFörstnerParams()
	var refined [4]Point
	for i := 0; i < 4; i++ {
		refined[i] = RefineCornerSubPixFörstner(img, corners[i], params)
	}
	return refined
}
