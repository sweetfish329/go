// Package main は test2-1.jpg, test2-2.jpg, test2.sgf を用いた
// 盤面画像認識の精度検証および評価を行うツール。
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/jpeg"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type Point struct {
	X, Y float64
}

type GroundTruth struct {
	BoardSize int          `json:"boardSize"`
	Grid      [19][19]int8 `json:"grid"`
}

func loadUpright(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := raw.Bounds()
	w, h := b.Dx(), b.Dy()
	upright := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			upright.Set(h-1-y, x, raw.At(x, y))
		}
	}
	return upright, nil
}

func solveLinearSystem(a [8][8]float64, b [8]float64) [8]float64 {
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

func transformPoint(m [9]float64, u, v float64) (float64, float64) {
	denom := m[6]*u + m[7]*v + m[8]
	if math.Abs(denom) < 1e-9 {
		denom = 1e-9
	}
	return (m[0]*u + m[1]*v + m[2]) / denom, (m[3]*u + m[4]*v + m[5]) / denom
}

func getPixelBilinear(img *image.RGBA, x, y float64) color.RGBA {
	b := img.Bounds()
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	if ix < b.Min.X || ix+1 >= b.Max.X || iy < b.Min.Y || iy+1 >= b.Max.Y {
		return img.RGBAAt(int(x), int(y))
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

func warpPerspectiveWithMargin(src *image.RGBA, srcCorners [4]Point, dstW, dstH int, margin float64) *image.RGBA {
	dstCorners := [4]Point{
		{X: margin, Y: margin},
		{X: float64(dstW) - margin, Y: margin},
		{X: float64(dstW) - margin, Y: float64(dstH) - margin},
		{X: margin, Y: float64(dstH) - margin},
	}
	invMat := getPerspectiveMatrix(dstCorners, srcCorners)

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			sx, sy := transformPoint(invMat, float64(x), float64(y))
			dst.SetRGBA(x, y, getPixelBilinear(src, sx, sy))
		}
	}
	return dst
}

func saveWarpedWithGrid(warped *image.RGBA, gridPoints [19][19]Point, outPath string) {
	b := warped.Bounds()
	debugImg := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			debugImg.Set(x, y, warped.At(x, y))
		}
	}

	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	green := color.RGBA{R: 0, G: 255, B: 0, A: 255}

	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			cx := int(gridPoints[y][x].X + 0.5)
			cy := int(gridPoints[y][x].Y + 0.5)
			c := red
			if (x == 3 || x == 9 || x == 15) && (y == 3 || y == 9 || y == 15) {
				c = green // 星
			}
			for dy := -3; dy <= 3; dy++ {
				for dx := -3; dx <= 3; dx++ {
					if dx*dx+dy*dy <= 9 {
						debugImg.Set(cx+dx, cy+dy, c)
					}
				}
			}
		}
	}

	f, _ := os.Create(outPath)
	defer f.Close()
	jpeg.Encode(f, debugImg, &jpeg.Options{Quality: 90})
}

type PointData struct {
	X, Y                    int
	CX, CY                  int
	R, G, B, L              float64
	RMB, NormR, NormB       float64
	LocalBgR, LocalBgB      float64
}

func evaluateGridPoints(warped *image.RGBA, gt GroundTruth, testLabel string, gridPoints [19][19]Point) float64 {
	dstW := warped.Bounds().Dx()
	dstH := warped.Bounds().Dy()

	var data [19][19]PointData
	sampleR := 5

	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			cx := int(gridPoints[y][x].X + 0.5)
			cy := int(gridPoints[y][x].Y + 0.5)

			var sumR, sumG, sumB, count float64
			for dy := -sampleR; dy <= sampleR; dy++ {
				for dx := -sampleR; dx <= sampleR; dx++ {
					if dx*dx+dy*dy <= sampleR*sampleR {
						px, py := cx+dx, cy+dy
						if px >= 0 && px < dstW && py >= 0 && py < dstH {
							c := warped.RGBAAt(px, py)
							sumR += float64(c.R)
							sumG += float64(c.G)
							sumB += float64(c.B)
							count++
						}
					}
				}
			}

			r, g, b := 80.0, 60.0, 40.0
			if count > 0 {
				r = sumR / count
				g = sumG / count
				b = sumB / count
			}
			l := 0.299*r + 0.587*g + 0.114*b

			data[y][x] = PointData{
				X:   x,
				Y:   y,
				CX:  cx,
				CY:  cy,
				R:   r,
				G:   g,
				B:   b,
				L:   l,
				RMB: r - b,
			}
		}
	}

	// 局所背景適応 (近傍3マスの中央値)
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			var rList, bList []float64
			for dy := -3; dy <= 3; dy++ {
				for dx := -3; dx <= 3; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < 19 && ny >= 0 && ny < 19 {
						rList = append(rList, data[ny][nx].R)
						bList = append(bList, data[ny][nx].B)
					}
				}
			}
			if len(rList) == 0 {
				rList = append(rList, 80.0)
				bList = append(bList, 40.0)
			}
			sort.Float64s(rList)
			sort.Float64s(bList)
			bgR := rList[len(rList)/2]
			bgB := bList[len(bList)/2]
			if bgR < 15 {
				bgR = 15
			}
			if bgB < 15 {
				bgB = 15
			}
			data[y][x].LocalBgR = bgR
			data[y][x].LocalBgB = bgB
			data[y][x].NormR = data[y][x].R / bgR
			data[y][x].NormB = data[y][x].B / bgB
		}
	}

	// 碁石判定 (0=空, 1=黒, 2=白)
	var detected [19][19]int8
	matchCount := 0
	confusion := [3][3]int{}

	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			d := data[y][x]
			stone := int8(0)

			// 白石判定: 無彩色かつ高輝度、局所背景青成分より高い
			if d.RMB < 32.0 && d.R >= 85.0 && (d.NormB > 1.12 || (d.NormR > 1.04 && d.RMB < 20.0)) {
				stone = 2
			} else if d.R >= 135 && d.RMB <= 22 {
				stone = 2
			}

			// 黒石判定: 無彩色かつ低輝度、局所背景より暗い
			if stone == 0 {
				if d.RMB < 35.0 {
					if d.R < 75.0 && (d.NormR < 0.90 || d.NormB < 0.90) {
						stone = 1
					} else if d.NormB < 0.82 && d.NormR < 0.88 {
						stone = 1
					} else if d.B < 45.0 && d.R < 75.0 && d.RMB < 25.0 {
						stone = 1
					}
				}
			}

			detected[y][x] = stone
			a := gt.Grid[y][x]
			confusion[a][stone]++
			if a == stone {
				matchCount++
			}
		}
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("   【%s】盤面認識評価レポート\n", testLabel)
	fmt.Printf("=======================================================\n")
	cols := "   A B C D E F G H J K L M N O P Q R S T"
	fmt.Println(cols)

	for y := 0; y < 19; y++ {
		rowStr := fmt.Sprintf("%2d ", 19-y)
		for x := 0; x < 19; x++ {
			p := detected[y][x]
			switch p {
			case 1:
				rowStr += "● "
			case 2:
				rowStr += "○ "
			default:
				rowStr += ". "
			}
		}
		fmt.Printf("%s%2d\n", rowStr, 19-y)
	}
	fmt.Println(cols)

	acc := float64(matchCount) / 361.0 * 100.0
	fmt.Printf("\n全361交点 正解率 (Accuracy): %.2f%% (%d / 361)\n", acc, matchCount)

	fmt.Println("\n--- 行ごとの正解率 (Row-by-Row Accuracy) ---")
	for y := 0; y < 19; y++ {
		rowCount := 0
		for x := 0; x < 19; x++ {
			if gt.Grid[y][x] == detected[y][x] {
				rowCount++
			}
		}
		rowAcc := float64(rowCount) / 19.0 * 100.0
		fmt.Printf("行 %2d: %5.1f%% (%2d / 19)\n", 19-y, rowAcc, rowCount)
	}
	fmt.Println("\n混同行列 (Confusion Matrix):")
	fmt.Println("             [予測:空] [予測:黒] [予測:白]")
	fmt.Printf("[正解:空(0)]   %4d     %4d     %4d\n", confusion[0][0], confusion[0][1], confusion[0][2])
	fmt.Printf("[正解:黒(1)]   %4d     %4d     %4d\n", confusion[1][0], confusion[1][1], confusion[1][2])
	fmt.Printf("[正解:白(2)]   %4d     %4d     %4d\n", confusion[2][0], confusion[2][1], confusion[2][2])

	calcMetrics := func(name string, cIdx int) {
		tp := confusion[cIdx][cIdx]
		totalActual := confusion[cIdx][0] + confusion[cIdx][1] + confusion[cIdx][2]
		totalPred := confusion[0][cIdx] + confusion[1][cIdx] + confusion[2][cIdx]
		recall := 0.0
		if totalActual > 0 {
			recall = float64(tp) / float64(totalActual) * 100.0
		}
		prec := 0.0
		if totalPred > 0 {
			prec = float64(tp) / float64(totalPred) * 100.0
		}
		fmt.Printf("%s - 再現率(Recall): %.1f%% (%d/%d), 適合率(Precision): %.1f%% (%d/%d)\n",
			name, recall, tp, totalActual, prec, tp, totalPred)
	}
	fmt.Println()
	calcMetrics("黒石 (●)", 1)
	calcMetrics("白石 (○)", 2)
	calcMetrics("空交点(.)", 0)

	colNames := []string{"A", "B", "C", "D", "E", "F", "G", "H", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T"}
	typeNames := []string{"空", "黒", "白"}
	fmt.Println("\n--- 主な誤判定交点 (Top 25) ---")
	errCount := 0
	for y := 0; y < 19 && errCount < 25; y++ {
		for x := 0; x < 19 && errCount < 25; x++ {
			a := gt.Grid[y][x]
			p := detected[y][x]
			if a != p {
				d := data[y][x]
				fmt.Printf("%s%d (%d,%d) [cx=%d,cy=%d]: 正解=%s, 予測=%s | R=%.1f, B=%.1f, RMB=%.1f, NormR=%.2f, NormB=%.2f\n",
					colNames[x], 19-y, x, y, d.CX, d.CY, typeNames[a], typeNames[p], d.R, d.B, d.RMB, d.NormR, d.NormB)
				errCount++
			}
		}
	}

	return acc
}

func findFile(filename string) string {
	candidates := []string{
		filename,
		filepath.Join("..", filename),
		filepath.Join("../..", filename),
		filepath.Join("wasm-kifu", filename),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return filename
}

func main() {
	gtFile := findFile("test2_ground_truth.json")
	gtData, err := os.ReadFile(gtFile)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", gtFile, err)
		return
	}
	var gt GroundTruth
	if err := json.Unmarshal(gtData, &gt); err != nil {
		fmt.Printf("Error unmarshaling GT: %v\n", err)
		return
	}

	// -------------------------------------------------------------
	// 1. test2-1.jpg (正面視点)
	// -------------------------------------------------------------
	img1Path := findFile("test2-1.jpg")
	img1, err := loadUpright(img1Path)
	if err != nil {
		fmt.Printf("Error loading %s: %v\n", img1Path, err)
		return
	}

	dstW := 680
	dstH := 730
	margin := 40.0

	// 四隅: A19, T19, T1, A1
	corners1 := [4]Point{
		{X: 456, Y: 696},  // A19
		{X: 1582, Y: 696}, // T19
		{X: 1949, Y: 1848},// T1
		{X: 101, Y: 1860}, // A1
	}
	warped1 := warpPerspectiveWithMargin(img1, corners1, dstW, dstH, margin)

	boardW := float64(dstW) - 2*margin
	boardH := float64(dstH) - 2*margin

	// 透視ピッチ勾配 (行方向)
	rowWeights1 := make([]float64, 18)
	for i := 0; i < 18; i++ {
		rowWeights1[i] = 33.0 + (39.0-33.0)*(float64(i)/17.0)
	}
	totalW1 := 0.0
	for _, rw := range rowWeights1 {
		totalW1 += rw
	}
	baseGridY1 := make([]float64, 19)
	accY1 := 0.0
	for i := 0; i < 18; i++ {
		accY1 += (rowWeights1[i] / totalW1) * boardH
		baseGridY1[i+1] = accY1
	}
	baseGridY1[18] = boardH

	var gridPoints1 [19][19]Point
	for y := 0; y < 19; y++ {
		cy := margin + baseGridY1[y]
		for x := 0; x < 19; x++ {
			tX := float64(x) / 18.0
			cx := margin + tX*boardW
			gridPoints1[y][x] = Point{X: cx, Y: cy}
		}
	}

	saveWarpedWithGrid(warped1, gridPoints1, "test2-1_warped_grid.jpg")
	acc1 := evaluateGridPoints(warped1, gt, "test2-1.jpg (初期アライメント)", gridPoints1)
	fmt.Printf("\ntest2-1.jpg 初期正解率: %.2f%%\n", acc1)

	// グリッドアライメント最適化 (台形スケーリング・オフセット探索)
	fmt.Println("\n--- グリッドアライメント最適化探索中... ---")
	bestAcc := acc1
	bestTopScaleX := 1.0
	bestBotScaleX := 1.0
	bestTopOffX := 0.0
	bestBotOffX := 0.0
	bestOffY := 8.0
	bestScaleY := 1.04
	var bestGrid [19][19]Point

	for topScaleX := 0.98; topScaleX <= 1.06; topScaleX += 0.02 {
		for botScaleX := 1.00; botScaleX <= 1.08; botScaleX += 0.02 {
			for topOffX := -8.0; topOffX <= 8.0; topOffX += 4.0 {
				for botOffX := -8.0; botOffX <= 8.0; botOffX += 4.0 {
					for offY := 4.0; offY <= 14.0; offY += 2.0 {
						for scaleY := 1.02; scaleY <= 1.06; scaleY += 0.02 {
							var testGrid [19][19]Point
							for y := 0; y < 19; y++ {
								tY := float64(y) / 18.0
								currScaleX := topScaleX + (botScaleX-topScaleX)*tY
								currOffX := topOffX + (botOffX-topOffX)*tY

								baseY := baseGridY1[y]
								cy := margin + (boardH/2.0) + (baseY-(boardH/2.0))*scaleY + offY
								for x := 0; x < 19; x++ {
									tX := float64(x) / 18.0
									baseX := tX * boardW
									cx := margin + (boardW/2.0) + (baseX-(boardW/2.0))*currScaleX + currOffX
									testGrid[y][x] = Point{X: cx, Y: cy}
								}
							}

							// 評価
							matches := 0
							for y := 0; y < 19; y++ {
								for x := 0; x < 19; x++ {
									cx := int(testGrid[y][x].X + 0.5)
									cy := int(testGrid[y][x].Y + 0.5)
									if cx < 0 || cx >= dstW || cy < 0 || cy >= dstH {
										continue
									}
									c := warped1.RGBAAt(cx, cy)
									r, _, b := float64(c.R), float64(c.G), float64(c.B)
									rmb := r - b

									pred := int8(0)
									if rmb < 32.0 && r >= 85.0 && b >= 60.0 {
										pred = 2
									} else if rmb < 35.0 && r < 75.0 && b < 75.0 {
										pred = 1
									}
									if pred == gt.Grid[y][x] {
										matches++
									}
								}
							}
							currAcc := float64(matches) / 361.0 * 100.0
							if currAcc > bestAcc {
								bestAcc = currAcc
								bestTopScaleX = topScaleX
								bestBotScaleX = botScaleX
								bestTopOffX = topOffX
								bestBotOffX = botOffX
								bestOffY = offY
								bestScaleY = scaleY
								bestGrid = testGrid
							}
						}
					}
				}
			}
		}
	}

	fmt.Printf("最適パラメータ発見: TopScaleX=%.2f, BotScaleX=%.2f, TopOffX=%.1f, BotOffX=%.1f, OffY=%.1f, ScaleY=%.2f (暫定Acc: %.2f%%)\n",
		bestTopScaleX, bestBotScaleX, bestTopOffX, bestBotOffX, bestOffY, bestScaleY, bestAcc)

	if bestAcc > acc1 {
		saveWarpedWithGrid(warped1, bestGrid, "test2-1_optimized_grid.jpg")
		accOpt := evaluateGridPoints(warped1, gt, "test2-1.jpg (最適化アライメント)", bestGrid)
		fmt.Printf("\ntest2-1.jpg 最適化後正解率: %.2f%%\n", accOpt)
	}

	// -------------------------------------------------------------
	// 2. test2-2.jpg (斜めアングル視点)
	// -------------------------------------------------------------
	img2Path := findFile("test2-2.jpg")
	img2, err := loadUpright(img2Path)
	if err != nil {
		fmt.Printf("Error loading %s: %v\n", img2Path, err)
		return
	}

	dstW2 := 680
	dstH2 := 730
	margin2 := 40.0

	// 四隅: A19, T19, T1, A1
	// A19: (340, 640), T19: (1380, 730), T1: (1795, 1806), A1: (380, 2094)
	corners2 := [4]Point{
		{X: 340, Y: 640},  // A19
		{X: 1380, Y: 730}, // T19
		{X: 1795, Y: 1806},// T1
		{X: 380, Y: 2094}, // A1
	}
	warped2 := warpPerspectiveWithMargin(img2, corners2, dstW2, dstH2, margin2)

	boardW2 := float64(dstW2) - 2*margin2
	boardH2 := float64(dstH2) - 2*margin2

	// 透視ピッチ勾配 (行方向)
	rowWeights2 := make([]float64, 18)
	for i := 0; i < 18; i++ {
		rowWeights2[i] = 30.0 + (42.0-30.0)*(float64(i)/17.0)
	}
	totalW2 := 0.0
	for _, rw := range rowWeights2 {
		totalW2 += rw
	}
	baseGridY2 := make([]float64, 19)
	accY2 := 0.0
	for i := 0; i < 18; i++ {
		accY2 += (rowWeights2[i] / totalW2) * boardH2
		baseGridY2[i+1] = accY2
	}
	baseGridY2[18] = boardH2

	var gridPoints2 [19][19]Point
	for y := 0; y < 19; y++ {
		cy := margin2 + baseGridY2[y]
		for x := 0; x < 19; x++ {
			tX := float64(x) / 18.0
			cx := margin2 + tX*boardW2
			gridPoints2[y][x] = Point{X: cx, Y: cy}
		}
	}

	saveWarpedWithGrid(warped2, gridPoints2, "test2-2_warped_grid.jpg")
	acc2Init := evaluateGridPoints(warped2, gt, "test2-2.jpg (斜め視点・初期アライメント)", gridPoints2)
	fmt.Printf("\ntest2-2.jpg 初期正解率: %.2f%%\n", acc2Init)

	// test2-2 アライメント最適化探索 (シアー・台形・ピッチ補正)
	fmt.Println("\n--- test2-2 グリッドアライメント最適化探索中... ---")
	bestAcc2 := acc2Init
	var bestGrid2 [19][19]Point
	bestTopOffX2 := 0.0
	bestBotOffX2 := 0.0
	bestTopScaleX2 := 1.0
	bestBotScaleX2 := 1.0
	bestOffY2 := 0.0
	bestScaleY2 := 1.0

	for tOffX := -36.0; tOffX <= 8.0; tOffX += 4.0 {
		for bOffX := -8.0; bOffX <= 20.0; bOffX += 4.0 {
			for tScaleX := 0.92; tScaleX <= 1.04; tScaleX += 0.02 {
				for bScaleX := 0.98; bScaleX <= 1.08; bScaleX += 0.02 {
					for oY := -16.0; oY <= 16.0; oY += 4.0 {
						for sY := 0.96; sY <= 1.06; sY += 0.02 {
							var testGrid [19][19]Point
							for y := 0; y < 19; y++ {
								tY := float64(y) / 18.0
								currScaleX := tScaleX + (bScaleX-tScaleX)*tY
								currOffX := tOffX + (bOffX-tOffX)*tY

								baseY := baseGridY2[y]
								cy := margin2 + (boardH2/2.0) + (baseY-(boardH2/2.0))*sY + oY
								for x := 0; x < 19; x++ {
									tX := float64(x) / 18.0
									baseX := tX * boardW2
									cx := margin2 + (boardW2/2.0) + (baseX-(boardW2/2.0))*currScaleX + currOffX
									testGrid[y][x] = Point{X: cx, Y: cy}
								}
							}

							matches := 0
							for y := 0; y < 19; y++ {
								for x := 0; x < 19; x++ {
									cx := int(testGrid[y][x].X + 0.5)
									cy := int(testGrid[y][x].Y + 0.5)
									if cx < 0 || cx >= dstW2 || cy < 0 || cy >= dstH2 {
										continue
									}
									c := warped2.RGBAAt(cx, cy)
									r, _, b := float64(c.R), float64(c.G), float64(c.B)
									rmb := r - b

									pred := int8(0)
									if rmb < 32.0 && r >= 85.0 && b >= 60.0 {
										pred = 2
									} else if rmb < 35.0 && r < 75.0 && b < 75.0 {
										pred = 1
									}
									if pred == gt.Grid[y][x] {
										matches++
									}
								}
							}
							currAcc := float64(matches) / 361.0 * 100.0
							if currAcc > bestAcc2 {
								bestAcc2 = currAcc
								bestGrid2 = testGrid
								bestTopOffX2 = tOffX
								bestBotOffX2 = bOffX
								bestTopScaleX2 = tScaleX
								bestBotScaleX2 = bScaleX
								bestOffY2 = oY
								bestScaleY2 = sY
							}
						}
					}
				}
			}
		}
	}
	fmt.Printf("test2-2 最適パラメータ: TopOffX=%.1f, BotOffX=%.1f, TopScaleX=%.2f, BotScaleX=%.2f, OffY=%.1f, ScaleY=%.2f (暫定Acc: %.2f%%)\n",
		bestTopOffX2, bestBotOffX2, bestTopScaleX2, bestBotScaleX2, bestOffY2, bestScaleY2, bestAcc2)

	if bestAcc2 > acc2Init {
		saveWarpedWithGrid(warped2, bestGrid2, "test2-2_optimized_grid.jpg")
		acc2Opt := evaluateGridPoints(warped2, gt, "test2-2.jpg (最適化アライメント)", bestGrid2)
		fmt.Printf("\ntest2-2.jpg 最適化後正解率: %.2f%%\n", acc2Opt)
	}
}
