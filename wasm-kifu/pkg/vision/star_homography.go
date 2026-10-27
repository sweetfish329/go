package vision

import (
	"fmt"
	"image"
	"math"
	"sort"
)

// StarPoint は盤面の星（花点）の検出結果
type StarPoint struct {
	GridX      int     `json:"gridX"`      // 0-indexed グリッド座標 (3, 9, 15)
	GridY      int     `json:"gridY"`      // 0-indexed グリッド座標 (3, 9, 15)
	NormX      float64 `json:"normX"`      // 正規化盤面座標 (GridX / 18.0)
	NormY      float64 `json:"normY"`      // 正規化盤面座標 (GridY / 18.0)
	ImgX       float64 `json:"imgX"`       // 画像上の検出サブピクセル座標 X
	ImgY       float64 `json:"imgY"`       // 画像上の検出サブピクセル座標 Y
	Confidence float64 `json:"confidence"` // 検出信頼度 (0.0 - 1.0)
}

// StarHomographyRefiner は「星（花点・天元）」を利用してホモグラフィと4隅を精密化する
type StarHomographyRefiner struct {
	MinStarsForRefine int     // 精密化に必要な最小星数 (推奨: 4)
	SearchRadiusRatio float64 // 探索半径比率 (1マス間隔に対する比率, 推奨: 0.35)
}

// NewStarHomographyRefiner はリファイナーを初期化する
func NewStarHomographyRefiner() *StarHomographyRefiner {
	return &StarHomographyRefiner{
		MinStarsForRefine: 4,
		SearchRadiusRatio: 0.32,
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

// RefineHomographyLM は Levenberg-Marquardt (LM) 法を用いて、
// 検出された星点群に対するホモグラフィ行列 H の非線形再射影誤差を最小化する。
func RefineHomographyLM(initH [9]float64, points []StarPoint) [9]float64 {
	if len(points) < 4 {
		return initH
	}

	curH := initH
	lambda := 1e-3

	calcResiduals := func(h [9]float64) (float64, []float64) {
		totalSq := 0.0
		var r []float64
		for _, pt := range points {
			p := ProjectPoint(h, pt.NormX, pt.NormY)
			rx := p.X - pt.ImgX
			ry := p.Y - pt.ImgY
			totalSq += rx*rx + ry*ry
			r = append(r, rx, ry)
		}
		return totalSq, r
	}

	bestCost, _ := calcResiduals(curH)

	for iter := 0; iter < 10; iter++ {
		var jtJ [8][8]float64
		var jtr [8]float64

		for _, pt := range points {
			u, v := pt.NormX, pt.NormY
			w := curH[6]*u + curH[7]*v + 1.0
			if math.Abs(w) < 1e-9 {
				w = 1e-9
			}
			invW := 1.0 / w
			xHat := (curH[0]*u + curH[1]*v + curH[2]) * invW
			yHat := (curH[3]*u + curH[4]*v + curH[5]) * invW

			rx := xHat - pt.ImgX
			ry := yHat - pt.ImgY

			// ヤコビアン偏微分行 (8要素)
			jx := [8]float64{u * invW, v * invW, invW, 0, 0, 0, -u * xHat * invW, -v * xHat * invW}
			jy := [8]float64{0, 0, 0, u * invW, v * invW, invW, -u * yHat * invW, -v * yHat * invW}

			for r0 := 0; r0 < 8; r0++ {
				for c0 := 0; c0 < 8; c0++ {
					jtJ[r0][c0] += jx[r0]*jx[c0] + jy[r0]*jy[c0]
				}
				jtr[r0] += jx[r0]*rx + jy[r0]*ry
			}
		}

		// (J^T J + lambda * diag) * delta = -jtr
		var a [8][8]float64
		var b [8]float64
		for r0 := 0; r0 < 8; r0++ {
			for c0 := 0; c0 < 8; c0++ {
				a[r0][c0] = jtJ[r0][c0]
			}
			a[r0][r0] += lambda * (math.Abs(jtJ[r0][r0]) + 1.0)
			b[r0] = -jtr[r0]
		}

		delta := solveLinearSystem8x8(a, b)

		candH := [9]float64{
			curH[0] + delta[0],
			curH[1] + delta[1],
			curH[2] + delta[2],
			curH[3] + delta[3],
			curH[4] + delta[4],
			curH[5] + delta[5],
			curH[6] + delta[6],
			curH[7] + delta[7],
			1.0,
		}

		candCost, _ := calcResiduals(candH)
		if candCost < bestCost {
			improvement := bestCost - candCost
			bestCost = candCost
			curH = candH
			lambda *= 0.3
			if improvement < 1e-4 {
				break
			}
		} else {
			lambda *= 5.0
		}
	}

	return curH
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
	searchRadius := int(math.Max(6, math.Round(approxSpacing*r.SearchRadiusRatio)))
	donutRadius := math.Max(8.0, math.Min(16.0, approxSpacing*0.22))

	getRGB := func(x, y int) (float64, float64, float64) {
		if x < 0 || x >= w || y < 0 || y >= h {
			return 0, 0, 0
		}
		r32, g32, b32, _ := img.At(x, y).RGBA()
		return float64(r32 >> 8), float64(g32 >> 8), float64(b32 >> 8)
	}
	getGray := func(x, y int) float64 {
		r, g, b := getRGB(x, y)
		return 0.299*r + 0.587*g + 0.114*b
	}

	var detectedStars []StarPoint

	for _, coord := range standardStarCoords {
		gx, gy := coord[0], coord[1]
		normX := float64(gx) / 18.0
		normY := float64(gy) / 18.0

		pred := ProjectPoint(hInit, normX, normY)
		cx := int(math.Round(pred.X))
		cy := int(math.Round(pred.Y))

		if cx-searchRadius < 5 || cx+searchRadius >= w-5 || cy-searchRadius < 5 || cy+searchRadius >= h-5 {
			continue
		}

		bestX, bestY := -1, -1
		bestContrast := -1.0
		bestBgGray := 0.0

		// 探索ウィンドウ内で星ドット（中心が暗く、周囲360度がすべて木目）を探索
		for dy := -searchRadius; dy <= searchRadius; dy++ {
			for dx := -searchRadius; dx <= searchRadius; dx++ {
				px := cx + dx
				py := cy + dy
				gray := getGray(px, py)
				if gray > 90.0 {
					continue
				}

				// ドーナツ型木目検査: 半径 donutRadius の8方位
				allWood := true
				sumBg := 0.0
				for angle := 0.0; angle < 2*math.Pi; angle += math.Pi / 4 {
					sx := int(math.Round(float64(px) + donutRadius*math.Cos(angle)))
					sy := int(math.Round(float64(py) + donutRadius*math.Sin(angle)))
					sr, _, sb := getRGB(sx, sy)
					sg := getGray(sx, sy)
					// 木目色判定: 赤みが強く、青が低く、十分明るい
					if sr < 95 || sr-sb < 14 || sg < 65 {
						allWood = false
						break
					}
					sumBg += sg
				}
				if !allWood {
					continue
				}

				bgGray := sumBg / 8.0
				contrast := bgGray - gray
				if contrast > bestContrast && contrast >= 25.0 {
					bestContrast = contrast
					bestBgGray = bgGray
					bestX, bestY = px, py
				}
			}
		}

		if bestX == -1 {
			continue // 石で隠れているか星なし
		}

		// 重心サブピクセル精細化 (ドット近傍 3x3)
		var sumW, sumWX, sumWY float64
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				px := bestX + dx
				py := bestY + dy
				gray := getGray(px, py)
				diff := bestBgGray - gray
				if diff > bestContrast*0.35 {
					sumW += diff
					sumWX += diff * float64(px)
					sumWY += diff * float64(py)
				}
			}
		}

		if sumW > 0 {
			subX := sumWX / sumW
			subY := sumWY / sumW

			// Förstner サブピクセル精細化
			refinedPt := RefineCornerSubPixFörstner(img, Point{X: subX, Y: subY}, DefaultFörstnerParams())
			// 過大ドリフト防止: 3px以上離れたら重心を採用
			finalPt := Point{X: subX, Y: subY}
			if math.Hypot(refinedPt.X-subX, refinedPt.Y-subY) <= 3.0 {
				finalPt = refinedPt
			}

			conf := math.Min(1.0, bestContrast/60.0)
			detectedStars = append(detectedStars, StarPoint{
				GridX:      gx,
				GridY:      gy,
				NormX:      normX,
				NormY:      normY,
				ImgX:       finalPt.X,
				ImgY:       finalPt.Y,
				Confidence: conf,
			})
		}
	}

	return detectedStars
}

// checkCollinear は3つの星点が同一線上にあるか判定する
func checkCollinear(p1, p2, p3 StarPoint) bool {
	area2 := (p2.NormX-p1.NormX)*(p3.NormY-p1.NormY) - (p2.NormY-p1.NormY)*(p3.NormX-p1.NormX)
	return math.Abs(area2) < 1e-4
}

// FindHomographyRANSAC は複数の星候補から RANSAC により外れ値（石や影）を除外し、
// 最良のインライア星群とそのホモグラフィ行列を返す
func FindHomographyRANSAC(stars []StarPoint, maxReprojErr float64) ([9]float64, []StarPoint, error) {
	numStars := len(stars)
	if numStars < 4 {
		return [9]float64{}, nil, fmt.Errorf("at least 4 stars required, got %d", numStars)
	}

	// 盤面上の星点は最大9個。信頼度の高い上位9個を優先
	candStars := stars
	if len(candStars) > 9 {
		sorted := make([]StarPoint, len(stars))
		copy(sorted, stars)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Confidence > sorted[j].Confidence
		})
		candStars = sorted[:9]
		numStars = len(candStars)
	}

	bestInliers := []StarPoint{}
	var bestH [9]float64

	// 4点の組み合わせを網羅探索（最大 C(9, 4) = 126 通り）
	for i1 := 0; i1 < numStars; i1++ {
		for i2 := i1 + 1; i2 < numStars; i2++ {
			for i3 := i2 + 1; i3 < numStars; i3++ {
				// 3点が同一直線上の場合はスキップ
				if checkCollinear(candStars[i1], candStars[i2], candStars[i3]) {
					continue
				}
				for i4 := i3 + 1; i4 < numStars; i4++ {
					// 任意の3点の共線判定
					if checkCollinear(candStars[i1], candStars[i2], candStars[i4]) ||
						checkCollinear(candStars[i1], candStars[i3], candStars[i4]) ||
						checkCollinear(candStars[i2], candStars[i3], candStars[i4]) {
						continue
					}

					sample := []StarPoint{candStars[i1], candStars[i2], candStars[i3], candStars[i4]}
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
			}
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

	// Levenberg-Marquardt (LM) 法により非線形再射影誤差を大域的極小値へ精密収束
	hOpt := RefineHomographyLM(hRefined, bestInliers)
	return hOpt, bestInliers, nil
}

// RefineCornersWithStarPoints は検出された星（花点）を用いてホモグラフィを再推定し、真の4隅を精密化する。
// 天板奥側（上辺）が外周石や影により縮んで検出された場合にも対応するため、
// マルチスケール候補（外周拡張）から最も幾何学的整合性の高い星群を自動選択する。
func (r *StarHomographyRefiner) RefineCornersWithStarPoints(img image.Image, initialCorners [4]Point) ([4]Point, int) {
	w := float64(img.Bounds().Dx())
	h := float64(img.Bounds().Dy())
	maxReprojAllowed := math.Max(15.0, math.Max(w, h)*0.015)

	tl, tr, br, bl := initialCorners[0], initialCorners[1], initialCorners[2], initialCorners[3]

	// 候補四隅群の生成: 初期四隅 (dWidth = 0) で星検出を行う
	dWidths := []float64{0.0}
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
			maxDist := math.Max(120.0, math.Max(w, h)*0.10)
			dist := math.Hypot(refined[i].X-initialCorners[i].X, refined[i].Y-initialCorners[i].Y)
			if dist > maxDist {
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

	// 2パス目: 補正された四隅を元に星を再サンプリングしてホモグラフィを極限まで精密化
	pass2Stars := r.DetectStarPoints(img, bestCand.refined)
	if len(pass2Stars) >= len(bestCand.stars) {
		h2, inliers2, err := FindHomographyRANSAC(pass2Stars, maxReprojAllowed)
		if err == nil && len(inliers2) >= 4 {
			refined2 := [4]Point{
				ProjectPoint(h2, 0, 0),
				ProjectPoint(h2, 1, 0),
				ProjectPoint(h2, 1, 1),
				ProjectPoint(h2, 0, 1),
			}
			oob := false
			for i := 0; i < 4; i++ {
				if refined2[i].X < 5 || refined2[i].X > w-5 || refined2[i].Y < 5 || refined2[i].Y > h-5 {
					oob = true
					break
				}
			}
			if !oob {
				return refined2, len(inliers2)
			}
		}
	}

	return bestCand.refined, len(bestCand.stars)
}
