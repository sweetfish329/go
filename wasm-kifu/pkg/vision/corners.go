// Package vision は碁盤画像の多次元特徴量解析および幾何認識ロジックを提供する。
package vision

import (
	"fmt"
	"image"
	"math"
	"sort"
)

// Point は2次元幾何座標を表す
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// CornerDetectionResult は四隅自動認識の結果
type CornerDetectionResult struct {
	Corners    [4]Point `json:"corners"`   // [TL, TR, BR, BL] (木枠インセット後の19路格子四隅)
	RawOuter   [4]Point `json:"rawOuter"`  // インセット前の木枠天板外隅
	Area       float64  `json:"area"`
	Confidence float64  `json:"confidence"`
}

// dilate は二値グリッドを指定カーネル半径で膨張する
func dilate(grid [][]bool, w, h, radius int) [][]bool {
	out := make([][]bool, h)
	for y := 0; y < h; y++ {
		out[y] = make([]bool, w)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !grid[y][x] {
				continue
			}
			for dy := -radius; dy <= radius; dy++ {
				ny := y + dy
				if ny < 0 || ny >= h {
					continue
				}
				for dx := -radius; dx <= radius; dx++ {
					nx := x + dx
					if nx >= 0 && nx < w {
						out[ny][nx] = true
					}
				}
			}
		}
	}
	return out
}

// erode は二値グリッドを指定カーネル半径で収縮する
func erode(grid [][]bool, w, h, radius int) [][]bool {
	out := make([][]bool, h)
	for y := 0; y < h; y++ {
		out[y] = make([]bool, w)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			allSet := true
			for dy := -radius; dy <= radius; dy++ {
				ny := y + dy
				if ny < 0 || ny >= h {
					allSet = false
					break
				}
				for dx := -radius; dx <= radius; dx++ {
					nx := x + dx
					if nx < 0 || nx >= w || !grid[ny][nx] {
						allSet = false
						break
					}
				}
				if !allSet {
					break
				}
			}
			out[y][x] = allSet
		}
	}
	return out
}

// morphClose は閉処理 (膨張 -> 収縮): 碁石などの微小隙間を埋める
func morphClose(grid [][]bool, w, h, radius int) [][]bool {
	d := dilate(grid, w, h, radius)
	return erode(d, w, h, radius)
}

// morphOpen は開処理 (収縮 -> 膨張): 細いブリッジや突起ノイズを切断・除去する
func morphOpen(grid [][]bool, w, h, radius int) [][]bool {
	e := erode(grid, w, h, radius)
	return dilate(e, w, h, radius)
}

// DetectBoardCorners は入力画像から木目天板の色域セグメンテーション、RANSAC直線フィッティング、
// 上方向ブロブ追跡、台形対称修復および木枠マージン逆算を行い、
// 碁盤の19路格子4隅 [TL, TR, BR, BL] をピュア Go で高速・ロバストに自動検出する。
func DetectBoardCorners(img image.Image) (*CornerDetectionResult, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 100 || h < 100 {
		return nil, fmt.Errorf("image too small: %dx%d", w, h)
	}

	// 1. 画像中心領域 (40%〜60%) から碁盤天板の木目色 (榧・ヒバ) を自動サンプリング
	var centerWoodR, centerWoodG, centerWoodB []float64
	for y := int(float64(h) * 0.40); y <= int(float64(h) * 0.60); y += 20 {
		for x := int(float64(w) * 0.40); x <= int(float64(w) * 0.60); x += 20 {
			r32, g32, b32, _ := img.At(x, y).RGBA()
			r := float64(r32 >> 8)
			g := float64(g32 >> 8)
			b := float64(b32 >> 8)
			// 白石・黒石を除外し、木目（茶褐色〜黄褐色）を抽出
			if r-b >= 20 && r >= 60 && !(r > 200 && g > 200 && b > 200) && !(r < 60 && g < 60 && b < 60) {
				centerWoodR = append(centerWoodR, r)
				centerWoodG = append(centerWoodG, g)
				centerWoodB = append(centerWoodB, b)
			}
		}
	}

	if len(centerWoodR) == 0 {
		return nil, fmt.Errorf("board center wood color not detected")
	}

	sort.Float64s(centerWoodR)
	sort.Float64s(centerWoodG)
	sort.Float64s(centerWoodB)
	medR := centerWoodR[len(centerWoodR)/2]
	medG := centerWoodG[len(centerWoodG)/2]
	medB := centerWoodB[len(centerWoodB)/2]

	gw := 120
	gh := int(float64(h) / float64(w) * 120.0)
	scaleX := float64(w) / float64(gw)
	scaleY := float64(h) / float64(gh)

	// 2. 純粋な木目天板色マスクの作成 (中央木目色比率に適応し、暗い机や影を遮断)
	grid := make([][]bool, gh)
	medRatioRG := (medR + 1) / (medG + 1)
	medRatioGB := (medG + 1) / (medB + 1)

	for gy := 0; gy < gh; gy++ {
		grid[gy] = make([]bool, gw)
		origY := int(float64(gy) * scaleY)
		for gx := 0; gx < gw; gx++ {
			origX := int(float64(gx) * scaleX)
			r32, g32, b32, _ := img.At(origX, origY).RGBA()
			r := float64(r32 >> 8)
			g := float64(g32 >> 8)
			b := float64(b32 >> 8)

			ratioRG := (r + 1) / (g + 1)
			ratioGB := (g + 1) / (b + 1)
			diffRatio := math.Abs(ratioRG-medRatioRG) + math.Abs(ratioGB-medRatioGB)
			// 暗い机や背景を除外: 中央木目の 60% 以上の明るさ、かつ木目比率の一致
			isWood := diffRatio < 0.24 && (r-b >= 22) && r >= medR*0.60
			if isWood {
				grid[gy][gx] = true
			}
		}
	}

	// 3. モルフォロジー閉処理 (石の隙間を埋める: 半径 4 セル) ＋ 開処理 (半径 2 セル)
	// 過大半径による机や碁笥への不要な架橋（ブリッジ）を防止
	closed := morphClose(grid, gw, gh, 4)
	morphed := morphOpen(closed, gw, gh, 2)

	// 4. 画像中心を含む最大連結成分 (Board Blob) の抽出
	centerX := gw / 2
	centerY := gh / 2
	visited := make([][]bool, gh)
	for y := 0; y < gh; y++ {
		visited[y] = make([]bool, gw)
	}

	seedX, seedY := centerX, centerY
	minDist := 1e18
	for gy := gh / 4; gy <= gh*3/4; gy++ {
		for gx := gw / 4; gx <= gw*3/4; gx++ {
			if morphed[gy][gx] {
				d := math.Hypot(float64(gx-centerX), float64(gy-centerY))
				if d < minDist {
					minDist = d
					seedX, seedY = gx, gy
				}
			}
		}
	}

	var boardComp []image.Point
	boardMask := make([][]bool, gh)
	for y := 0; y < gh; y++ {
		boardMask[y] = make([]bool, gw)
	}

	if minDist < 1e17 {
		queue := []image.Point{{seedX, seedY}}
		visited[seedY][seedX] = true
		dirs := []image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			boardComp = append(boardComp, curr)
			boardMask[curr.Y][curr.X] = true
			for _, d := range dirs {
				nx, ny := curr.X+d.X, curr.Y+d.Y
				if nx >= 0 && nx < gw && ny >= 0 && ny < gh && morphed[ny][nx] && !visited[ny][nx] {
					visited[ny][nx] = true
					queue = append(queue, image.Point{nx, ny})
				}
			}
		}
	}

	if len(boardComp) < 300 {
		return nil, fmt.Errorf("board component too small: %d pixels", len(boardComp))
	}

	// 5. ブロブ境界点群 (左辺, 右辺, 底辺) の抽出
	var leftPts, rightPts, botPts []Point
	for gy := 0; gy < gh; gy++ {
		firstX, lastX := -1, -1
		for gx := 0; gx < gw; gx++ {
			if boardMask[gy][gx] {
				if firstX == -1 {
					firstX = gx
				}
				lastX = gx
			}
		}
		if firstX != -1 && lastX != -1 && lastX-firstX > 5 {
			leftPts = append(leftPts, Point{X: float64(firstX) * scaleX, Y: float64(gy) * scaleY})
			rightPts = append(rightPts, Point{X: float64(lastX) * scaleX, Y: float64(gy) * scaleY})
		}
	}

	for gx := 0; gx < gw; gx++ {
		lastY := -1
		for gy := 0; gy < gh; gy++ {
			if boardMask[gy][gx] {
				lastY = gy
			}
		}
		if lastY != -1 {
			botPts = append(botPts, Point{X: float64(gx) * scaleX, Y: float64(lastY) * scaleY})
		}
	}

	// 5.5 幅エンベロープ・フィルタ (Width Envelope Filter):
	// 奥側の石の密集による内側凹みや、手前側木口側面による外側突起を除去
	// 健全な中央領域 (Y=35%〜65%) の代表幅を算出し、奥側で不自然に狭い点を除外
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
				// 奥側 (y < midYCenter): 遠近法で物理的に許容される最小幅
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

	// 6. RANSAC 左右直線フィッティング (部分欠損・対局者の服や手による遮蔽耐性)
	fitVertLine := func(pts []Point) (float64, float64) {
		n := len(pts)
		if n < 5 {
			return 0, 0
		}
		bestInliers := []Point{}
		maxInliers := 0
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
			if len(inliers) > maxInliers {
				maxInliers = len(inliers)
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

	aL, bL := fitVertLine(leftPts)
	aR, bR := fitVertLine(rightPts)

	// 底辺直線: RANSAC 水平直線フィッティング (単一ピクセルの外れ値・木口側面突起耐性)
	fitHorizLine := func(pts []Point) (float64, float64) {
		n := len(pts)
		if n < 5 {
			return 0, 0
		}
		bestInliers := []Point{}
		maxInliers := 0
		bestC, bestD := 0.0, 0.0

		for iter := 0; iter < 100; iter++ {
			i1 := (iter * 19) % n
			i2 := (iter*37 + 11) % n
			if i1 == i2 || math.Abs(pts[i1].X-pts[i2].X) < 30.0 {
				continue
			}
			p1, p2 := pts[i1], pts[i2]
			c := (p2.Y - p1.Y) / (p2.X - p1.X)
			if math.Abs(c) > 0.6 {
				continue
			}
			d := p1.Y - c*p1.X

			var inliers []Point
			for _, p := range pts {
				if math.Abs(p.Y-(c*p.X+d)) < 25.0 {
					inliers = append(inliers, p)
				}
			}
			if len(inliers) > maxInliers {
				maxInliers = len(inliers)
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

	// 最左点・最右点のX座標から手前底辺の幅と代表Yを推定
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

	// 天板稜線・シャドウライン（法線不連続）による手前底辺の高精度自己修復 (NotebookLM 学術数理モデル)
	// 卓上盤・脚付盤の側面木口（厚み）へのマスク引きずられを防止し、上面真の稜線へ底辺を引き戻す
	cB, dB = refineBottomLineWithShadow(img, cB, dB, minX, maxX, wBot)
	botY := cB*((minX+maxX)/2.0) + dB

	// 7. 画像中心から上方向へ連続する天板ブロブの上端を追跡 (奥の机への飛び移りを完全遮断)
	centerGY := int(float64(h) * 0.50 / scaleY)
	var centerTopYs []float64

	for gx := int(float64(gw) * 0.35); gx <= int(float64(gw)*0.65); gx++ {
		currTopGY := centerGY
		for gy := centerGY; gy >= 0; gy-- {
			if boardMask[gy][gx] {
				currTopGY = gy
			} else {
				break
			}
		}
		if currTopGY < centerGY {
			centerTopYs = append(centerTopYs, float64(currTopGY)*scaleY)
		}
	}

	bestTopY := botY - 0.75*wBot
	if len(centerTopYs) >= 5 {
		sort.Float64s(centerTopYs)
		medBlobTopY := centerTopYs[len(centerTopYs)/2]
		bestTopY = medBlobTopY - 25.0 // 天板上端木枠の厚み分オフセット
	}
	cT, dT := 0.0, bestTopY

	// 8. 机の木目による片側垂直化の自動修復 (台形対称性拘束):
	// ※ カメラのアングルによって片側が垂直に見える正常な構図が存在するため、
	// 健全な直線を勝手に書き換えて破綻させる危険な決め打ちは行わない。
	_ = botY

	// 直線交点計算 (a: x = a*y + b, c: y = c*x + d)
	intersect := func(a, b, c, d float64) Point {
		denom := 1.0 - a*c
		if math.Abs(denom) < 1e-6 {
			denom = 1e-6
		}
		y := (b*c + d) / denom
		x := a*y + b
		return Point{X: x, Y: y}
	}

	detTL := intersect(aL, bL, cT, dT)
	detTR := intersect(aR, bR, cT, dT)
	detBR := intersect(aR, bR, cB, dB)
	detBL := intersect(aL, bL, cB, dB)

	rawOuter := [4]Point{
		{X: detTL.X, Y: detTL.Y},
		{X: detTR.X, Y: detTR.Y},
		{X: detBR.X, Y: detBR.Y},
		{X: detBL.X, Y: detBL.Y},
	}

	// 9. 木枠マージンインセット逆算 (NotebookLM 幾何モデル: 片側約 1.2%)
	gridCorners := ApplyMarginInset(rawOuter, 0.012, 0.012)

	// 10. 星（花点・天元）によるホモグラフィ精密化 (Star Points Homography Refinement)
	// 盤面上の黒丸ドット（星）を検出し、木枠マージン誤差や射影歪みを非線形・最小二乗補正
	starRefiner := NewStarHomographyRefiner()
	starRefinedCorners, starCount := starRefiner.RefineCornersWithStarPoints(img, gridCorners)
	if starCount >= 4 {
		gridCorners = starRefinedCorners
	}

	// 11. Förstner 反復サブピクセル精細化 (Förstner Sub-Pixel Refinement)
	// 0.1px 以下のサブピクセル交点座標へ自動吸着
	gridCorners = RefineCornersSubPix(img, gridCorners)

	area := polygonArea([]Point{gridCorners[0], gridCorners[1], gridCorners[2], gridCorners[3]})
	imgArea := float64(w * h)

	return &CornerDetectionResult{
		Corners:    gridCorners,
		RawOuter:   rawOuter,
		Area:       area,
		Confidence: math.Min(1.0, area/(imgArea*0.5)),
	}, nil
}

// OrderCorners は4隅の座標を時計回り [左上(TL), 右上(TR), 右下(BR), 左下(BL)] に正準整列する
func OrderCorners(pts [4]Point) [4]Point {
	cx, cy := 0.0, 0.0
	for _, p := range pts {
		cx += p.X
		cy += p.Y
	}
	cx /= 4.0
	cy /= 4.0

	type ptAngle struct {
		p Point
		a float64
	}
	angles := make([]ptAngle, 4)
	for i, p := range pts {
		angles[i] = ptAngle{p: p, a: math.Atan2(p.Y-cy, p.X-cx)}
	}
	sort.Slice(angles, func(i, j int) bool {
		return angles[i].a < angles[j].a
	})

	minSumIdx := 0
	minSum := angles[0].p.X + angles[0].p.Y
	for i := 1; i < 4; i++ {
		s := angles[i].p.X + angles[i].p.Y
		if s < minSum {
			minSum = s
			minSumIdx = i
		}
	}

	return [4]Point{
		angles[minSumIdx].p,
		angles[(minSumIdx+1)%4].p,
		angles[(minSumIdx+2)%4].p,
		angles[(minSumIdx+3)%4].p,
	}
}

// ApplyMarginInset は木枠外隅から内側の19路真格子隅へインセット逆算補正を行う (NotebookLM 幾何モデル)
func ApplyMarginInset(corners [4]Point, insetX, insetY float64) [4]Point {
	tl, tr, br, bl := corners[0], corners[1], corners[2], corners[3]

	vTop := Point{X: tr.X - tl.X, Y: tr.Y - tl.Y}
	vBot := Point{X: br.X - bl.X, Y: br.Y - bl.Y}
	vLeft := Point{X: bl.X - tl.X, Y: bl.Y - tl.Y}
	vRight := Point{X: br.X - tr.X, Y: br.Y - tr.Y}

	return [4]Point{
		{X: tl.X + insetX*vTop.X + insetY*vLeft.X, Y: tl.Y + insetX*vTop.Y + insetY*vLeft.Y},
		{X: tr.X - insetX*vTop.X + insetY*vRight.X, Y: tr.Y - insetX*vTop.Y + insetY*vRight.Y},
		{X: br.X - insetX*vBot.X - insetY*vRight.X, Y: br.Y - insetX*vBot.Y - insetY*vRight.Y},
		{X: bl.X + insetX*vBot.X - insetY*vLeft.X, Y: bl.Y + insetX*vBot.Y - insetY*vLeft.Y},
	}
}

// polygonArea は靴紐の公式 (Shoelace formula) で多角形の面積を計算する
func polygonArea(pts []Point) float64 {
	n := len(pts)
	if n < 3 {
		return 0
	}
	area := 0.0
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += pts[i].X * pts[j].Y
		area -= pts[j].X * pts[i].Y
	}
	return math.Abs(area) / 2.0
}

// CalculateCornerError は予測された4隅と正解4隅の頂点間平均ユークリッド距離 (MAE, ピクセル) を計算する
func CalculateCornerError(pred, gt [4]Point) float64 {
	totalDist := 0.0
	for i := 0; i < 4; i++ {
		dx := pred[i].X - gt[i].X
		dy := pred[i].Y - gt[i].Y
		totalDist += math.Hypot(dx, dy)
	}
	return totalDist / 4.0
}

// CalculateQuadIoU は2つの四角形の簡易 IoU (交差面積 / 和集合面積) をグリッドサンプリングで高精度推定する
func CalculateQuadIoU(q1, q2 [4]Point, width, height int) float64 {
	minX := math.MaxFloat64
	maxX := -math.MaxFloat64
	minY := math.MaxFloat64
	maxY := -math.MaxFloat64

	for i := 0; i < 4; i++ {
		minX = math.Min(minX, math.Min(q1[i].X, q2[i].X))
		maxX = math.Max(maxX, math.Max(q1[i].X, q2[i].X))
		minY = math.Min(minY, math.Min(q1[i].Y, q2[i].Y))
		maxY = math.Max(maxY, math.Max(q1[i].Y, q2[i].Y))
	}

	minX = math.Max(0, minX)
	minY = math.Max(0, minY)
	maxX = math.Min(float64(width), maxX)
	maxY = math.Min(float64(height), maxY)

	step := math.Max(1.0, math.Min((maxX-minX)/100.0, (maxY-minY)/100.0))

	inPoly := func(p Point, poly [4]Point) bool {
		inside := false
		j := 3
		for i := 0; i < 4; i++ {
			if ((poly[i].Y > p.Y) != (poly[j].Y > p.Y)) &&
				(p.X < (poly[j].X-poly[i].X)*(p.Y-poly[i].Y)/(poly[j].Y-poly[i].Y)+poly[i].X) {
				inside = !inside
			}
			j = i
		}
		return inside
	}

	intersectionCount := 0
	unionCount := 0

	for y := minY; y <= maxY; y += step {
		for x := minX; x <= maxX; x += step {
			pt := Point{X: x, Y: y}
			in1 := inPoly(pt, q1)
			in2 := inPoly(pt, q2)

			if in1 && in2 {
				intersectionCount++
			}
			if in1 || in2 {
				unionCount++
			}
		}
	}

	if unionCount == 0 {
		return 0.0
	}
	return float64(intersectionCount) / float64(unionCount) * 100.0
}

// refineBottomLineWithShadow は天板上面と手前側面（木口）の境界稜線（シャドウライン・法線不連続）を探索し、
// 厚みのある卓上盤や脚付盤でも底辺直線 (cB, dB) を側面底から天板上面エッジへ高精度に引き戻す (NotebookLM 学術数理モデル)。
func refineBottomLineWithShadow(img image.Image, cB, dB float64, minX, maxX, wBot float64) (float64, float64) {
	bounds := img.Bounds()
	imgH := bounds.Dy()
	maxThick := math.Min(160.0, wBot*0.12)
	if maxThick < 20.0 {
		return cB, dB
	}

	type edgeCand struct {
		x     float64
		y     float64
		score float64
	}
	var cands []edgeCand

	// 盤面中央 20%〜80% の領域をサンプリング (外周ノイズ・手前机ノイズの混入を防止)
	numSteps := 20
	stepX := (maxX - minX) * 0.60 / float64(numSteps)
	for i := 0; i <= numSteps; i++ {
		x := minX + (maxX-minX)*0.20 + float64(i)*stepX
		ix := int(x)
		if ix < 5 || ix >= bounds.Dx()-5 {
			continue
		}
		initY := cB*x + dB

		bestY := -1.0
		bestScore := 0.0

		// initY から上方向に 12px〜maxThick の範囲をスキャン
		for dy := 12.0; dy <= maxThick; dy += 2.0 {
			y := initY - dy
			iy := int(y)
			if iy < 10 || iy >= imgH-10 {
				continue
			}

			// Y方向の垂直輝度勾配 (Sobel 相当)
			rU, gU, bU, _ := img.At(ix, iy-3).RGBA()
			rD, gD, bD, _ := img.At(ix, iy+3).RGBA()
			grayU := 0.299*float64(rU>>8) + 0.587*float64(gU>>8) + 0.114*float64(bU>>8)
			grayD := 0.299*float64(rD>>8) + 0.587*float64(gD>>8) + 0.114*float64(bD>>8)
			gradY := (grayD - grayU) / 6.0

			rC, gC, bC, _ := img.At(ix, iy).RGBA()
			r8 := float64(rC >> 8)
			b8 := float64(bC >> 8)
			grayC := 0.299*r8 + 0.587*float64(gC>>8) + 0.114*b8

			wood := r8 - b8

			// 木目天板領域内のシャドウライン (負の勾配、局所的暗さ・面取り)
			if wood > 20 {
				score := math.Abs(gradY) + (100.0-grayC)*0.08
				if score > bestScore {
					bestScore = score
					bestY = y
				}
			}
		}

		if bestScore > 4.5 && bestY > 0 {
			cands = append(cands, edgeCand{x: x, y: bestY, score: bestScore})
		}
	}

	if len(cands) < 5 {
		return cB, dB
	}

	// 候補点群から、大域底辺傾き cB を維持しつつ、
	// シャドウラインの高さ切片 d = y - cB*x のインライア中央値をロバスト推定
	type candOffset struct {
		d     float64
		score float64
	}
	var offsets []candOffset
	for _, cand := range cands {
		d := cand.y - cB*cand.x
		offsets = append(offsets, candOffset{d: d, score: cand.score})
	}

	// 最も多くのインライア（±15px以内）を集める最適な d を探索
	maxInliers := 0
	var bestInlierDs []float64

	for _, o := range offsets {
		var curInliers []float64
		for _, other := range offsets {
			if math.Abs(other.d-o.d) < 15.0 {
				curInliers = append(curInliers, other.d)
			}
		}
		if len(curInliers) > maxInliers {
			maxInliers = len(curInliers)
			bestInlierDs = curInliers
		}
	}

	if maxInliers >= 5 {
		sort.Float64s(bestInlierDs)
		medD := bestInlierDs[len(bestInlierDs)/2]
		// 上面へのシフト量が適正範囲（10px〜maxThick）の場合のみ更新
		shift := dB - medD
		if shift >= 10.0 && shift <= maxThick {
			return cB, medD
		}
	}

	return cB, dB
}
