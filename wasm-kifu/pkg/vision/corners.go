// Package vision は碁盤画像の多次元特徴量解析および幾何認識ロジックを提供する。
package vision

import (
	"fmt"
	"image"
	"math"
	"sort"
)

// DetectBoardCorners は碁盤天板ブロブ検出、木枠境界フィッティング、
// 最外周格子線（1線）直接検出、およびFörstnerサブピクセル吸着を順次実行し、
// 碁盤の19路格子4隅 [TL, TR, BR, BL] を高精度・ロバストに自動検出する。
func DetectBoardCorners(img image.Image) (*CornerDetectionResult, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 100 || h < 100 {
		return nil, fmt.Errorf("image too small: %dx%d", w, h)
	}

	// 1. 天板木目ブロブの検出 (モルフォロジー閉・開処理 ＆ 最大連結成分抽出)
	blob, err := DetectBoardBlob(img)
	if err != nil {
		return nil, fmt.Errorf("board blob detection failed: %w", err)
	}

	// 2. ブロブ境界点群から木枠天板の外縁四隅 (RawOuter) を推定
	rawOuter := fitOuterBoundaryCorners(img, blob, w, h)

	// 3. 最外周格子線 (1線: 上下左右4本) の直接検出
	// 木枠からの間接マージンシフトではなく、天板上の黒い1線そのものを直接サンプリング
	directGrid, _ := Detect1LinesDirectly(img, blob, rawOuter)
	var gridCorners [4]Point

	method := "DirectGrid"
	if directGrid != nil && directGrid.Success {
		gridCorners = directGrid.Corners
	} else {
		method = "MarginInset"
		// フォールバック: 木枠マージンインセット逆算 (日本・韓国標準規格: 横 3.2%, 縦 2.5%)
		gridCorners = ApplyMarginInset(rawOuter, 0.032, 0.025)
	}

	// 4. 星（花点・天元）によるホモグラフィ精密化 (DirectGrid / MarginInset の成否に関わらず適用)
	// 上下両側に星がバランスよく存在する場合（上下各行に1個以上、計4個以上）のみ適用
	starRefiner := NewStarHomographyRefiner()
	stars := starRefiner.DetectStarPoints(img, gridCorners)
	hasTopStar := false
	hasBotStar := false
	for _, s := range stars {
		if s.GridY <= 3 {
			hasTopStar = true
		}
		if s.GridY >= 15 {
			hasBotStar = true
		}
	}
	if len(stars) >= 4 && hasTopStar && hasBotStar {
		starRefinedCorners, starCount := starRefiner.RefineCornersWithStarPoints(img, gridCorners)
		if starCount >= 4 {
			iouOrig := CalculateQuadIoU(gridCorners, starRefinedCorners, w, h)
			if iouOrig >= 75.0 {
				gridCorners = starRefinedCorners
				method = "StarHomography"
			}
		}
	}

	// 5. Förstner 十字交差点サブピクセル吸着 (0.1px 精度へ自動収束)
	gridCorners = RefineCornersSubPix(img, gridCorners)

	// 可視化・デバッグ用: 星（花点）の検出結果を取得
	detectedStars := starRefiner.DetectStarPoints(img, gridCorners)

	area := PolygonArea([]Point{gridCorners[0], gridCorners[1], gridCorners[2], gridCorners[3]})
	imgArea := float64(w * h)

	return &CornerDetectionResult{
		Corners:      gridCorners,
		RawOuter:     rawOuter,
		Area:         area,
		Confidence:   math.Min(1.0, area/(imgArea*0.5)),
		Method:       method,
		Stars:        detectedStars,
		BlobMaskRGBA: blob.ToRGBA(),
	}, nil
}

// fitOuterBoundaryCorners は天板ブロブ境界点群から木枠外縁4直線をRANSAC推定し、外隅4点を算出する
func fitOuterBoundaryCorners(img image.Image, blob *BoardBlobResult, w, h int) [4]Point {
	fitVertLine := func(pts []Point) (float64, float64) {
		n := len(pts)
		if n < 5 {
			return 0, 0
		}
		var bestInliers []Point
		bestA, bestB := 0.0, 0.0
		for iter := 0; iter < 80; iter++ {
			i1 := (iter * 17) % n
			i2 := (iter*31 + 13) % n
			if i1 == i2 || math.Abs(pts[i1].Y-pts[i2].Y) < 15.0 {
				continue
			}
			p1, p2 := pts[i1], pts[i2]
			a := (p2.X - p1.X) / (p2.Y - p1.Y)
			b := p1.X - a*p1.Y

			var inliers []Point
			for _, p := range pts {
				if math.Abs(p.X-(a*p.Y+b)) < 25.0 {
					inliers = append(inliers, p)
				}
			}
			if len(inliers) > len(bestInliers) {
				bestInliers = inliers
				bestA, bestB = a, b
			}
		}

		if len(bestInliers) >= 3 {
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
		}
		return bestA, bestB
	}

	leftPts := blob.LeftPts
	rightPts := blob.RightPts

	// 5.5 幅エンベロープ・フィルタ (Width Envelope Filter):
	// 奥側の石の密集による内側凹みや、手前側木口側面による外側突起を除去
	if len(leftPts) >= 10 && len(rightPts) >= 10 {
		var midWidths []float64
		midYMin := float64(h) * 0.35
		midYMax := float64(h) * 0.65
		for i := 0; i < len(leftPts) && i < len(rightPts); i++ {
			if leftPts[i].Y >= midYMin && leftPts[i].Y <= midYMax {
				w := rightPts[i].X - leftPts[i].X
				if w > 50 {
					midWidths = append(midWidths, w)
				}
			}
		}
		if len(midWidths) >= 5 {
			sort.Float64s(midWidths)
			medMidWidth := midWidths[len(midWidths)/2]
			midYCenter := (midYMin + midYMax) * 0.5

			var cleanLeft, cleanRight []Point
			for i := 0; i < len(leftPts) && i < len(rightPts); i++ {
				y := leftPts[i].Y
				w := rightPts[i].X - leftPts[i].X
				minExpectedW := medMidWidth - math.Max(0, midYCenter-y)*0.35
				if w >= minExpectedW*0.88 {
					cleanLeft = append(cleanLeft, leftPts[i])
					cleanRight = append(cleanRight, rightPts[i])
				}
			}
			if len(cleanLeft) >= 10 {
				leftPts = cleanLeft
				rightPts = cleanRight
			}
		}
	}

	aL, bL := fitVertLine(leftPts)
	aR, bR := fitVertLine(rightPts)

	// 底辺直線 (手前碁石密集による凹み耐性フィルタ)
	botPts := blob.BotPts
	if len(botPts) >= 15 {
		var ys []float64
		for _, p := range botPts {
			ys = append(ys, p.Y)
		}
		sort.Float64s(ys)
		p75Y := ys[int(float64(len(ys))*0.75)]
		var deepBotPts []Point
		for _, p := range botPts {
			if p.Y >= p75Y-140.0 {
				deepBotPts = append(deepBotPts, p)
			}
		}
		if len(deepBotPts) >= 8 {
			botPts = deepBotPts
		}
	}

	fitHorizLine := func(pts []Point) (float64, float64) {
		n := len(pts)
		if n < 5 {
			return 0, 0
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
			if math.Abs(c) > 0.40 {
				continue
			}
			d := p1.Y - c*p1.X

			var inliers []Point
			for _, p := range pts {
				if math.Abs(p.Y-(c*p.X+d)) < 30.0 {
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
				bestC = num / den
			}
			bestD = meanY - bestC*meanX
		}
		return bestC, bestD
	}

	cB, dB := fitHorizLine(botPts)

	minX, maxX := 1e18, -1e18
	for _, p := range botPts {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
	}
	wBot := maxX - minX

	// シャドウライン自己修復
	cB, dB = refineBottomLineWithShadow(img, cB, dB, minX, maxX, wBot)
	botY := cB*((minX+maxX)/2.0) + dB

	// 天板上端の追跡 (中央連続追跡により奥の机への飛び移りを遮断)
	centerGY := int(float64(h) * 0.50 / blob.ScaleY)
	var topPts []Point
	var centerTopYs []float64

	for gx := int(float64(blob.GW) * 0.25); gx <= int(float64(blob.GW)*0.75); gx++ {
		currTopGY := centerGY
		for gy := centerGY; gy >= 0; gy-- {
			if blob.Mask[gy][gx] {
				currTopGY = gy
			} else {
				break
			}
		}
		if currTopGY < centerGY {
			pt := Point{X: float64(gx) * blob.ScaleX, Y: float64(currTopGY) * blob.ScaleY}
			topPts = append(topPts, pt)
			centerTopYs = append(centerTopYs, pt.Y)
		}
	}

	bestTopY := botY - 0.75*wBot
	if len(centerTopYs) >= 5 {
		sort.Float64s(centerTopYs)
		medBlobTopY := centerTopYs[len(centerTopYs)/2]
		bestTopY = medBlobTopY - 25.0
	}

	// 透視投影による上辺傾きの収束拘束
	perspectiveSlopeRatio := 0.35
	cT := cB * perspectiveSlopeRatio
	dT := bestTopY - cT*float64(w)*0.5

	if len(topPts) >= 10 {
		fittedC, fittedD := fitHorizLine(topPts)
		isConsistent := false
		if math.Abs(cB) < 0.03 {
			isConsistent = math.Abs(fittedC) <= 0.08
		} else if cB > 0 {
			isConsistent = fittedC >= -0.02 && fittedC <= cB*1.1
		} else {
			isConsistent = fittedC <= 0.02 && fittedC >= cB*1.1
		}
		if isConsistent {
			cT = fittedC
			dT = fittedD - 25.0
		}
	}

	detTL := IntersectSlopeLines(cT, dT, aL, bL)
	detTR := IntersectSlopeLines(cT, dT, aR, bR)
	detBR := IntersectSlopeLines(cB, dB, aR, bR)
	detBL := IntersectSlopeLines(cB, dB, aL, bL)

	return OrderCorners([4]Point{detTL, detTR, detBR, detBL})
}

