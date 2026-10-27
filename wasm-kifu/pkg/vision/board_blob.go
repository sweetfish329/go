package vision

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
)

// BoardBlobResult は天板木目ブロブの検出結果
type BoardBlobResult struct {
	Mask      [][]bool // ブロブの2値マスク [gh][gw]
	GW        int      // マスク幅
	GH        int      // マスク高さ
	ScaleX    float64  // マスクから元画像Xへの倍率
	ScaleY    float64  // マスクから元画像Yへの倍率
	LeftPts   []Point  // 左境界点群 (元画像座標)
	RightPts  []Point  // 右境界点群 (元画像座標)
	BotPts    []Point  // 底境界点群 (元画像座標)
	TopPts    []Point  // 上境界点群 (元画像座標)
	MedWoodR  float64
	MedWoodG  float64
	MedWoodB  float64
	PixelArea int
}

// DetectBoardBlob は画像から碁盤天板の木目ブロブを検出し、境界点群を抽出する
func DetectBoardBlob(img image.Image) (*BoardBlobResult, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 100 || h < 100 {
		return nil, fmt.Errorf("image too small: %dx%d", w, h)
	}

	// 1. 画像中心領域から碁盤天板の均一な木目色 (榧・ヒバ) を自動サンプリング
	// 局所 5x5 の輝度標準偏差を計算し、碁石の急峻な輪郭エッジを除外
	calcLocalStdDev := func(cx, cy int) float64 {
		var grays []float64
		for dy := -2; dy <= 2; dy += 2 {
			py := cy + dy
			if py < 0 || py >= h {
				continue
			}
			for dx := -2; dx <= 2; dx += 2 {
				px := cx + dx
				if px < 0 || px >= w {
					continue
				}
				r32, g32, b32, _ := img.At(px, py).RGBA()
				r := float64(r32 >> 8)
				g := float64(g32 >> 8)
				b := float64(b32 >> 8)
				gray := 0.299*r + 0.587*g + 0.114*b
				grays = append(grays, gray)
			}
		}
		if len(grays) < 5 {
			return 999.0
		}
		n := float64(len(grays))
		meanGray := 0.0
		for _, g := range grays {
			meanGray += g
		}
		meanGray /= n
		var sumSqDiff float64
		for _, g := range grays {
			sumSqDiff += (g - meanGray) * (g - meanGray)
		}
		return math.Sqrt(sumSqDiff / n)
	}

	var centerWoodR, centerWoodG, centerWoodB []float64
	for y := int(float64(h) * 0.40); y <= int(float64(h) * 0.60); y += 20 {
		for x := int(float64(w) * 0.40); x <= int(float64(w) * 0.60); x += 20 {
			r32, g32, b32, _ := img.At(x, y).RGBA()
			r := float64(r32 >> 8)
			g := float64(g32 >> 8)
			b := float64(b32 >> 8)
			// 白石・黒石を除外し、木目（茶褐色〜黄褐色）を抽出
			if r-b >= 20 && r >= 60 && !(r > 200 && g > 200 && b > 200) && !(r < 60 && g < 60 && b < 60) {
				stdDev := calcLocalStdDev(x, y)
				if stdDev <= 35.0 {
					centerWoodR = append(centerWoodR, r)
					centerWoodG = append(centerWoodG, g)
					centerWoodB = append(centerWoodB, b)
				}
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
			isWood := diffRatio < 0.28 && (r-b >= 22) && r >= medR*0.60
			if isWood {
				grid[gy][gx] = true
			}
		}
	}

	// 3. モルフォロジー閉処理 (石の隙間を埋める: 半径 4 セル) ＋ 開処理 (半径 2 セル)
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
				nx, ny := curr.X + d.X, curr.Y + d.Y
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

	// 5. ブロブ境界点群 (左辺, 右辺, 底辺, 上辺) の抽出
	var leftPts, rightPts, botPts, topPts []Point
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
		firstY, lastY := -1, -1
		for gy := 0; gy < gh; gy++ {
			if boardMask[gy][gx] {
				if firstY == -1 {
					firstY = gy
				}
				lastY = gy
			}
		}
		if lastY != -1 {
			botPts = append(botPts, Point{X: float64(gx) * scaleX, Y: float64(lastY) * scaleY})
		}
		if firstY != -1 {
			topPts = append(topPts, Point{X: float64(gx) * scaleX, Y: float64(firstY) * scaleY})
		}
	}

	return &BoardBlobResult{
		Mask:      boardMask,
		GW:        gw,
		GH:        gh,
		ScaleX:    scaleX,
		ScaleY:    scaleY,
		LeftPts:   leftPts,
		RightPts:  rightPts,
		BotPts:    botPts,
		TopPts:    topPts,
		MedWoodR:  medR,
		MedWoodG:  medG,
		MedWoodB:  medB,
		PixelArea: len(boardComp),
	}, nil
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

// refineBottomLineWithShadow は天板上面と手前側面（木口）の境界稜線（シャドウライン・法線不連続）を探索し、
// 厚みのある卓上盤や脚付盤でも底辺直線 (cB, dB) を側面底から天板上面エッジへ高精度に引き戻す (NotebookLM 学術数理モデル)。
func refineBottomLineWithShadow(img image.Image, cB, dB float64, minX, maxX, wBot float64) (float64, float64) {
	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()
	scale := math.Min(1.0, math.Max(float64(imgW), float64(imgH))/2156.0)
	if scale < 0.4 {
		scale = 0.4
	}

	maxThick := math.Min(160.0*scale, wBot*0.12)
	if maxThick < 16.0*scale {
		return cB, dB
	}

	type edgeCand struct {
		x     float64
		y     float64
		score float64
	}
	var cands []edgeCand

	numSteps := 20
	stepX := (maxX - minX) * 0.60 / float64(numSteps)
	minDy := math.Max(8.0, 12.0*scale)
	stepDy := math.Max(1.0, 2.0*scale)

	for i := 0; i <= numSteps; i++ {
		x := minX + (maxX-minX)*0.20 + float64(i)*stepX
		ix := int(x)
		if ix < 5 || ix >= imgW-5 {
			continue
		}
		initY := cB*x + dB

		bestY := -1.0
		bestScore := 0.0

		for dy := minDy; dy <= maxThick; dy += stepDy {
			y := initY - dy
			iy := int(y)
			if iy < 10 || iy >= imgH-10 {
				continue
			}

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

	type candOffset struct {
		d     float64
		score float64
	}
	var offsets []candOffset
	for _, cand := range cands {
		d := cand.y - cB*cand.x
		offsets = append(offsets, candOffset{d: d, score: cand.score})
	}

	maxInliers := 0
	var bestInlierDs []float64
	inlierTol := 15.0 * scale

	for _, o := range offsets {
		var curInliers []float64
		for _, other := range offsets {
			if math.Abs(other.d-o.d) < inlierTol {
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
		shift := dB - medD
		if shift >= 10.0*scale && shift <= maxThick {
			return cB, medD
		}
	}

	return cB, dB
}

// ToRGBA は天板ブロブの2値マスクを可視化・デバッグ用の *image.RGBA 画像として生成する。
// 天板領域は木目調（明るい黄褐色）、背景は暗色でレンダリングする。
func (b *BoardBlobResult) ToRGBA() *image.RGBA {
	rgba := image.NewRGBA(image.Rect(0, 0, b.GW, b.GH))
	for y := 0; y < b.GH; y++ {
		for x := 0; x < b.GW; x++ {
			if b.Mask[y][x] {
				// 天板ブロブ領域: 碁盤色 (R: 219, G: 178, B: 104)
				rgba.SetRGBA(x, y, color.RGBA{R: 219, G: 178, B: 104, A: 255})
			} else {
				// 背景: 暗灰色 (R: 35, G: 38, B: 48)
				rgba.SetRGBA(x, y, color.RGBA{R: 35, G: 38, B: 48, A: 255})
			}
		}
	}
	return rgba
}

