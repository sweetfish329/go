// Package vision の warp.go は碁盤画像の射影変換（パースペクティブ補正）と
// ワープ後の画像から全361交点の多次元特徴量を抽出するライブラリ関数を提供する。
//
// eval_test3/main.go の warpPerspectiveWithMargin を pkg として昇格し、
// board-detector.js の _perspectiveTransform / _extractFeaturesFromWarped を純Goへ移植。
package vision

import (
	"image"
	"image/color"
	"math"
)

// solveLinearSystem は8×8の連立一次方程式をガウス消去法で解く。
// 射影行列計算 (getPerspectiveMatrix) から使用する。
func solveLinearSystem(a [8][8]float64, b [8]float64) [8]float64 {
	var x [8]float64
	for i := 0; i < 8; i++ {
		// ピボット選択 (部分ピボット)
		maxRow := i
		for k := i + 1; k < 8; k++ {
			if math.Abs(a[k][i]) > math.Abs(a[maxRow][i]) {
				maxRow = k
			}
		}
		a[i], a[maxRow] = a[maxRow], a[i]
		b[i], b[maxRow] = b[maxRow], b[i]
		pivot := a[i][i]
		if math.Abs(pivot) < 1e-9 {
			continue
		}
		for j := i; j < 8; j++ {
			a[i][j] /= pivot
		}
		b[i] /= pivot
		for k := 0; k < 8; k++ {
			if k != i {
				factor := a[k][i]
				for j := i; j < 8; j++ {
					a[k][j] -= factor * a[i][j]
				}
				b[k] -= factor * b[i]
			}
		}
	}
	for i := 0; i < 8; i++ {
		x[i] = b[i]
	}
	return x
}

// getPerspectiveMatrix は4対応点 (src→dst) から3×3射影変換行列を算出する。
// 返値は行優先 [h0..h7, 1.0] の9要素配列。
func getPerspectiveMatrix(src [4]Point, dst [4]Point) [9]float64 {
	var a [8][8]float64
	var b [8]float64
	for i := 0; i < 4; i++ {
		sx, sy := src[i].X, src[i].Y
		dx, dy := dst[i].X, dst[i].Y
		a[i*2] = [8]float64{sx, sy, 1, 0, 0, 0, -sx * dx, -sy * dx}
		b[i*2] = dx
		a[i*2+1] = [8]float64{0, 0, 0, sx, sy, 1, -sx * dy, -sy * dy}
		b[i*2+1] = dy
	}
	h := solveLinearSystem(a, b)
	return [9]float64{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1.0}
}

// transformPoint は射影変換行列 m を用いて座標 (u, v) を変換する。
func transformPoint(m [9]float64, u, v float64) (float64, float64) {
	denom := m[6]*u + m[7]*v + m[8]
	if math.Abs(denom) < 1e-9 {
		denom = 1e-9
	}
	return (m[0]*u + m[1]*v + m[2]) / denom, (m[3]*u + m[4]*v + m[5]) / denom
}

// getPixelBilinear はバイリニア補間でピクセル色を取得する。
// 範囲外は最近傍に丸める。
func getPixelBilinear(img *image.RGBA, x, y float64) color.RGBA {
	b := img.Bounds()
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	// 境界チェック: 補間に必要な4点が全て範囲内であること
	if ix < b.Min.X || ix+1 >= b.Max.X || iy < b.Min.Y || iy+1 >= b.Max.Y {
		// 範囲内にクランプして最近傍ピクセルを返す
		cx := ix
		if cx < b.Min.X {
			cx = b.Min.X
		} else if cx >= b.Max.X {
			cx = b.Max.X - 1
		}
		cy := iy
		if cy < b.Min.Y {
			cy = b.Min.Y
		} else if cy >= b.Max.Y {
			cy = b.Max.Y - 1
		}
		return img.RGBAAt(cx, cy)
	}
	fx, fy := x-float64(ix), y-float64(iy)
	c00 := img.RGBAAt(ix, iy)
	c10 := img.RGBAAt(ix+1, iy)
	c01 := img.RGBAAt(ix, iy+1)
	c11 := img.RGBAAt(ix+1, iy+1)

	interp := func(v00, v10, v01, v11 uint8) uint8 {
		top := float64(v00)*(1-fx) + float64(v10)*fx
		bot := float64(v01)*(1-fx) + float64(v11)*fx
		return uint8(top*(1-fy) + bot*fy)
	}
	return color.RGBA{
		R: interp(c00.R, c10.R, c01.R, c11.R),
		G: interp(c00.G, c10.G, c01.G, c11.G),
		B: interp(c00.B, c10.B, c01.B, c11.B),
		A: 255,
	}
}

// toRGBA は任意の image.Image を *image.RGBA に変換する。
// すでに *image.RGBA の場合はそのまま返す。
func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	b := img.Bounds()
	rgba := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	return rgba
}

// WarpPerspectiveWithMargin は指定した四隅と外周マージンで射影変換（透視補正）を実行し、
// 正面視点に変換された *image.RGBA を返す。
//
// srcCorners: 入力画像上の四隅座標 [TL, TR, BR, BL]
// dstW, dstH: 出力画像サイズ
// margin: 出力画像内の余白 (px)。格子四隅は (margin, margin) 〜 (dstW-margin, dstH-margin) に配置される。
func WarpPerspectiveWithMargin(src image.Image, srcCorners [4]Point, dstW, dstH int, margin float64) *image.RGBA {
	dstCorners := [4]Point{
		{X: margin, Y: margin},
		{X: float64(dstW) - margin, Y: margin},
		{X: float64(dstW) - margin, Y: float64(dstH) - margin},
		{X: margin, Y: float64(dstH) - margin},
	}
	// 逆射影行列: dst座標からsrc座標へのマッピング
	invMat := getPerspectiveMatrix(dstCorners, srcCorners)

	srcRGBA := toRGBA(src)
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			sx, sy := transformPoint(invMat, float64(x), float64(y))
			dst.SetRGBA(x, y, getPixelBilinear(srcRGBA, sx, sy))
		}
	}
	return dst
}

// WarpSize は碁盤の縦横比 (日本・韓国規格: 約1.077) と四隅形状から
// 最適な出力サイズ (dstW, dstH) とマージン (40px) を決定する。
//
// 縦長（通常の対局者視点）: dstW=680, dstH=732
// 横長（横からの撮影）: dstW=732, dstH=680
func WarpSize(corners [4]Point) (dstW, dstH int, margin float64) {
	// 四辺の長さを計算して縦横を判定
	wTop := math.Hypot(corners[1].X-corners[0].X, corners[1].Y-corners[0].Y)
	wBot := math.Hypot(corners[2].X-corners[3].X, corners[2].Y-corners[3].Y)
	hLeft := math.Hypot(corners[3].X-corners[0].X, corners[3].Y-corners[0].Y)
	hRight := math.Hypot(corners[2].X-corners[1].X, corners[2].Y-corners[1].Y)
	avgW := (wTop + wBot) / 2
	avgH := (hLeft + hRight) / 2

	margin = 40.0
	if avgH >= avgW {
		// 縦長 (通常視点): 碁盤縦横比 1.077 = 45.5cm / 42.4cm
		dstW = 680
		dstH = 732 // round(680 * 1.077)
	} else {
		// 横長 (横撮り): 縦横を入れ替え
		dstW = 732
		dstH = 680
	}
	return
}

// ExtractBoardFeatures はワープ済みの正面画像 (*image.RGBA) から
// 全361交点の多次元特徴量 (IntersectionFeatures) を抽出する。
//
// board-detector.js の _extractFeaturesFromWarped を純Goへ移植。
// dstW, dstH は WarpPerspectiveWithMargin に渡したものと同じ値を使用すること。
func ExtractBoardFeatures(warped *image.RGBA, dstW, dstH int) *BoardFeaturesData {
	const margin = 40.0
	boardW := float64(dstW) - 2*margin
	boardH := float64(dstH) - 2*margin

	// 消失点・複比（Cross-Ratio）に基づく有理関数透視ピッチモデル
	// f(t) = (boardH * rRatio * t) / ((rRatio - 1) * t + 18)
	// pStart=33.0 (奥行間隔), pEnd=39.0 (手前間隔)
	pStart := 33.0
	pEnd := 39.0
	rRatio := math.Sqrt(pStart / pEnd)
	numDenomCoeff := rRatio - 1.0

	gridY := make([]float64, BoardSize)
	for i := 0; i < BoardSize; i++ {
		t := float64(i)
		denom := numDenomCoeff*t + float64(BoardSize-1)
		var yOff float64
		if denom != 0 {
			yOff = (boardH * rRatio * t) / denom
		} else {
			yOff = boardH * (t / float64(BoardSize-1))
		}
		gridY[i] = margin + math.Round(yOff)
	}
	gridY[0] = margin
	gridY[BoardSize-1] = margin + boardH

	stepX := boardW / float64(BoardSize-1)
	gridX := make([]float64, BoardSize)
	for i := 0; i < BoardSize; i++ {
		gridX[i] = margin + math.Round(float64(i)*stepX)
	}

	bounds := warped.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()

	// ドーナツ走査: 内半径 innerR の中心格子線を除外し、外半径 outerR まで走査
	const innerR = 2
	const outerR = 7

	data := &BoardFeaturesData{}

	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			cx := int(gridX[x] + 0.5)
			cy := int(gridY[y] + 0.5)

			var sumR, sumG, sumB, count float64
			var lVals []float64 // 不均一性(D)計算用

			for dy := -outerR; dy <= outerR; dy++ {
				for dx := -outerR; dx <= outerR; dx++ {
					dist2 := dx*dx + dy*dy
					if dist2 > outerR*outerR {
						continue
					}
					// ドーナツ内側（格子線中心）を除外
					if dist2 <= innerR*innerR {
						continue
					}
					px, py := cx+dx, cy+dy
					if px < 0 || px >= imgW || py < 0 || py >= imgH {
						continue
					}
					c := warped.RGBAAt(px, py)
					r := float64(c.R)
					g := float64(c.G)
					b := float64(c.B)
					l := 0.299*r + 0.587*g + 0.114*b
					sumR += r
					sumG += g
					sumB += b
					lVals = append(lVals, l)
					count++
				}
			}

			avgR, avgG, avgB := 80.0, 60.0, 40.0
			if count > 0 {
				avgR = sumR / count
				avgG = sumG / count
				avgB = sumB / count
			}

			// 輝度
			L := 0.299*avgR + 0.587*avgG + 0.114*avgB

			// クロミナンス C = RGB標準偏差近似 (skolchin/gbr 変種)
			avgMax := math.Max(avgR, math.Max(avgG, avgB))
			avgMin := math.Min(avgR, math.Min(avgG, avgB))
			C := avgMax - avgMin

			// 不均一性 D: ドーナツ内Lの標準偏差
			D := 0.0
			if len(lVals) > 1 {
				meanL := L
				sumSq := 0.0
				for _, lv := range lVals {
					d := lv - meanL
					sumSq += d * d
				}
				D = math.Sqrt(sumSq / float64(len(lVals)))
			}

			// 色相 H (0-360)
			H := math.Atan2(math.Sqrt(3)*(avgG-avgB), 2*avgR-avgG-avgB) * 180.0 / math.Pi
			if H < 0 {
				H += 360.0
			}

			// 彩度 S (最大最小差近似)
			S := avgMax - avgMin

			// YCrCb
			_, Cr, Cb := RGBToYCrCb(avgR, avgG, avgB)
			chroma := ComputeChroma(Cr, Cb)

			// 木目度 RMB
			RMB := avgR - avgB

			idx := y*BoardSize + x
			data.Features[idx] = IntersectionFeatures{
				X:      x,
				Y:      y,
				L:      L,
				C:      C,
				D:      D,
				H:      H,
				S:      S,
				R:      avgR,
				G:      avgG,
				B:      avgB,
				RMB:    RMB,
				Cr:     Cr,
				Cb:     Cb,
				Chroma: chroma,
			}
		}
	}

	return data
}
