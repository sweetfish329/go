package vision

import (
	"fmt"
	"image"
	"math"
	"sort"
)

// DirectGrid1LineResult は1線直接検出の結果
type DirectGrid1LineResult struct {
	Corners      [4]Point // [TL, TR, BR, BL]
	LineTop      LineHV
	LineRight    LineHV
	LineBot      LineHV
	LineLeft     LineHV
	OkTop        bool
	OkRight      bool
	OkBot        bool
	OkLeft       bool
	NumLines     int
	RejectReason string
	Success      bool
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

	// 各辺の長さから透視投影に応じた1マスの推定ピッチを個別に計算
	pitchTop := math.Hypot(rawOuter[1].X-rawOuter[0].X, rawOuter[1].Y-rawOuter[0].Y) / 18.0
	pitchBot := math.Hypot(rawOuter[2].X-rawOuter[3].X, rawOuter[2].Y-rawOuter[3].Y) / 18.0
	pitchLeft := math.Hypot(rawOuter[3].X-rawOuter[0].X, rawOuter[3].Y-rawOuter[0].Y) / 18.0
	pitchRight := math.Hypot(rawOuter[2].X-rawOuter[1].X, rawOuter[2].Y-rawOuter[1].Y) / 18.0

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

		minScanIn := math.Max(16.0, pitch*0.35)
		maxScanIn := math.Max(50.0, pitch*1.15)
		minDrop := 7.0
		minSideDrop := 2.0
		maxVal := 120.0

		switch edgeType {
		case "top":
			minScanIn = math.Max(24.0, pitch*0.48)
			maxScanIn = math.Max(50.0, pitch*1.15)
			minDrop = 5.5
			minSideDrop = 1.5
			maxVal = 125.0
		case "left":
			minScanIn = math.Max(16.0, pitch*0.35)
			maxScanIn = math.Max(50.0, pitch*1.15)
			minDrop = 7.0
			minSideDrop = 2.0
			maxVal = 110.0 // 木目シミや節の除外
		case "bot":
			minScanIn = math.Max(16.0, pitch*0.25)
			maxScanIn = math.Max(65.0, pitch*1.15)
			minDrop = 7.0
			minSideDrop = 2.0
			maxVal = 125.0
		case "right":
			minScanIn = math.Max(22.0, pitch*0.45)
			maxScanIn = math.Max(55.0, pitch*1.15)
			minDrop = 7.0
			minSideDrop = 2.5
			maxVal = 120.0
		}

		step := math.Max(5.0, math.Min(12.0, pitch*0.14))

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
			for d := edgeDist + minScanIn; d <= edgeDist+maxScanIn; d += 1.0 {
				px := int(baseX + nx*d)
				py := int(baseY + ny*d)
				val := getGray(px, py)
				if val > maxVal {
					continue
				}

				bgL := getGray(int(baseX+nx*(d-step)), int(baseY+ny*(d-step)))
				bgR := getGray(int(baseX+nx*(d+step)), int(baseY+ny*(d+step)))
				base := (bgL + bgR) / 2.0
				drop := base - val
				// 真の谷判定: 片側だけの段差（影や面取り）を排除し、両側ともに暗い溝（格子線）のみを検出
				if drop >= minDrop && bgL-val >= minSideDrop && bgR-val >= minSideDrop {
					// 幅広の暗部（黒石）判定: 格子線の幅は通常 1〜3px なので、離れた位置も暗い場合は黒石としてスキップ
					valFar := getGray(int(baseX+nx*(d+step*1.5)), int(baseY+ny*(d+step*1.5)))
					if valFar < val+8.0 && val < 60.0 {
						continue
					}

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
	ptsTop := sample1LinePoints(rawOuter[0], rawOuter[1], "top", pitchTop)
	ptsRight := sample1LinePoints(rawOuter[1], rawOuter[2], "right", pitchRight)
	ptsBot := sample1LinePoints(rawOuter[3], rawOuter[2], "bot", pitchBot)
	ptsLeft := sample1LinePoints(rawOuter[0], rawOuter[3], "left", pitchLeft)

	// 2. 4直線の方程式をフィッティング
	// まず手前で鮮明な底辺直線 (cBot, dBot) をフィッティング
	cBot, dBot, okBot := fitHorizLineRANSAC(ptsBot)
	aRight, bRight, okRight := fitVertLineRANSAC(ptsRight)
	aLeft, bLeft, okLeft := fitVertLineRANSAC(ptsLeft)

	// 上辺直線 (cTop, dTop): 奥の上辺は透視投影の消失点に向かって水平に収束するため、
	// 手前底辺 cBot と同符号で、かつより水平に近い傾きを緩やかに拘束
	fitTopLineRANSAC := func(pts []Point, cBotRef float64, okBotRef bool) (float64, float64, bool) {
		n := len(pts)
		if n < 5 {
			return 0, 0, false
		}
		var bestInliers []Point
		bestC, bestD := 0.0, 0.0

		isPerspectiveConsistent := func(c float64) bool {
			if !okBotRef {
				return math.Abs(c) <= 0.25
			}
			// カメラの傾き（ロール角）や遠近法による消失点収束を考慮
			if math.Abs(c-cBotRef) <= 0.08 {
				return math.Abs(c) <= 0.25
			}
			if cBotRef > 0 {
				return c >= -0.05 && c <= math.Max(cBotRef*1.25, cBotRef+0.05) && math.Abs(c) <= 0.25
			} else {
				return c <= 0.05 && c >= math.Min(cBotRef*1.25, cBotRef-0.05) && math.Abs(c) <= 0.25
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

	// 3線確定時の幾何補完 (Tri-Line Geometric Completion):
	// 4辺のうち3辺が高精度に検出され、1辺のみが碁石密集や影で欠損した場合、
	// 消失点 (Vanishing Point) と対向辺・木枠境界から欠損した1直線を数学的に厳密外挿復元する。
	numSuccess := 0
	if okTop {
		numSuccess++
	}
	if okRight {
		numSuccess++
	}
	if okBot {
		numSuccess++
	}
	if okLeft {
		numSuccess++
	}

	if numSuccess == 3 {
		// ケース1: 上辺のみ欠損 (!okTop && okBot && okLeft && okRight)
		if !okTop && okBot && okLeft && okRight {
			denomV := aLeft - aRight
			if math.Abs(denomV) > 1e-4 {
				yV := (bRight - bLeft) / denomV
				yRawBot := (rawOuter[3].Y + rawOuter[2].Y) * 0.5
				yRawTop := (rawOuter[0].Y + rawOuter[1].Y) * 0.5
				y1LineBot := cBot*cx + dBot
				deltaBot := math.Abs(yRawBot - y1LineBot)

				distBotV := math.Abs(yRawBot - yV)
				distTopV := math.Abs(yRawTop - yV)
				if distBotV > 10.0 {
					scale := distTopV / distBotV
					deltaTop := math.Max(8.0, deltaBot*scale)
					cTop = cBot * scale
					dTop = (yRawTop + deltaTop) - cTop*cx
					okTop = true
				}
			}
		} else if !okBot && okTop && okLeft && okRight {
			// ケース2: 底辺のみ欠損 (!okBot && okTop && okLeft && okRight)
			denomV := aLeft - aRight
			if math.Abs(denomV) > 1e-4 {
				yV := (bRight - bLeft) / denomV
				yRawBot := (rawOuter[3].Y + rawOuter[2].Y) * 0.5
				yRawTop := (rawOuter[0].Y + rawOuter[1].Y) * 0.5
				y1LineTop := cTop*cx + dTop
				deltaTop := math.Abs(y1LineTop - yRawTop)

				distBotV := math.Abs(yRawBot - yV)
				distTopV := math.Abs(yRawTop - yV)
				if distTopV > 10.0 {
					scale := distBotV / distTopV
					deltaBot := math.Max(8.0, deltaTop*scale)
					cBot = cTop * scale
					dBot = (yRawBot - deltaBot) - cBot*cx
					okBot = true
				}
			}
		} else if !okRight && okLeft && okTop && okBot {
			// ケース3: 右辺のみ欠損 (!okRight && okLeft && okTop && okBot)
			denomH := cTop - cBot
			if math.Abs(denomH) > 1e-4 {
				xH := (dBot - dTop) / denomH
				xRawLeft := (rawOuter[0].X + rawOuter[3].X) * 0.5
				xRawRight := (rawOuter[1].X + rawOuter[2].X) * 0.5
				x1LineLeft := aLeft*cy + bLeft
				deltaLeft := math.Abs(x1LineLeft - xRawLeft)

				distLeftH := math.Abs(xRawLeft - xH)
				distRightH := math.Abs(xRawRight - xH)
				if distLeftH > 10.0 {
					scale := distRightH / distLeftH
					deltaRight := math.Max(8.0, deltaLeft*scale)
					aRight = aLeft * scale
					bRight = (xRawRight - deltaRight) - aRight*cy
					okRight = true
				}
			}
		} else if !okLeft && okRight && okTop && okBot {
			// ケース4: 左辺のみ欠損 (!okLeft && okRight && okTop && okBot)
			denomH := cTop - cBot
			if math.Abs(denomH) > 1e-4 {
				xH := (dBot - dTop) / denomH
				xRawLeft := (rawOuter[0].X + rawOuter[3].X) * 0.5
				xRawRight := (rawOuter[1].X + rawOuter[2].X) * 0.5
				x1LineRight := aRight*cy + bRight
				deltaRight := math.Abs(xRawRight - x1LineRight)

				distLeftH := math.Abs(xRawLeft - xH)
				distRightH := math.Abs(xRawRight - xH)
				if distRightH > 10.0 {
					scale := distLeftH / distRightH
					deltaLeft := math.Max(8.0, deltaRight*scale)
					aLeft = aRight * scale
					bLeft = (xRawLeft + deltaLeft) - aLeft*cy
					okLeft = true
				}
			}
		}
	}

	rejectReason := ""

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
		if areaRaw <= 100 || area1Line <= 100 {
			rejectReason = fmt.Sprintf("area too small: raw=%.1f, 1line=%.1f", areaRaw, area1Line)
		} else {
			ratio := area1Line / areaRaw
			if ratio < 0.70 || ratio > 0.985 {
				rejectReason = fmt.Sprintf("ratio out of range: %.3f", ratio)
			} else {
				// 各頂点が画像境界付近に収まっているか検証 (画面端ギリギリでの撮影を許容)
				valid := true
				for idx, c := range corners {
					if c.X < -25 || c.X > float64(w+25) || c.Y < -25 || c.Y > float64(h+25) {
						valid = false
						rejectReason = fmt.Sprintf("corner %d out of bounds: (%.1f, %.1f)", idx, c.X, c.Y)
						break
					}
				}
				if valid {
					for i := 0; i < 4; i++ {
						distRaw := math.Hypot(rawOuter[i].X-cx, rawOuter[i].Y-cy)
						distDG := math.Hypot(corners[i].X-cx, corners[i].Y-cy)
						if distDG > distRaw+10.0 {
							valid = false
							rejectReason = fmt.Sprintf("corner %d distDG (%.1f) > distRaw (%.1f)", i, distDG, distRaw)
							break
						}
					}
				}
				if valid {
					return &DirectGrid1LineResult{
						Corners:   corners,
						LineTop:   LineHV{Slope: cTop, Intercept: dTop, IsHorizontal: true},
						LineRight: LineHV{Slope: aRight, Intercept: bRight, IsHorizontal: false},
						LineBot:   LineHV{Slope: cBot, Intercept: dBot, IsHorizontal: true},
						LineLeft:  LineHV{Slope: aLeft, Intercept: bLeft, IsHorizontal: false},
						OkTop:     okTop,
						OkRight:   okRight,
						OkBot:     okBot,
						OkLeft:    okLeft,
						NumLines:  numSuccess,
						Success:   true,
					}, nil
				}
			}
		}
	}

	// 一部の辺でサンプル不足の場合はフォールバックフラグを返す
	return &DirectGrid1LineResult{
		OkTop:        okTop,
		OkRight:      okRight,
		OkBot:        okBot,
		OkLeft:       okLeft,
		NumLines:     numSuccess,
		RejectReason: rejectReason,
		Success:      false,
	}, nil
}
