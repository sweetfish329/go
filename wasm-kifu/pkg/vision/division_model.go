package vision

import (
	"image"
	"image/color"
	"math"
)

// DivisionModelCamera は1パラメータ分割モデル (Fitzgibbon 1-Parameter Division Model) によるレンズ歪み補正器
type DivisionModelCamera struct {
	Width       float64
	Height      float64
	CenterX     float64
	CenterY     float64
	MaxRadius   float64 // 対角線半径 (正規化用)
	K1          float64 // 歪みパラメータ (< 0 で樽型歪み補正)
}

// NewDivisionModelCamera は画像サイズと歪み係数 k1 からカメラモデルを作成する
func NewDivisionModelCamera(w, h float64, k1 float64) *DivisionModelCamera {
	cx := w / 2.0
	cy := h / 2.0
	diag := math.Hypot(cx, cy)
	return &DivisionModelCamera{
		Width:     w,
		Height:    h,
		CenterX:   cx,
		CenterY:   cy,
		MaxRadius: diag,
		K1:        k1,
	}
}

// UndistortPoint は歪んだ観測点 Pd を不歪化（直線復元）座標 Pu に変換する
// x_u = c_x + (x_d - c_x) / (1 + k1 * (r_d / R)^2)
func (cam *DivisionModelCamera) UndistortPoint(pd Point) Point {
	dx := pd.X - cam.CenterX
	dy := pd.Y - cam.CenterY
	rdSq := dx*dx + dy*dy
	rnSq := rdSq / (cam.MaxRadius * cam.MaxRadius)

	factor := 1.0 + cam.K1*rnSq
	if math.Abs(factor) < 1e-4 {
		factor = 1e-4
	}

	return Point{
		X: cam.CenterX + dx/factor,
		Y: cam.CenterY + dy/factor,
	}
}

// DistortPoint は不歪化座標 Pu を歪み画像上の座標 Pd に逆変換する
func (cam *DivisionModelCamera) DistortPoint(pu Point) Point {
	dx := pu.X - cam.CenterX
	dy := pu.Y - cam.CenterY
	ru := math.Hypot(dx, dy)
	if ru < 1e-6 {
		return pu
	}

	k := cam.K1

	// ru = rd / (1 + k * (rd/R)^2)
	// => k/R^2 * rd^2 - rd/ru + 1 = 0
	// 2次方程式 a * rd^2 + b * rd + c = 0
	// a = k / R^2, b = -1, c = ru
	if math.Abs(k) < 1e-7 {
		return pu
	}

	a := k * ru / (cam.MaxRadius * cam.MaxRadius)
	b := -1.0
	c := ru

	discriminant := b*b - 4.0*a*c
	if discriminant < 0 {
		return pu
	}

	// k->0 で rd->ru となる安定な解を選択
	// 分子有理化: rd = 2c / (-b + sqrt(discriminant))
	rd := (2.0 * c) / (1.0 + math.Sqrt(discriminant))
	scale := rd / ru

	return Point{
		X: cam.CenterX + dx*scale,
		Y: cam.CenterY + dy*scale,
	}
}

// UndistortCorners は4隅 [TL, TR, BR, BL] を不歪化する
func (cam *DivisionModelCamera) UndistortCorners(corners [4]Point) [4]Point {
	var res [4]Point
	for i := 0; i < 4; i++ {
		res[i] = cam.UndistortPoint(corners[i])
	}
	return res
}

// UndistortImage はバイリニア補間を用いて画像全体を不歪化する
func (cam *DivisionModelCamera) UndistortImage(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// dst(x, y) の不歪化座標に対応する元画像上の歪み座標を逆引き
			pu := Point{X: float64(x), Y: float64(y)}
			pd := cam.DistortPoint(pu)

			sx := pd.X
			sy := pd.Y

			// バイリニア補間サンプリング
			x0 := int(math.Floor(sx))
			y0 := int(math.Floor(sy))
			x1 := x0 + 1
			y1 := y0 + 1

			if x0 >= 0 && x1 < w && y0 >= 0 && y1 < h {
				fx := sx - float64(x0)
				fy := sy - float64(y0)

				c00 := src.At(x0, y0)
				c10 := src.At(x1, y0)
				c01 := src.At(x0, y1)
				c11 := src.At(x1, y1)

				r00, g00, b00, a00 := c00.RGBA()
				r10, g10, b10, a10 := c10.RGBA()
				r01, g01, b01, a01 := c01.RGBA()
				r11, g11, b11, a11 := c11.RGBA()

				interp := func(v00, v10, v01, v11 uint32) uint8 {
					top := float64(v00>>8)*(1.0-fx) + float64(v10>>8)*fx
					bot := float64(v01>>8)*(1.0-fx) + float64(v11>>8)*fx
					val := top*(1.0-fy) + bot*fy
					return uint8(math.Max(0, math.Min(255, math.Round(val))))
				}

				dst.Set(x, y, color.RGBA{
					R: interp(r00, r10, r01, r11),
					G: interp(g00, g10, g01, g11),
					B: interp(b00, b10, b01, b11),
					A: interp(a00, a10, a01, a11),
				})
			}
		}
	}

	return dst
}
