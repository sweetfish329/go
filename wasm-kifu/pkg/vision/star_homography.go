package vision

import (
	"fmt"
	"image"
	"math"
)

// StarPoint は盤面の星（花点）の検出結果
type StarPoint struct {
	GridX      int     // 0-indexed グリッド座標 (3, 9, 15)
	GridY      int     // 0-indexed グリッド座標 (3, 9, 15)
	NormX      float64 // 正規化盤面座標 (GridX / 18.0)
	NormY      float64 // 正規化盤面座標 (GridY / 18.0)
	ImgX       float64 // 画像上の検出サブピクセル座標 X
	ImgY       float64 // 画像上の検出サブピクセル座標 Y
	Confidence float64 // 検出信頼度 (0.0 - 1.0)
}

// StarHomographyRefiner は「星（花点・天元）」を利用してホモグラフィと4隅を精密化する
type StarHomographyRefiner struct {
	MinStarsForRefine int     // 精密化に必要な最小星数 (推奨: 4)
	SearchRadiusRatio float64 // 探索半径比率 (1マス間隔に対する比率, 推奨: 0.35)
}

// NewStarHomographyRefiner はリファイナーを初期化する
func NewStarHomographyRefiner() *StarHomographyRefiner {
	return &StarHomographyRefiner{
		MinStarsForRefine: 7,
		SearchRadiusRatio: 0.35,
	}
}

// 19路盤の9つの星の座標 (0-indexed)
var standardStarCoords = [9][2]int{
	{3, 3}, {9, 3}, {15, 3},
	{3, 9}, {9, 9}, {15, 9},
	{3, 15}, {9, 15}, {15, 15},
}

// getPerspectiveMatrixFromCorners は単位正方形 [0,1]x[0,1] から4隅への3x3射影変換行列を計算する
func getPerspectiveMatrixFromCorners(corners [4]Point) [9]float64 {
	// src: (0,0), (1,0), (1,1), (0,1)
	// dst: corners[0], corners[1], corners[2], corners[3]
	var src [4]Point = [4]Point{
		{X: 0, Y: 0},
		{X: 1, Y: 0},
		{X: 1, Y: 1},
		{X: 0, Y: 1},
	}

	var a [8][8]float64
	var b [8]float64
	for i := 0; i < 4; i++ {
		sx, sy := src[i].X, src[i].Y
		dx, dy := corners[i].X, corners[i].Y

		a[i*2][0] = sx
		a[i*2][1] = sy
		a[i*2][2] = 1
		a[i*2][3] = 0
		a[i*2][4] = 0
		a[i*2][5] = 0
		a[i*2][6] = -sx * dx
		a[i*2][7] = -sy * dx
		b[i*2] = dx

		a[i*2+1][0] = 0
		a[i*2+1][1] = 0
		a[i*2+1][2] = 0
		a[i*2+1][3] = sx
		a[i*2+1][4] = sy
		a[i*2+1][5] = 1
		a[i*2+1][6] = -sx * dy
		a[i*2+1][7] = -sy * dy
		b[i*2+1] = dy
	}

	hSol := solveLinearSystem8x8(a, b)
	return [9]float64{
		hSol[0], hSol[1], hSol[2],
		hSol[3], hSol[4], hSol[5],
		hSol[6], hSol[7], 1.0,
	}
}

// ProjectPoint は射影変換行列 H を適用して点 (x, y) を射影する
func ProjectPoint(h [9]float64, x, y float64) Point {
	w := h[6]*x + h[7]*y + h[8]
	if math.Abs(w) < 1e-9 {
		w = 1e-9
	}
	px := (h[0]*x + h[1]*y + h[2]) / w
	py := (h[3]*x + h[4]*y + h[5]) / w
	return Point{X: px, Y: py}
}

// solveLinearSystem8x8 はガウス消去法で8元連立1次方程式を解く
func solveLinearSystem8x8(a [8][8]float64, b [8]float64) [8]float64 {
	var x [8]float64
	for i := 0; i < 8; i++ {
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

// FindHomographyLeastSquares は複数の対応点 (Norm -> Img) から最小二乗法でロバストなホモグラフィを求める
func FindHomographyLeastSquares(points []StarPoint) ([9]float64, error) {
	n := len(points)
	if n < 4 {
		return [9]float64{}, fmt.Errorf("at least 4 points required, got %d", n)
	}

	// 2N x 8 の連立正規方程式 A^T A h = A^T b を構築
	var atA [8][8]float64
	var atB [8]float64

	for _, pt := range points {
		sx, sy := pt.NormX, pt.NormY
		dx, dy := pt.ImgX, pt.ImgY

		row0 := [8]float64{sx, sy, 1, 0, 0, 0, -sx * dx, -sy * dx}
		row1 := [8]float64{0, 0, 0, sx, sy, 1, -sx * dy, -sy * dy}

		for i := 0; i < 8; i++ {
			for j := 0; j < 8; j++ {
				atA[i][j] += row0[i]*row0[j] + row1[i]*row1[j]
			}
			atB[i] += row0[i]*dx + row1[i]*dy
		}
	}

	hSol := solveLinearSystem8x8(atA, atB)
	return [9]float64{
		hSol[0], hSol[1], hSol[2],
		hSol[3], hSol[4], hSol[5],
		hSol[6], hSol[7], 1.0,
	}, nil
}

// DetectStarPoints は画像および粗い4隅から盤面の「星」を探索・検出する
func (r *StarHomographyRefiner) DetectStarPoints(img image.Image, initialCorners [4]Point) []StarPoint {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	hInit := getPerspectiveMatrixFromCorners(initialCorners)

	// 格子の推定1マス間隔 (ピクセル)
	p00 := ProjectPoint(hInit, 0, 0)
	p10 := ProjectPoint(hInit, 1.0/18.0, 0)
	approxSpacing := math.Hypot(p10.X-p00.X, p10.Y-p00.Y)
	searchRadius := int(math.Max(4, math.Round(approxSpacing*r.SearchRadiusRatio)))

	getRGB := func(x, y int) (float64, float64, float64) {
		r32, g32, b32, _ := img.At(x, y).RGBA()
		return float64(r32 >> 8), float64(g32 >> 8), float64(b32 >> 8)
	}

	var detectedStars []StarPoint

	for _, coord := range standardStarCoords {
		gx, gy := coord[0], coord[1]
		normX := float64(gx) / 18.0
		normY := float64(gy) / 18.0

		pred := ProjectPoint(hInit, normX, normY)
		cx := int(math.Round(pred.X))
		cy := int(math.Round(pred.Y))

		if cx-searchRadius < 2 || cx+searchRadius >= w-2 || cy-searchRadius < 2 || cy+searchRadius >= h-2 {
			continue
		}

		// 探索ウィンドウ内で星（黒ドット）の探索
		// 背景木目色のサンプリング（外枠リング）
		var bgR, bgG, bgB, bgCount float64
		for dy := -searchRadius; dy <= searchRadius; dy++ {
			for dx := -searchRadius; dx <= searchRadius; dx++ {
				dist := math.Hypot(float64(dx), float64(dy))
				if dist >= float64(searchRadius)*0.75 && dist <= float64(searchRadius) {
					px := cx + dx
					py := cy + dy
					if px >= 0 && px < w && py >= 0 && py < h {
						cr, cg, cb := getRGB(px, py)
						bgR += cr
						bgG += cg
						bgB += cb
						bgCount++
					}
				}
			}
		}

		if bgCount < 5 {
			continue
		}
		bgR /= bgCount
		bgG /= bgCount
		bgB /= bgCount
		bgGray := 0.299*bgR + 0.587*bgG + 0.114*bgB

		// 木目判定（背景が木目色であること: R > B + 15）
		if bgR-bgB < 15 || bgGray < 40 {
			// 背景が木目でない場合（石で完全に覆われているか盤外）
			continue
		}

		// 中心近傍で局所最小輝度（黒ドット）を探索
		minGray := 1e9
		bestX, bestY := cx, cy
		for dy := -searchRadius + 1; dy <= searchRadius - 1; dy++ {
			for dx := -searchRadius + 1; dx <= searchRadius - 1; dx++ {
				px := cx + dx
				py := cy + dy
				cr, cg, cb := getRGB(px, py)
				gray := 0.299*cr + 0.587*cg + 0.114*cb
				if gray < minGray {
					minGray = gray
					bestX, bestY = px, py
				}
			}
		}

		// コントラスト比の確認: 星ドットは周囲の木目より明らかに暗い
		contrast := bgGray - minGray
		if contrast < 18.0 {
			continue // 星が見つからない
		}

		// 重心サブピクセル精細化 (ドット近傍 3x3)
		var sumW, sumWX, sumWY float64
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				px := bestX + dx
				py := bestY + dy
				cr, cg, cb := getRGB(px, py)
				gray := 0.299*cr + 0.587*cg + 0.114*cb
				diff := bgGray - gray
				if diff > contrast*0.4 {
					sumW += diff
					sumWX += diff * float64(px)
					sumWY += diff * float64(py)
				}
			}
		}

		if sumW > 0 {
			subX := sumWX / sumW
			subY := sumWY / sumW

			// Förstner サブピクセル精細化も併用して正確な中心を確定
			refinedPt := RefineCornerSubPixFörstner(img, Point{X: subX, Y: subY}, DefaultFörstnerParams())

			conf := math.Min(1.0, contrast/60.0)
			detectedStars = append(detectedStars, StarPoint{
				GridX:      gx,
				GridY:      gy,
				NormX:      normX,
				NormY:      normY,
				ImgX:       refinedPt.X,
				ImgY:       refinedPt.Y,
				Confidence: conf,
			})
		}
	}

	return detectedStars
}

// FindHomographyRANSAC は複数の星候補から RANSAC により外れ値（石や影）を除外し、
// 最良のインライア星群とそのホモグラフィ行列を返す
func FindHomographyRANSAC(stars []StarPoint, maxReprojErr float64) ([9]float64, []StarPoint, error) {
	numStars := len(stars)
	if numStars < 4 {
		return [9]float64{}, nil, fmt.Errorf("at least 4 stars required, got %d", numStars)
	}

	bestInliers := []StarPoint{}
	var bestH [9]float64
	iters := 150
	if numStars == 4 {
		iters = 1
	}

	for it := 0; it < iters; it++ {
		i1 := it % numStars
		i2 := (it*3 + 1) % numStars
		i3 := (it*7 + 2) % numStars
		i4 := (it*11 + 3) % numStars
		if i1 == i2 || i1 == i3 || i1 == i4 || i2 == i3 || i2 == i4 || i3 == i4 {
			continue
		}
		sample := []StarPoint{stars[i1], stars[i2], stars[i3], stars[i4]}
		h, err := FindHomographyLeastSquares(sample)
		if err != nil {
			continue
		}

		var inliers []StarPoint
		for _, s := range stars {
			p := ProjectPoint(h, s.NormX, s.NormY)
			if math.Hypot(p.X-s.ImgX, p.Y-s.ImgY) <= maxReprojErr {
				inliers = append(inliers, s)
			}
		}
		if len(inliers) > len(bestInliers) {
			bestInliers = inliers
			bestH = h
		}
	}

	if len(bestInliers) < 4 {
		return [9]float64{}, nil, fmt.Errorf("no valid 4-point inlier set found")
	}

	// インライア全点を用いて再最小二乗フィッティング
	hRefined, err := FindHomographyLeastSquares(bestInliers)
	if err != nil {
		return bestH, bestInliers, nil
	}
	return hRefined, bestInliers, nil
}

// RefineCornersWithStarPoints は検出された星（花点）を用いてホモグラフィを再推定し、真の4隅を精密化する。
// 天板奥側（上辺）が外周石や影により縮んで検出された場合にも対応するため、
// マルチスケール候補（外周拡張）から最も幾何学的整合性の高い星群を自動選択する。
func (r *StarHomographyRefiner) RefineCornersWithStarPoints(img image.Image, initialCorners [4]Point) ([4]Point, int) {
	w := float64(img.Bounds().Dx())
	h := float64(img.Bounds().Dy())
	maxReprojAllowed := math.Max(15.0, math.Max(w, h)*0.015)

	tl, tr, br, bl := initialCorners[0], initialCorners[1], initialCorners[2], initialCorners[3]

	// 候補四隅群の生成:
	// 1. まず初期四隅そのまま (dWidth = 0) で星検出を試みる
	// 2. 初期四隅で十分な星 (>= 4個) が低誤差で検出できれば採用
	// 3. 奥側の石密集でブロブが縮んでいる場合のみ、外側拡張候補 (dw > 0) を探索
	dWidths := []float64{0.0, 40.0, 80.0, 120.0, 160.0}
	type candidateResult struct {
		refined       [4]Point
		stars         []StarPoint
		meanReprojErr float64
		score         float64
		dWidth        float64
	}
	var bestCand *candidateResult

	for _, dw := range dWidths {
		candCorners := [4]Point{
			{X: tl.X - dw*0.5, Y: tl.Y},
			{X: tr.X + dw*0.5, Y: tr.Y},
			br,
			bl,
		}

		detectedStars := r.DetectStarPoints(img, candCorners)
		if len(detectedStars) < 4 {
			continue
		}

		// RANSAC により外れ値星（石や木目影の誤検出）を棄却
		hRefined, inlierStars, err := FindHomographyRANSAC(detectedStars, maxReprojAllowed)
		if err != nil || len(inlierStars) < 4 {
			continue
		}

		refined := [4]Point{
			ProjectPoint(hRefined, 0, 0),
			ProjectPoint(hRefined, 1, 0),
			ProjectPoint(hRefined, 1, 1),
			ProjectPoint(hRefined, 0, 1),
		}

		// 再投影誤差 (インライアのみ)
		totalReprojErr := 0.0
		totalConf := 0.0
		for _, star := range inlierStars {
			proj := ProjectPoint(hRefined, star.NormX, star.NormY)
			errD := math.Hypot(proj.X-star.ImgX, proj.Y-star.ImgY)
			totalReprojErr += errD
			totalConf += star.Confidence
		}
		meanReprojErr := totalReprojErr / float64(len(inlierStars))

		// 妥当性チェック: 画像境界内かつ過大ドリフト防止
		oob := false
		for i := 0; i < 4; i++ {
			if refined[i].X < 5 || refined[i].X > w-5 ||
				refined[i].Y < 5 || refined[i].Y > h-5 {
				oob = true
				break
			}
			dist := math.Hypot(refined[i].X-initialCorners[i].X, refined[i].Y-initialCorners[i].Y)
			if dist > 45.0 {
				oob = true
				break
			}
		}
		if oob {
			continue
		}

		// ベース (dw == 0) で星が 6個以上・低誤差なら即座に採用
		if dw == 0.0 && len(inlierStars) >= 6 && meanReprojErr <= 10.0 {
			bestCand = &candidateResult{
				refined:       refined,
				stars:         inlierStars,
				meanReprojErr: meanReprojErr,
				score:         1e9,
				dWidth:        dw,
			}
			break
		}

		// 総合スコア: インライア星数重視 (+20点)、再投影誤差ペナルティ (-1.5点/px)、拡張ペナルティ
		score := float64(len(inlierStars))*20.0 - meanReprojErr*1.5 + totalConf*5.0 - dw*0.1

		if bestCand == nil || score > bestCand.score {
			bestCand = &candidateResult{
				refined:       refined,
				stars:         inlierStars,
				meanReprojErr: meanReprojErr,
				score:         score,
				dWidth:        dw,
			}
		}
	}

	if bestCand == nil {
		return initialCorners, 0
	}

	return bestCand.refined, len(bestCand.stars)
}
