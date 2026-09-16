package vision

import (
	"image"
	"math"
	"sort"
)

// MeasureDynamicWoodMargins は木枠4辺の外縁から内側の最外周格子線（1線）までの実測ピクセル距離を動的に測定する。
// 返却値: [Top, Right, Bottom, Left] の各辺マージン（ピクセル単位）
func MeasureDynamicWoodMargins(img image.Image, rawOuter [4]Point) [4]float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	getRGB := func(x, y int) (float64, float64, float64) {
		if x < 0 || x >= w || y < 0 || y >= h {
			return 0, 0, 0
		}
		r, g, b, _ := img.At(x, y).RGBA()
		return float64(r >> 8), float64(g >> 8), float64(b >> 8)
	}
	getGray := func(x, y int) float64 {
		r, g, b := getRGB(x, y)
		return 0.299*r + 0.587*g + 0.114*b
	}

	// 盤面中心
	cx := (rawOuter[0].X + rawOuter[1].X + rawOuter[2].X + rawOuter[3].X) / 4.0
	cy := (rawOuter[0].Y + rawOuter[1].Y + rawOuter[2].Y + rawOuter[3].Y) / 4.0

	// 盤面の概略スケール（幅・高さ）
	boardW := math.Hypot(rawOuter[1].X-rawOuter[0].X, rawOuter[1].Y-rawOuter[0].Y)
	boardH := math.Hypot(rawOuter[3].X-rawOuter[0].X, rawOuter[3].Y-rawOuter[0].Y)
	diag := math.Hypot(boardW, boardH)

	// 最大探索深度（盤サイズに応じた安全値: 約 7%）
	maxScanDist := math.Max(40.0, math.Min(120.0, diag*0.065))

	// 各辺の法線ベクトル（中心方向）
	getNormalTowardsCenter := func(p1, p2 Point) (float64, float64) {
		dx := p2.X - p1.X
		dy := p2.Y - p1.Y
		nx := -dy
		ny := dx
		l := math.Hypot(nx, ny)
		if l < 1e-6 {
			return 0, 0
		}
		nx /= l
		ny /= l

		mx := (p1.X + p2.X) * 0.5
		my := (p1.Y + p2.Y) * 0.5

		// 中心に向いているか確認
		if nx*(cx-mx)+ny*(cy-my) < 0 {
			nx = -nx
			ny = -ny
		}
		return nx, ny
	}

	// 1D メディアンフィルタ (木目高周波ノイズ除去)
	medianFilter1D := func(data []float64, window int) []float64 {
		n := len(data)
		res := make([]float64, n)
		r := window / 2
		for i := 0; i < n; i++ {
			var vals []float64
			for j := i - r; j <= i+r; j++ {
				if j >= 0 && j < n {
					vals = append(vals, data[j])
				}
			}
			sort.Float64s(vals)
			res[i] = vals[len(vals)/2]
		}
		return res
	}

	// Huber 損失 M-推定 (IRLS) によるロバスト平均マージン決定
	huberMEstimate := func(samples []float64) float64 {
		if len(samples) == 0 {
			return 0
		}
		sort.Float64s(samples)
		mu := samples[len(samples)/2] // 初期値: メディアン
		delta := 6.0                  // 閾値: 6px

		for iter := 0; iter < 10; iter++ {
			var sumW, sumWX float64
			for _, x := range samples {
				r := math.Abs(x - mu)
				w := 1.0
				if r > delta {
					w = delta / r
				}
				sumW += w
				sumWX += w * x
			}
			if sumW < 1e-6 {
				break
			}
			newMu := sumWX / sumW
			if math.Abs(newMu-mu) < 0.05 {
				break
			}
			mu = newMu
		}
		return mu
	}

	measureEdge := func(p1, p2 Point, defaultMargin float64) float64 {
		nx, ny := getNormalTowardsCenter(p1, p2)
		if nx == 0 && ny == 0 {
			return defaultMargin
		}

		var measured []float64
		numSamples := 21 // 21箇所で高密度走査

		// 1マスの推定ピッチ (18路で18等分)
		gridPitch := math.Hypot(p2.X-p1.X, p2.Y-p1.Y) / 18.0
		// 木枠マージンは物理的に1マス間隔未満 (最大でも 0.80 * pitch)
		maxScan := math.Min(maxScanDist, gridPitch*0.80)

		for i := 2; i <= numSamples-2; i++ {
			t := float64(i) / float64(numSamples-1)
			startX := p1.X*(1-t) + p2.X*t
			startY := p1.Y*(1-t) + p2.Y*t

			// 1. Circle-First / Blob Masking: 始点近傍の木目性チェック
			r0, g0, b0 := getRGB(int(startX+nx*5.0), int(startY+ny*5.0))
			gray0 := 0.299*r0 + 0.587*g0 + 0.114*b0
			// 木目色でない（白石、黒石、暗い影）場合は碁石遮蔽としてスキップ
			if r0 < b0+12.0 || gray0 < 70.0 || gray0 > 205.0 {
				continue
			}

			// 2. 1D プロファイル走査 (d = 4px 〜 maxScan)
			var dists []float64
			var grays []float64
			for d := 4.0; d <= maxScan; d += 1.0 {
				sx := int(startX + nx*d)
				sy := int(startY + ny*d)
				dists = append(dists, d)
				grays = append(grays, getGray(sx, sy))
			}

			if len(grays) < 10 {
				continue
			}

			// 3. 1D メディアン平滑化で木目テクスチャの微小凹凸を除去
			smoothed := medianFilter1D(grays, 5)

			// 4. エッジエンベロープ（谷の局所極小 ＆ 2階差分曲率）
			minVal := 1e9
			bestDist := -1.0
			for j := 2; j < len(smoothed)-2; j++ {
				d := dists[j]
				val := smoothed[j]
				// 谷の曲率 (2階差分: 周囲より深く鋭く落ち込んでいること)
				curv := (smoothed[j-2] + smoothed[j+2]) - 2*val
				if val < minVal && curv > 7.0 {
					minVal = val
					bestDist = d
				}
			}

			// コントラストチェック（木目背景より明らかに暗く、物理的余白の範囲内）
			if bestDist >= 10.0 && bestDist <= 85.0 && (gray0-minVal >= 12.0) {
				measured = append(measured, bestDist)
			}
		}

		if len(measured) >= 3 {
			return huberMEstimate(measured)
		}

		return defaultMargin
	}

	defaultMarginHoriz := boardW * 0.028
	defaultMarginVert := boardH * 0.025

	marginTop := measureEdge(rawOuter[0], rawOuter[1], defaultMarginVert)
	marginRight := measureEdge(rawOuter[1], rawOuter[2], defaultMarginHoriz)
	marginBot := measureEdge(rawOuter[3], rawOuter[2], defaultMarginVert)
	marginLeft := measureEdge(rawOuter[0], rawOuter[3], defaultMarginHoriz)

	return [4]float64{marginTop, marginRight, marginBot, marginLeft}
}

// ApplyDynamicMarginShift は実測されたマージン量 [Top, Right, Bottom, Left] に基づき、
// 木枠4辺を内側へ幾何学的に平行移動させて真の19路格子四隅交点を導出する。
func ApplyDynamicMarginShift(rawOuter [4]Point, margins [4]float64) [4]Point {
	tl, tr, br, bl := rawOuter[0], rawOuter[1], rawOuter[2], rawOuter[3]
	mTop, mRight, mBot, mLeft := margins[0], margins[1], margins[2], margins[3]

	// 盤面中心
	cx := (tl.X + tr.X + br.X + bl.X) / 4.0
	cy := (tl.Y + tr.Y + br.Y + bl.Y) / 4.0

	// 直線の方程式 A*x + B*y + C = 0 (A^2 + B^2 = 1 に正規化)
	// 点 p1 -> p2 を通る直線: dy*x - dx*y + (dx*y1 - dy*x1) = 0
	makeLine := func(p1, p2 Point, offset float64) (float64, float64, float64) {
		dx := p2.X - p1.X
		dy := p2.Y - p1.Y
		a := dy
		b := -dx
		l := math.Hypot(a, b)
		if l < 1e-6 {
			return 1, 0, -p1.X
		}
		a /= l
		b /= l
		c := -(a*p1.X + b*p1.Y)

		// 中心方向へのシフト: (cx, cy) を代入したときの符号を確認
		distCenter := a*cx + b*cy + c
		if distCenter > 0 {
			// 中心方向は + 側
			c -= offset
		} else {
			// 中心方向は - 側
			c += offset
		}
		return a, b, c
	}

	// 4辺の平行移動直線
	// 上辺 (TL -> TR)
	aTop, bTop, cTop := makeLine(tl, tr, mTop)
	// 右辺 (TR -> BR)
	aRight, bRight, cRight := makeLine(tr, br, mRight)
	// 底辺 (BR -> BL)
	aBot, bBot, cBot := makeLine(br, bl, mBot)
	// 左辺 (BL -> TL)
	aLeft, bLeft, cLeft := makeLine(bl, tl, mLeft)

	// 2直線 A1*x + B1*y + C1 = 0 と A2*x + B2*y + C2 = 0 の交点
	intersectLines := func(a1, b1, c1, a2, b2, c2 float64, fallback Point) Point {
		det := a1*b2 - a2*b1
		if math.Abs(det) < 1e-6 {
			return fallback
		}
		x := (b1*c2 - b2*c1) / det
		y := (a2*c1 - a1*c2) / det
		return Point{X: x, Y: y}
	}

	resTL := intersectLines(aTop, bTop, cTop, aLeft, bLeft, cLeft, tl)
	resTR := intersectLines(aTop, bTop, cTop, aRight, bRight, cRight, tr)
	resBR := intersectLines(aBot, bBot, cBot, aRight, bRight, cRight, br)
	resBL := intersectLines(aBot, bBot, cBot, aLeft, bLeft, cLeft, bl)

	return [4]Point{resTL, resTR, resBR, resBL}
}
