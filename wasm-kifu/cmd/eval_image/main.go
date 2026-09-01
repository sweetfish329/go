package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"math"
	"os"
	"sort"
)

type Point struct {
	X, Y float64
}

// 射影変換行列の計算 (Direct Linear Transform)
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

func transformPoint(m [9]float64, u, v float64) (float64, float64) {
	denom := m[6]*u + m[7]*v + m[8]
	if math.Abs(denom) < 1e-9 {
		denom = 1e-9
	}
	return (m[0]*u + m[1]*v + m[2]) / denom, (m[3]*u + m[4]*v + m[5]) / denom
}

func getPixelBilinear(img image.Image, x, y float64) (r, g, b float64) {
	bounds := img.Bounds()
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	if ix < bounds.Min.X || ix+1 >= bounds.Max.X || iy < bounds.Min.Y || iy+1 >= bounds.Max.Y {
		c := img.At(int(x), int(y))
		r1, g1, b1, _ := c.RGBA()
		return float64(r1 >> 8), float64(g1 >> 8), float64(b1 >> 8)
	}
	fx, fy := x-float64(ix), y-float64(iy)
	r00, g00, b00, _ := img.At(ix, iy).RGBA()
	r10, g10, b10, _ := img.At(ix+1, iy).RGBA()
	r01, g01, b01, _ := img.At(ix, iy).RGBA()
	r11, g11, b11, _ := img.At(ix+1, iy+1).RGBA()
	p00 := [3]float64{float64(r00 >> 8), float64(g00 >> 8), float64(b00 >> 8)}
	p10 := [3]float64{float64(r10 >> 8), float64(g10 >> 8), float64(b10 >> 8)}
	p01 := [3]float64{float64(r01 >> 8), float64(g01 >> 8), float64(b01 >> 8)}
	p11 := [3]float64{float64(r11 >> 8), float64(g11 >> 8), float64(b11 >> 8)}
	var res [3]float64
	for i := 0; i < 3; i++ {
		top := p00[i]*(1-fx) + p10[i]*fx
		bot := p01[i]*(1-fx) + p11[i]*fx
		res[i] = top*(1-fy) + bot*fy
	}
	return res[0], res[1], res[2]
}

type Feature struct {
	X, Y        int
	L, C, D, S  float64
	LocalMeanL  float64
	NormL       float64
}

type GroundTruth struct {
	BoardSize int          `json:"boardSize"`
	Grid      [19][19]int8 `json:"grid"`
}

func main() {
	filePath := "test_image.jpg"
	if _, err := os.Stat(filePath); err != nil {
		filePath = "../test_image.jpg"
		if _, err := os.Stat(filePath); err != nil {
			filePath = "../../test_image.jpg"
		}
	}
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Failed to open %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	rawImg, _, err := image.Decode(file)
	if err != nil {
		fmt.Printf("Failed to decode image: %v\n", err)
		return
	}
	rawBounds := rawImg.Bounds()
	rw, rh := rawBounds.Dx(), rawBounds.Dy()

	// 正立画像 (1814 x 2418) を作成 (EXIF Orientation 6: 時計回り90度回転)
	upright := image.NewRGBA(image.Rect(0, 0, rh, rw))
	for y := 0; y < rh; y++ {
		for x := 0; x < rw; x++ {
			upright.Set(rh-1-y, x, rawImg.At(x, y))
		}
	}

	// 外枠格子線4隅 (正立座標)
	// TL(A19), TR(T19), BR(T1), BL(A1)
	srcCorners := [4]Point{
		{X: 448, Y: 545},
		{X: 1588, Y: 595},
		{X: 1552, Y: 1955},
		{X: 112, Y: 1695},
	}

	dstSize := 570.0
	dstCorners := [4]Point{
		{X: 0, Y: 0},
		{X: dstSize, Y: 0},
		{X: dstSize, Y: dstSize},
		{X: 0, Y: dstSize},
	}

	invMat := getPerspectiveMatrix(dstCorners, srcCorners)
	gridSpacing := dstSize / 18.0

	warped := image.NewRGBA(image.Rect(0, 0, int(dstSize), int(dstSize)))
	for py := 0; py < int(dstSize); py++ {
		for px := 0; px < int(dstSize); px++ {
			sx, sy := transformPoint(invMat, float64(px), float64(py))
			r, g, b := getPixelBilinear(upright, sx, sy)
			warped.Set(px, py, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255})
		}
	}

	// 各交点の特徴量抽出 (最外周の内向きサンプリング + 木目度 R-B)
	type PointData struct {
		X, Y    int
		L, C, S float64
		R, G, B float64
		RMB     float64
		LocalBg float64
		NormL   float64
	}

	data := [19][19]PointData{}

	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			cx := float64(x) * gridSpacing
			cy := float64(y) * gridSpacing

			minDx, maxDx := -6.0, 6.0
			minDy, maxDy := -6.0, 6.0
			if x == 0 {
				minDx = 0.0
			}
			if x == 18 {
				maxDx = 0.0
			}
			if y == 0 {
				minDy = 0.0
			}
			if y == 18 {
				maxDy = 0.0
			}

			var totalR, totalG, totalB, count float64
			for dy := minDy; dy <= maxDy; dy += 1.0 {
				for dx := minDx; dx <= maxDx; dx += 1.0 {
					d2 := dx*dx + dy*dy
					if d2 <= 30.0 {
						px := int(cx + dx + 0.5)
						py := int(cy + dy + 0.5)
						if px >= 0 && px < 570 && py >= 0 && py < 570 {
							r, g, b := getPixelBilinear(warped, float64(px), float64(py))
							totalR += r
							totalG += g
							totalB += b
							count++
						}
					}
				}
			}

			r := totalR / count
			g := totalG / count
			b := totalB / count
			l := 0.299*r + 0.587*g + 0.114*b
			m := (r + g + b) / 3.0
			c := math.Sqrt(((r-m)*(r-m) + (g-m)*(g-m) + (b-m)*(b-m)) / 3.0)
			maxV := math.Max(r, math.Max(g, b))
			minV := math.Min(r, math.Min(g, b))
			s := 0.0
			if maxV > 0 {
				s = ((maxV - minV) / maxV) * 255.0
			}

			data[y][x] = PointData{
				X:   x,
				Y:   y,
				L:   l,
				C:   c,
				S:   s,
				R:   r,
				G:   g,
				B:   b,
				RMB: r - b,
			}
		}
	}

	// 局所背景適応 (近傍2交点以内の L の中央値)
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			var nList []float64
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < 19 && ny >= 0 && ny < 19 {
						nList = append(nList, data[ny][nx].L)
					}
				}
			}
			sort.Float64s(nList)
			bg := nList[len(nList)/2]
			if bg < 10 {
				bg = 10
			}
			data[y][x].LocalBg = bg
			data[y][x].NormL = data[y][x].L / bg
		}
	}

	// 高精度アンサンブル石判定 (0=空, 1=黒, 2=白)
	detected := [19][19]int8{}
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			d := data[y][x]
			isBorder := (d.X == 0 || d.X == 18 || d.Y == 0 || d.Y == 18)

			stone := int8(0)

			// 盤外・境界保護
			if d.X == 0 || (d.Y == 0 && d.L < 100.0) || (d.Y == 18 && d.L < 100.0) {
				detected[y][x] = 0
				continue
			}

			// 白石判定
			if d.RMB < 30.0 && d.C < 25.0 {
				if d.L > 160.0 || (d.NormL > 1.20 && d.L > 120.0) {
					stone = 2
				} else if d.RMB < 10.0 && d.NormL > 1.05 && d.L > 105.0 {
					stone = 2
				}
			}
			if stone == 0 && d.L > 185.0 && d.S < 85.0 && d.RMB < 35.0 {
				stone = 2
			}

			// 黒石判定
			if stone == 0 {
				if !isBorder && d.RMB < 20.0 && d.C < 16.0 {
					if d.L < 75.0 || d.NormL < 0.72 {
						stone = 1
					} else if d.L < 95.0 && d.C < 8.0 && d.RMB < 10.0 {
						stone = 1
					}
				}
				if stone == 0 && !isBorder && d.NormL < 0.60 && d.C < 18.0 && d.RMB < 25.0 {
					stone = 1
				}
			}

			detected[y][x] = stone
		}
	}

	// Ground Truth の読み込み
	gtPath := "test_ground_truth.json"
	if _, err := os.Stat(gtPath); err != nil {
		gtPath = "wasm-kifu/test_ground_truth.json"
	}
	var gt GroundTruth
	hasGT := false
	if gtData, err := os.ReadFile(gtPath); err == nil {
		if err := json.Unmarshal(gtData, &gt); err == nil {
			hasGT = true
		}
	}

	// 精度評価
	fmt.Println("=== 盤面認識評価レポート (Ground Truth 比較) ===")
	cols := "   A B C D E F G H J K L M N O P Q R S T"
	fmt.Println(cols)
	matchCount := 0
	totalCount := 19 * 19
	// 混同行列 [actual][predicted]
	confusion := [3][3]int{}

	for y := 0; y < 19; y++ {
		rowStr := fmt.Sprintf("%2d ", 19-y)
		for x := 0; x < 19; x++ {
			p := detected[y][x]
			if hasGT {
				a := gt.Grid[y][x]
				confusion[a][p]++
				if a == p {
					matchCount++
				}
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

	if hasGT {
		acc := float64(matchCount) / float64(totalCount) * 100.0
		fmt.Printf("\n全361交点 正解率 (Accuracy): %.2f%% (%d / %d)\n", acc, matchCount, totalCount)
		fmt.Println("\n混同行列 (Confusion Matrix):")
		fmt.Println("             [予測:空] [予測:黒] [予測:白]")
		fmt.Printf("[正解:空(0)]   %4d     %4d     %4d\n", confusion[0][0], confusion[0][1], confusion[0][2])
		fmt.Printf("[正解:黒(1)]   %4d     %4d     %4d\n", confusion[1][0], confusion[1][1], confusion[1][2])
		fmt.Printf("[正解:白(2)]   %4d     %4d     %4d\n", confusion[2][0], confusion[2][1], confusion[2][2])

		// 各クラスの Recall / Precision
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

		// 誤判定された箇所の詳細リスト
		fmt.Println("\n--- 主な誤判定交点 (Top 10) ---")
		errCount := 0
		colNames := []string{"A", "B", "C", "D", "E", "F", "G", "H", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T"}
		typeNames := []string{"空", "黒", "白"}
		for y := 0; y < 19 && errCount < 10; y++ {
			for x := 0; x < 19 && errCount < 10; x++ {
				a := gt.Grid[y][x]
				p := detected[y][x]
				if a != p {
					d := data[y][x]
					fmt.Printf("%s%d (%d,%d): 正解=%s, 予測=%s | L=%.1f (Bg=%.1f, Norm=%.2f), C=%.1f, RMB=%.1f, S=%.1f\n",
						colNames[x], 19-y, x, y, typeNames[a], typeNames[p], d.L, d.LocalBg, d.NormL, d.C, d.RMB, d.S)
					errCount++
				}
			}
		}
	}
}
