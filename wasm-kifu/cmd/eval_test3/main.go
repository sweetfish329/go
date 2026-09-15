// Package main は test3-1.jpg, test3-2.jpg (回転撮影), test3.sgf を用いた
// 盤面画像認識の精度検証および回転耐性テストを実行する評価ツール。
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
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

// loadUpright は EXIF Orientation 6 (時計回り90度回転) を正立画像 (rh x rw) としてロードする
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

// rotate90CW は画像を時計回りに90度回転する
func rotate90CW(img *image.RGBA) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, img.At(x, y))
		}
	}
	return dst
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

func warpPerspective(src *image.RGBA, srcCorners [4]Point, dstW, dstH int) *image.RGBA {
	dstCorners := [4]Point{
		{X: 0, Y: 0},
		{X: float64(dstW), Y: 0},
		{X: float64(dstW), Y: float64(dstH)},
		{X: 0, Y: float64(dstH)},
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

type PointData struct {
	X, Y                    int
	R, G, B, L              float64
	RMB, NormR, NormB       float64
	LocalBgR, LocalBgB      float64
}

// evaluateWarped は補正済みワープ画像に対して全361交点の石判定と精度評価を実行する
func evaluateWarped(warped *image.RGBA, gt GroundTruth, testLabel string, gridX, gridY []float64, dxShift, dyShift int) float64 {
	dstW := warped.Bounds().Dx()
	dstH := warped.Bounds().Dy()

	var data [19][19]PointData
	sampleR := 5

	for y := 0; y < 19; y++ {
		cy := int(gridY[y] + 0.5)
		for x := 0; x < 19; x++ {
			cx := int(gridX[x] + 0.5)

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

			r := sumR / count
			g := sumG / count
			b := sumB / count
			l := 0.299*r + 0.587*g + 0.114*b

			data[y][x] = PointData{
				X:   x,
				Y:   y,
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
	var rawDetected [19][19]int8
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

			rawDetected[y][x] = stone
		}
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("   【%s】盤面認識評価レポート\n", testLabel)
	fmt.Printf("=======================================================\n")
	cols := "   A B C D E F G H J K L M N O P Q R S T"
	fmt.Println(cols)
	matchCount := 0
	confusion := [3][3]int{}

	var detected [19][19]int8
	for y := 0; y < 19; y++ {
		rowStr := fmt.Sprintf("%2d ", 19-y)
		for x := 0; x < 19; x++ {
			srcY := y + dyShift
			srcX := x + dxShift
			p := int8(0)
			if srcY >= 0 && srcY < 19 && srcX >= 0 && srcX < 19 {
				p = rawDetected[srcY][srcX]
			}
			detected[y][x] = p
			a := gt.Grid[y][x]
			confusion[a][p]++
			if a == p {
				matchCount++
			}
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
				srcY := y + dyShift
				srcX := x + dxShift
				if srcY >= 0 && srcY < 19 && srcX >= 0 && srcX < 19 {
					d := data[srcY][srcX]
					fmt.Printf("%s%d (%d,%d): 正解=%s, 予測=%s | R=%.1f, B=%.1f, RMB=%.1f, NormR=%.2f, NormB=%.2f\n",
						colNames[x], 19-y, x, y, typeNames[a], typeNames[p], d.R, d.B, d.RMB, d.NormR, d.NormB)
					errCount++
				}
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
	gtFile := findFile("test3_ground_truth.json")
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

	dstW := 600
	dstH := 646

	// -------------------------------------------------------------
	// 1. test3-1.jpg (通常視点)
	// -------------------------------------------------------------
	img1Path := findFile("test3-1.jpg")
	img1, err := loadUpright(img1Path)
	if err != nil {
		fmt.Printf("Error loading %s: %v\n", img1Path, err)
		return
	}
	corners1 := [4]Point{
		{X: 465, Y: 555},  // A19
		{X: 1310, Y: 650}, // T19
		{X: 1235, Y: 1715},// T1
		{X: 100, Y: 1440},  // A1
	}
	warped1 := warpPerspective(img1, corners1, dstW, dstH)

	rowWeights1 := make([]float64, 18)
	for i := 0; i < 18; i++ {
		rowWeights1[i] = 33.0 + (39.0-33.0)*(float64(i)/17.0)
	}
	totalW1 := 0.0
	for _, w := range rowWeights1 {
		totalW1 += w
	}
	gridY1 := make([]float64, 19)
	accY1 := 0.0
	for i := 0; i < 18; i++ {
		accY1 += (rowWeights1[i] / totalW1) * float64(dstH-1)
		gridY1[i+1] = accY1
	}
	gridY1[18] = float64(dstH - 1)

	stepX1 := float64(dstW-1) / 18.0
	gridX1 := make([]float64, 19)
	for i := 0; i < 19; i++ {
		gridX1[i] = float64(i) * stepX1
	}

	// ベベルマージンオフセット (dx=1, dy=1) を適用
	acc1 := evaluateWarped(warped1, gt, "test3-1.jpg (通常視点)", gridX1, gridY1, 1, 1)

	// -------------------------------------------------------------
	// 2. test3-2.jpg (回転撮影: 東から西を見る角度で撮影)
	// -------------------------------------------------------------
	img2Path := findFile("test3-2.jpg")
	img2, err := loadUpright(img2Path)
	if err != nil {
		fmt.Printf("Error loading %s: %v\n", img2Path, err)
		return
	}

	// 270度時計回り回転 (反時計回り90度回転) で正立向きに補正
	rot270 := rotate90CW(rotate90CW(rotate90CW(img2)))
	corners2 := [4]Point{
		{X: 530, Y: 170},  // A19
		{X: 1440, Y: 120}, // T19
		{X: 1670, Y: 1420},// T1
		{X: 420, Y: 1490}, // A1
	}
	warped2 := warpPerspective(rot270, corners2, dstW, dstH)

	colWeights2 := make([]float64, 18)
	for i := 0; i < 18; i++ {
		colWeights2[i] = 28.0 + (38.0-28.0)*(float64(i)/17.0)
	}
	totalW2 := 0.0
	for _, w := range colWeights2 {
		totalW2 += w
	}
	gridX2 := make([]float64, 19)
	accX2 := 0.0
	for i := 0; i < 18; i++ {
		accX2 += (colWeights2[i] / totalW2) * float64(dstW-1)
		gridX2[i+1] = accX2
	}
	gridX2[18] = float64(dstW - 1)

	stepY2 := float64(dstH-1) / 18.0
	gridY2 := make([]float64, 19)
	for i := 0; i < 19; i++ {
		gridY2[i] = float64(i) * stepY2
	}

	// test3-2 におけるマージンオフセット (dx=-2, dy=1)
	acc2 := evaluateWarped(warped2, gt, "test3-2.jpg (回転撮影・270°補正)", gridX2, gridY2, -2, 1)

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("   総合評価サマリー\n")
	fmt.Printf("=======================================================\n")
	fmt.Printf("test3-1.jpg (通常視点):         正解率 %.2f%%\n", acc1)
	fmt.Printf("test3-2.jpg (回転撮影・270°補正): 正解率 %.2f%%\n", acc2)
}
