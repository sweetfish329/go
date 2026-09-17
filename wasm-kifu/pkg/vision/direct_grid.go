package vision

import (
	"image"
	"math"
	"sort"
)

// DirectGrid1LineResult は1線直接検出の結果
type DirectGrid1LineResult struct {
	Corners   [4]Point // [TL, TR, BR, BL]
	LineTop   LineHV
	LineRight LineHV
	LineBot   LineHV
	LineLeft  LineHV
	Success   bool
}

// Detect1LinesDirectly は天板ブロブ内側から最外周格子線（1線: 上下左右4本）を直接検出し、
// その幾何交点から真の格子四隅 [TL, TR, BR, BL] を算出する。
func Detect1LinesDirectly(img image.Image, blob *BoardBlobResult, rawOuter [4]Point) (*DirectGrid1LineResult, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

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

	// 盤面幅と高さから1マスの推定ピッチを計算
	boardW := math.Hypot(rawOuter[1].X-rawOuter[0].X, rawOuter[1].Y-rawOuter[0].Y)
	boardH := math.Hypot(rawOuter[3].X-rawOuter[0].X, rawOuter[3].Y-rawOuter[0].Y)
	pitchW := boardW / 18.0
	pitchH := boardH / 18.0

	// 各辺の木枠境界から内側への法線ベクトル
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
		if nx*(cx-mx)+ny*(cy-my) < 0 {
			nx = -nx
			ny = -ny
		}
		return nx, ny
	}

	// 1辺に沿って最外周格子線（1線）の点群を走査・サンプリング
	sample1LinePoints := func(p1, p2 Point, edgeType string, pitch float64) []Point {
		nx, ny := getNormalTowardsCenter(p1, p2)
		if nx == 0 && ny == 0 {
			return nil
		}

		var pts []Point
		numSamples := 35

		maxScanIn := math.Max(60.0, math.Min(220.0, pitch*2.5))
		startScan := 18.0
		minDrop := 16.0
		maxVal := 105.0

		switch edgeType {
		case "top":
			startScan = math.Max(70.0, pitch*0.9)
			maxScanIn = math.Max(90.0, pitch*1.6)
			minDrop = 8.0
		case "left":
			startScan = 15.0
			maxVal = 95.0 // 木目シミや節の除外
		case "bot":
			startScan = 12.0
		case "right":
			startScan = 18.0
		}

		startOffset := -math.Max(15.0, pitch*0.45)
		endOffset := maxScanIn

		for i := 2; i <= numSamples-2; i++ {
			t := float64(i) / float64(numSamples-1)
			baseX := p1.X*(1-t) + p2.X*t
			baseY := p1.Y*(1-t) + p2.Y*t

			// 1. 木枠エッジ (外側から入って木目 R-B >= 20 が安定する地点)
			edgeDist := -999.0
			for d := startOffset; d <= endOffset-15.0; d += 1.0 {
				px := int(baseX + nx*d)
				py := int(baseY + ny*d)
				r, g, b := getRGB(px, py)
				if r-b >= 20 && r >= 60 && r > g && g > b {
					// 連続木目確認
					r1, _, b1 := getRGB(int(baseX+nx*(d+2)), int(baseY+ny*(d+2)))
					r2, _, b2 := getRGB(int(baseX+nx*(d+4)), int(baseY+ny*(d+4)))
					if r1-b1 >= 20 && r2-b2 >= 20 {
						edgeDist = d
						break
					}
				}
			}
			if edgeDist == -999.0 {
				continue
			}

			// 2. 木枠エッジから内側へ最初に出現する漆黒線 (first significant dark valley) を探索
			bestDist := -1.0
			for d := edgeDist + startScan; d <= edgeDist+maxScanIn; d += 1.0 {
				px := int(baseX + nx*d)
				py := int(baseY + ny*d)
				val := getGray(px, py)
				if val > maxVal {
					continue
				}

				bgL := getGray(int(baseX+nx*(d-6)), int(baseY+ny*(d-6)))
				bgR := getGray(int(baseX+nx*(d+6)), int(baseY+ny*(d+6)))
				base := (bgL + bgR) / 2.0
				drop := base - val
				if drop >= minDrop {
					// 局所最小点（谷の底）までアライメント
					for dNext := d + 1.0; dNext <= d+6.0; dNext += 1.0 {
						valNext := getGray(int(baseX+nx*dNext), int(baseY+ny*dNext))
						if valNext < val {
							val = valNext
							d = dNext
						}
					}
					bestDist = d
					break // 最初に出現した漆黒線 = 1線！2線へは行かない！
				}
			}

			if bestDist != -1.0 {
				pts = append(pts, Point{
					X: baseX + nx*bestDist,
					Y: baseY + ny*bestDist,
				})
			}
		}
		return pts
	}

	// RANSAC による垂直直線フィッティング x = a * y + b
	fitVertLineRANSAC := func(pts []Point) (float64, float64, bool) {
		n := len(pts)
		if n < 5 {
			return 0, 0, false
		}
		var bestInliers []Point
		bestA, bestB := 0.0, 0.0
		for iter := 0; iter < 120; iter++ {
			i1 := (iter * 17) % n
			i2 := (iter*31 + 13) % n
			if i1 == i2 || math.Abs(pts[i1].Y-pts[i2].Y) < 40.0 {
				continue
			}
			p1, p2 := pts[i1], pts[i2]
			a := (p2.X - p1.X) / (p2.Y - p1.Y)
			b := p1.X - a*p1.Y
			var inliers []Point
			for _, p := range pts {
				if math.Abs(p.X-(a*p.Y+b)) < 10.0 {
					inliers = append(inliers, p)
				}
			}
			if len(inliers) > len(bestInliers) {
				bestInliers = inliers
				bestA, bestB = a, b
			}
		}
		if len(bestInliers) < 5 {
			return 0, 0, false
		}
		meanX, meanY := 0.0, 0.0
		for _, p := range bestInliers {
			meanX += p.X
			meanY += p.Y
		}
		meanX /= float64(len(bestInliers))
		meanY /= float64(len(bestInliers))
		var num, den float64
		for _, p := range bestInliers {
			num += (p.Y - meanY) * (p.X - meanX)
			den += (p.Y - meanY) * (p.Y - meanY)
		}
		if den > 1e-6 {
			bestA = num / den
		}
		bestB = meanX - bestA*meanY
		return bestA, bestB, true
	}

	// RANSAC による水平直線フィッティング y = c * x + d
	fitHorizLineRANSAC := func(pts []Point) (float64, float64, bool) {
		n := len(pts)
		if n < 5 {
			return 0, 0, false
		}
		var bestInliers []Point
		bestC, bestD := 0.0, 0.0
		for iter := 0; iter < 120; iter++ {
			i1 := (iter * 19) % n
			i2 := (iter*37 + 11) % n
			if i1 == i2 || math.Abs(pts[i1].X-pts[i2].X) < 40.0 {
				continue
			}
			p1, p2 := pts[i1], pts[i2]
			c := (p2.Y - p1.Y) / (p2.X - p1.X)
			d := p1.Y - c*p1.X
			var inliers []Point
			for _, p := range pts {
				if math.Abs(p.Y-(c*p.X+d)) < 10.0 {
					inliers = append(inliers, p)
				}
			}
			if len(inliers) > len(bestInliers) {
				bestInliers = inliers
				bestC, bestD = c, d
			}
		}
		if len(bestInliers) < 5 {
			return 0, 0, false
		}
		meanX, meanY := 0.0, 0.0
		for _, p := range bestInliers {
			meanX += p.X
			meanY += p.Y
		}
		meanX /= float64(len(bestInliers))
		meanY /= float64(len(bestInliers))
		var num, den float64
		for _, p := range bestInliers {
			num += (p.X - meanX) * (p.Y - meanY)
			den += (p.X - meanX) * (p.X - meanX)
		}
		if den > 1e-6 {
			bestC = num / den
		}
		bestD = meanY - bestC*meanX
		return bestC, bestD, true
	}

	// 1. 各辺の1線点群をサンプリング
	ptsTop := sample1LinePoints(rawOuter[0], rawOuter[1], "top", pitchH)
	ptsRight := sample1LinePoints(rawOuter[1], rawOuter[2], "right", pitchW)
	ptsBot := sample1LinePoints(rawOuter[3], rawOuter[2], "bot", pitchH)
	ptsLeft := sample1LinePoints(rawOuter[0], rawOuter[3], "left", pitchW)

	// 2. 4直線の方程式をフィッティング
	// まず手前で鮮明な底辺直線 (cBot, dBot) をフィッティング
	cBot, dBot, okBot := fitHorizLineRANSAC(ptsBot)
	aRight, bRight, okRight := fitVertLineRANSAC(ptsRight)
	aLeft, bLeft, okLeft := fitVertLineRANSAC(ptsLeft)

	// 上辺直線 (cTop, dTop): 奥の上辺は透視投影の消失点に向かって水平に収束するため、
	// 手前底辺 cBot と同符号で、かつより水平に近い傾き (0 <= cTop/cBot <= 0.85) を厳密に拘束
	fitTopLineRANSAC := func(pts []Point, cBotRef float64, okBotRef bool) (float64, float64, bool) {
		n := len(pts)
		if n < 5 {
			return 0, 0, false
		}
		var bestInliers []Point
		bestC, bestD := 0.0, 0.0

		isPerspectiveConsistent := func(c float64) bool {
			if !okBotRef {
				return math.Abs(c) <= 0.20
			}
			if math.Abs(cBotRef) < 0.03 {
				return math.Abs(c) <= 0.06
			} else if cBotRef > 0 {
				return c >= cBotRef*0.05 && c <= cBotRef*0.60 && math.Abs(c) <= 0.13
			} else {
				return c <= cBotRef*0.05 && c >= cBotRef*0.60 && math.Abs(c) <= 0.13
			}
		}

		for iter := 0; iter < 120; iter++ {
			i1 := (iter * 19) % n
			i2 := (iter*37 + 11) % n
			if i1 == i2 || math.Abs(pts[i1].X-pts[i2].X) < 40.0 {
				continue
			}
			p1, p2 := pts[i1], pts[i2]
			c := (p2.Y - p1.Y) / (p2.X - p1.X)
			if !isPerspectiveConsistent(c) {
				continue
			}
			d := p1.Y - c*p1.X

			var inliers []Point
			for _, p := range pts {
				if math.Abs(p.Y-(c*p.X+d)) < 12.0 {
					inliers = append(inliers, p)
				}
			}
			if len(inliers) > len(bestInliers) {
				bestInliers = inliers
				bestC, bestD = c, d
			}
		}

		if len(bestInliers) >= 5 {
			meanX, meanY := 0.0, 0.0
			for _, p := range bestInliers {
				meanX += p.X
				meanY += p.Y
			}
			meanX /= float64(len(bestInliers))
			meanY /= float64(len(bestInliers))
			var num, den float64
			for _, p := range bestInliers {
				num += (p.X - meanX) * (p.Y - meanY)
				den += (p.X - meanX) * (p.X - meanX)
			}
			if den > 1e-6 {
				cCandidate := num / den
				if isPerspectiveConsistent(cCandidate) {
					bestC = cCandidate
					bestD = meanY - bestC*meanX
					return bestC, bestD, true
				}
			}
			return bestC, bestD, true
		}

		// 透視整合インライアが不足する場合は、透視収束傾き (cBot * 0.35) を傾きとし、点群中央値から切片を決定
		if okBotRef && len(pts) >= 4 {
			cFixed := cBotRef * 0.35
			var ds []float64
			for _, p := range pts {
				ds = append(ds, p.Y-cFixed*p.X)
			}
			sort.Float64s(ds)
			dMed := ds[len(ds)/2]
			return cFixed, dMed, true
		}

		return 0, 0, false
	}

	cTop, dTop, okTop := fitTopLineRANSAC(ptsTop, cBot, okBot)

	// すべての辺で直接フィッティングが成功した場合、4直線の交点から四隅を決定
	if okTop && okRight && okBot && okLeft {
		tl := IntersectSlopeLines(cTop, dTop, aLeft, bLeft)
		tr := IntersectSlopeLines(cTop, dTop, aRight, bRight)
		br := IntersectSlopeLines(cBot, dBot, aRight, bRight)
		bl := IntersectSlopeLines(cBot, dBot, aLeft, bLeft)

		corners := OrderCorners([4]Point{tl, tr, br, bl})
		area1Line := PolygonArea([]Point{corners[0], corners[1], corners[2], corners[3]})
		areaRaw := PolygonArea([]Point{rawOuter[0], rawOuter[1], rawOuter[2], rawOuter[3]})

		// 1線領域は木枠外形より必ず内側にあり、面積比は 80% 〜 98.5% の範囲に収まる
		// (木枠エッジそのものを拾ってしまった場合や過大発散した場合は安全にフォールバック)
		if areaRaw > 100 && area1Line > 100 {
			ratio := area1Line / areaRaw
			if ratio >= 0.70 && ratio <= 0.985 {
				// 各頂点が画像内かつ木枠外形の内側（中心側）に収まっているか検証
				valid := true
				for _, c := range corners {
					if c.X < 5 || c.X > float64(w-5) || c.Y < 5 || c.Y > float64(h-5) {
						valid = false
						break
					}
				}
				for i := 0; i < 4; i++ {
					distRaw := math.Hypot(rawOuter[i].X-cx, rawOuter[i].Y-cy)
					distDG := math.Hypot(corners[i].X-cx, corners[i].Y-cy)
					if distDG > distRaw+5.0 {
						valid = false
						break
					}
				}
				if valid {
					return &DirectGrid1LineResult{
						Corners:   corners,
						LineTop:   LineHV{Slope: cTop, Intercept: dTop, IsHorizontal: true},
						LineRight: LineHV{Slope: aRight, Intercept: bRight, IsHorizontal: false},
						LineBot:   LineHV{Slope: cBot, Intercept: dBot, IsHorizontal: true},
						LineLeft:  LineHV{Slope: aLeft, Intercept: bLeft, IsHorizontal: false},
						Success:   true,
					}, nil
				}
			}
		}
	}

	// 一部の辺でサンプル不足の場合はフォールバックフラグを返す
	return &DirectGrid1LineResult{
		Success: false,
	}, nil
}
