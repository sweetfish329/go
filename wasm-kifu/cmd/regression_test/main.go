// Package main はリポジトリ内の全対局画像・SGF正解データに対する
// 画像認識精度の回帰テスト（リグレッションテスト）を一括実行するランナーツール。
//
// 2種類のパイプラインをサポート:
//   - デフォルト    : NormR/NormB型 (test3-1, test2-1 など)
//   - EvalImagePipeline: NormL型 (test_image.jpg 専用)
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
	"strings"
	"time"
)

// ─────────────────── 共通型 ───────────────────

type Point struct{ X, Y float64 }

type GroundTruth struct {
	BoardSize int          `json:"boardSize"`
	Grid      [19][19]int8 `json:"grid"`
}

// ─────────────────── テストケース定義 ───────────────────

type TestCase struct {
	// 識別情報
	ID   string
	Name string
	// ファイルパス
	ImagePath string
	GTPath    string
	// 合格基準
	MinAccuracy float64

	// --- パイプライン選択 ---
	// EvalImagePipeline=true のとき NormL型パイプラインを使用
	EvalImagePipeline bool

	// --- NormR/NormB型パイプライン共通パラメータ ---
	Corners    [4]Point
	DstW       int
	DstH       int
	Margin     float64
	PitchStart float64 // ピッチ勾配開始値 (行0→18)
	PitchEnd   float64
	TopOffX    float64
	BotOffX    float64
	TopOffY    float64
	BotOffY    float64
	TopScaleX  float64
	BotScaleX  float64
	ScaleY     float64
	OffY       float64
}

// ─────────────────── 結果型 ───────────────────

type TestResult struct {
	ID          string
	Name        string
	Accuracy    float64
	MinAccuracy float64
	Passed      bool
	Matches     int
	Total       int
	BlackRec    float64
	BlackPrec   float64
	WhiteRec    float64
	WhitePrec   float64
	EmptyRec    float64
	EmptyPrec   float64
	Duration    time.Duration
}

// ─────────────────── ユーティリティ ───────────────────

func findFile(filename string) string {
	cands := []string{
		filename,
		filepath.Join("..", filename),
		filepath.Join("../..", filename),
		filepath.Join("wasm-kifu", filename),
	}
	for _, c := range cands {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return filename
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

func solveLinear(a [8][8]float64, b [8]float64) [8]float64 {
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
				f := a[k][i]
				for j := i; j < 8; j++ {
					a[k][j] -= f * a[i][j]
				}
				b[k] -= f * b[i]
			}
		}
	}
	for i := 0; i < 8; i++ {
		x[i] = b[i]
	}
	return x
}

func perspMat(src, dst [4]Point) [9]float64 {
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
	h := solveLinear(a, b)
	return [9]float64{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1.0}
}

func xformPt(m [9]float64, u, v float64) (float64, float64) {
	d := m[6]*u + m[7]*v + m[8]
	if math.Abs(d) < 1e-9 {
		d = 1e-9
	}
	return (m[0]*u + m[1]*v + m[2]) / d, (m[3]*u + m[4]*v + m[5]) / d
}

func bilinear(img *image.RGBA, x, y float64) color.RGBA {
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
	lerp := func(a, b, t float64) float64 { return a*(1-t) + b*t }
	interp := func(v00, v10, v01, v11 uint8) uint8 {
		return uint8(lerp(lerp(float64(v00), float64(v10), fx),
			lerp(float64(v01), float64(v11), fx), fy))
	}
	return color.RGBA{
		R: interp(c00.R, c10.R, c01.R, c11.R),
		G: interp(c00.G, c10.G, c01.G, c11.G),
		B: interp(c00.B, c10.B, c01.B, c11.B),
		A: 255,
	}
}

func warpWithMargin(src *image.RGBA, corners [4]Point, dstW, dstH int, margin float64) *image.RGBA {
	dc := [4]Point{
		{margin, margin},
		{float64(dstW) - margin, margin},
		{float64(dstW) - margin, float64(dstH) - margin},
		{margin, float64(dstH) - margin},
	}
	inv := perspMat(dc, corners)
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			sx, sy := xformPt(inv, float64(x), float64(y))
			dst.SetRGBA(x, y, bilinear(src, sx, sy))
		}
	}
	return dst
}

func loadGT(path string) (GroundTruth, error) {
	var gt GroundTruth
	data, err := os.ReadFile(findFile(path))
	if err != nil {
		return gt, err
	}
	return gt, json.Unmarshal(data, &gt)
}

func confusion3Metrics(conf [3][3]int, cIdx int) (rec, prec float64) {
	tp := conf[cIdx][cIdx]
	act := conf[cIdx][0] + conf[cIdx][1] + conf[cIdx][2]
	pred := conf[0][cIdx] + conf[1][cIdx] + conf[2][cIdx]
	if act > 0 {
		rec = float64(tp) / float64(act) * 100
	}
	if pred > 0 {
		prec = float64(tp) / float64(pred) * 100
	}
	return
}

// ─────────────────── Pipeline A: NormR/NormB型 ───────────────────

func runNormRBPipeline(tc TestCase, gt GroundTruth) (*TestResult, error) {
	start := time.Now()

	img, err := loadUpright(findFile(tc.ImagePath))
	if err != nil {
		return nil, err
	}

	dstW, dstH := tc.DstW, tc.DstH
	margin := tc.Margin
	warped := warpWithMargin(img, tc.Corners, dstW, dstH, margin)

	boardW := float64(dstW) - 2*margin
	boardH := float64(dstH) - 2*margin

	// ピッチ勾配 Y
	ps, pe := tc.PitchStart, tc.PitchEnd
	if ps == 0 && pe == 0 {
		ps, pe = 1, 1
	}
	rowW := make([]float64, 18)
	for i := range rowW {
		rowW[i] = ps + (pe-ps)*(float64(i)/17)
	}
	totalW := 0.0
	for _, v := range rowW {
		totalW += v
	}
	baseY := make([]float64, 19)
	acc := 0.0
	for i := 0; i < 18; i++ {
		acc += (rowW[i] / totalW) * boardH
		baseY[i+1] = acc
	}
	baseY[18] = boardH

	// グリッド点生成
	type pd struct {
		cx, cy                 int
		R, G, B                float64
		RMB, NormR, NormB      float64
		LocalBgR, LocalBgB     float64
	}
	var data [19][19]pd
	sR := 5

	for y := 0; y < 19; y++ {
		tY := float64(y) / 18
		offX := tc.TopOffX + (tc.BotOffX-tc.TopOffX)*tY
		offY := tc.TopOffY + (tc.BotOffY-tc.TopOffY)*tY
		scX := tc.TopScaleX + (tc.BotScaleX-tc.TopScaleX)*tY
		scY := tc.ScaleY
		if scY == 0 {
			scY = 1
		}

		var cy float64
		if tc.TopOffY != 0 || tc.BotOffY != 0 {
			// eval_test3 互換: cy = margin + baseY[y] + currOffY (ピッチ中央寄せなし)
			cy = margin + baseY[y] + offY + tc.OffY
		} else {
			// eval_test2 型: cy = margin + center + (base - center) * scaleY + offY
			cy = margin + (boardH/2) + (baseY[y]-(boardH/2))*scY + tc.OffY + offY
		}

		for x := 0; x < 19; x++ {
			tX := float64(x) / 18
			cx := margin + (boardW/2) + (tX*boardW-(boardW/2))*scX + offX

			var sR2, sG2, sB2, cnt float64
			for dy := -sR; dy <= sR; dy++ {
				for dx := -sR; dx <= sR; dx++ {
					if dx*dx+dy*dy <= sR*sR {
						px, py := int(cx)+dx, int(cy)+dy
						if px >= 0 && px < dstW && py >= 0 && py < dstH {
							c := warped.RGBAAt(px, py)
							sR2 += float64(c.R)
							sG2 += float64(c.G)
							sB2 += float64(c.B)
							cnt++
						}
					}
				}
			}
			r, g, b := 80.0, 60.0, 40.0
			if cnt > 0 {
				r = sR2 / cnt
				g = sG2 / cnt
				b = sB2 / cnt
			}
			_ = g
			data[y][x] = pd{cx: int(cx), cy: int(cy), R: r, B: b, RMB: r - b}
		}
	}

	// 局所背景適応
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			var rL, bL []float64
			for dy := -3; dy <= 3; dy++ {
				for dx := -3; dx <= 3; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < 19 && ny >= 0 && ny < 19 {
						rL = append(rL, data[ny][nx].R)
						bL = append(bL, data[ny][nx].B)
					}
				}
			}
			sort.Float64s(rL)
			sort.Float64s(bL)
			bgR := rL[len(rL)/2]
			bgB := bL[len(bL)/2]
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

	// 石判定
	var detected [19][19]int8
	matches := 0
	var conf [3][3]int
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			d := data[y][x]
			stone := int8(0)
			// 白石
			if d.RMB < 32 && d.R >= 85 && (d.NormB > 1.12 || (d.NormR > 1.04 && d.RMB < 20)) {
				stone = 2
			} else if d.R >= 135 && d.RMB <= 22 {
				stone = 2
			}
			// 黒石
			if stone == 0 && d.RMB < 35 {
				if d.R < 75 && (d.NormR < 0.90 || d.NormB < 0.90) {
					stone = 1
				} else if d.NormB < 0.82 && d.NormR < 0.88 {
					stone = 1
				} else if d.B < 45 && d.R < 75 && d.RMB < 25 {
					stone = 1
				}
			}
			detected[y][x] = stone
			a := gt.Grid[y][x]
			conf[a][stone]++
			if a == stone {
				matches++
			}
		}
	}

	bR, bP := confusion3Metrics(conf, 1)
	wR, wP := confusion3Metrics(conf, 2)
	eR, eP := confusion3Metrics(conf, 0)
	acc2 := float64(matches) / 361 * 100

	return &TestResult{
		ID: tc.ID, Name: tc.Name,
		Accuracy: acc2, MinAccuracy: tc.MinAccuracy, Passed: acc2 >= tc.MinAccuracy,
		Matches: matches, Total: 361,
		BlackRec: bR, BlackPrec: bP,
		WhiteRec: wR, WhitePrec: wP,
		EmptyRec: eR, EmptyPrec: eP,
		Duration: time.Since(start),
	}, nil
}

// ─────────────────── Pipeline B: NormL型 (eval_image 互換) ───────────────────

func runNormLPipeline(tc TestCase, gt GroundTruth) (*TestResult, error) {
	start := time.Now()

	// loadUpright で EXIF Orientation 6 補正
	img, err := loadUpright(findFile(tc.ImagePath))
	if err != nil {
		return nil, err
	}

	// 570x570 正方形ワープ (eval_image と同一)
	const dstSize = 570.0
	srcCorners := tc.Corners
	dstCorners := [4]Point{{0, 0}, {dstSize, 0}, {dstSize, dstSize}, {0, dstSize}}
	inv := perspMat(dstCorners, srcCorners)
	warped := image.NewRGBA(image.Rect(0, 0, int(dstSize), int(dstSize)))
	for py := 0; py < int(dstSize); py++ {
		for px := 0; px < int(dstSize); px++ {
			sx, sy := xformPt(inv, float64(px), float64(py))
			warped.SetRGBA(px, py, bilinear(img, sx, sy))
		}
	}

	gridSpacing := dstSize / 18.0

	type pd2 struct {
		X, Y    int
		L, C, S float64
		R, G, B float64
		RMB     float64
		LocalBg float64
		NormL   float64
	}
	var data [19][19]pd2

	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			cx := float64(x) * gridSpacing
			cy := float64(y) * gridSpacing

			minDx, maxDx := -6.0, 6.0
			minDy, maxDy := -6.0, 6.0
			if x == 0 {
				minDx = 0
			}
			if x == 18 {
				maxDx = 0
			}
			if y == 0 {
				minDy = 0
			}
			if y == 18 {
				maxDy = 0
			}

			var tR, tG, tB, cnt float64
			for dy := minDy; dy <= maxDy; dy++ {
				for dx := minDx; dx <= maxDx; dx++ {
					if dx*dx+dy*dy <= 30 {
						px := int(cx + dx + 0.5)
						py := int(cy + dy + 0.5)
						if px >= 0 && px < int(dstSize) && py >= 0 && py < int(dstSize) {
							c := warped.RGBAAt(px, py)
							tR += float64(c.R)
							tG += float64(c.G)
							tB += float64(c.B)
							cnt++
						}
					}
				}
			}

			r, g, b := tR/cnt, tG/cnt, tB/cnt
			l := 0.299*r + 0.587*g + 0.114*b
			m := (r + g + b) / 3
			c := math.Sqrt(((r-m)*(r-m) + (g-m)*(g-m) + (b-m)*(b-m)) / 3)
			maxV := math.Max(r, math.Max(g, b))
			minV := math.Min(r, math.Min(g, b))
			s := 0.0
			if maxV > 0 {
				s = (maxV - minV) / maxV * 255
			}
			data[y][x] = pd2{X: x, Y: y, L: l, C: c, S: s, R: r, G: g, B: b, RMB: r - b}
		}
	}

	// 局所背景 (L の中央値、近傍 2 交点)
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			var nl []float64
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < 19 && ny >= 0 && ny < 19 {
						nl = append(nl, data[ny][nx].L)
					}
				}
			}
			sort.Float64s(nl)
			bg := nl[len(nl)/2]
			if bg < 10 {
				bg = 10
			}
			data[y][x].LocalBg = bg
			data[y][x].NormL = data[y][x].L / bg
		}
	}

	// 石判定 (eval_image と同一ロジック)
	var detected [19][19]int8
	matches := 0
	var conf [3][3]int

	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			d := data[y][x]
			isBorder := d.X == 0 || d.X == 18 || d.Y == 0 || d.Y == 18
			stone := int8(0)

			// 境界保護（eval_image と同一）
			if d.X == 0 || (d.Y == 0 && d.L < 100) || (d.Y == 18 && d.L < 100) {
				detected[y][x] = 0
				a := gt.Grid[y][x]
				conf[a][0]++
				if a == 0 {
					matches++
				}
				continue
			}

			// 白石
			if d.RMB < 30 && d.C < 25 {
				if d.L > 160 || (d.NormL > 1.20 && d.L > 120) {
					stone = 2
				} else if d.RMB < 10 && d.NormL > 1.05 && d.L > 105 {
					stone = 2
				}
			}
			if stone == 0 && d.L > 185 && d.S < 85 && d.RMB < 35 {
				stone = 2
			}
			// 黒石
			if stone == 0 {
				if !isBorder && d.RMB < 20 && d.C < 16 {
					if d.L < 75 || d.NormL < 0.72 {
						stone = 1
					} else if d.L < 95 && d.C < 8 && d.RMB < 10 {
						stone = 1
					}
				}
				if stone == 0 && !isBorder && d.NormL < 0.60 && d.C < 18 && d.RMB < 25 {
					stone = 1
				}
			}

			detected[y][x] = stone
			a := gt.Grid[y][x]
			conf[a][stone]++
			if a == stone {
				matches++
			}
		}
	}

	bR, bP := confusion3Metrics(conf, 1)
	wR, wP := confusion3Metrics(conf, 2)
	eR, eP := confusion3Metrics(conf, 0)
	acc := float64(matches) / 361 * 100

	return &TestResult{
		ID: tc.ID, Name: tc.Name,
		Accuracy: acc, MinAccuracy: tc.MinAccuracy, Passed: acc >= tc.MinAccuracy,
		Matches: matches, Total: 361,
		BlackRec: bR, BlackPrec: bP,
		WhiteRec: wR, WhitePrec: wP,
		EmptyRec: eR, EmptyPrec: eP,
		Duration: time.Since(start),
	}, nil
}

// ─────────────────── ディスパッチャ ───────────────────

func runTestCase(tc TestCase) (*TestResult, error) {
	gt, err := loadGT(tc.GTPath)
	if err != nil {
		return nil, fmt.Errorf("GT load error [%s]: %w", tc.ID, err)
	}
	if tc.EvalImagePipeline {
		return runNormLPipeline(tc, gt)
	}
	return runNormRBPipeline(tc, gt)
}

// ─────────────────── テストケース定義テーブル ───────────────────

var testCases = []TestCase{
	// ----------------------------------------------------------------
	// TEST-3-1: test3-1.jpg  王座戦予選 204手 通常俯角正面視点
	// 目標 94.46% (make eval-test3)  本ランナー基準 >=92.0%
	// ※ 正式な最高精度確認は make eval-test3 を使用すること
	// ----------------------------------------------------------------
	{
		ID:          "TEST-3-1",
		Name:        "test3-1.jpg (王座戦予選 204手 正面俯角)",
		ImagePath:   "test3-1.jpg",
		GTPath:      "test3_ground_truth.json",
		Corners:     [4]Point{{465, 555}, {1310, 650}, {1235, 1715}, {100, 1440}},
		DstW:        680,
		DstH:        730,
		Margin:      40.0,
		PitchStart:  33.0,
		PitchEnd:    39.0,
		TopOffX:     44.0,
		BotOffX:     -12.0,
		TopOffY:     30.0,
		BotOffY:     30.0,
		TopScaleX:   1.00,
		BotScaleX:   1.06,
		ScaleY:      1.0,
		MinAccuracy: 92.0,
	},
	// ----------------------------------------------------------------
	// TEST-2-1: test2-1.jpg  第14期竜星戦 282手 正面視点
	// 目標 92.24%  基準 >=90.0%
	// ----------------------------------------------------------------
	{
		ID:          "TEST-2-1",
		Name:        "test2-1.jpg (竜星戦 282手 正面)",
		ImagePath:   "test2-1.jpg",
		GTPath:      "test2_ground_truth.json",
		Corners:     [4]Point{{456, 696}, {1582, 696}, {1949, 1848}, {101, 1860}},
		DstW:        680,
		DstH:        730,
		Margin:      40.0,
		PitchStart:  33.0,
		PitchEnd:    39.0,
		TopOffX:     0.0,
		BotOffX:     4.0,
		TopOffY:     0.0,
		BotOffY:     0.0,
		TopScaleX:   1.02,
		BotScaleX:   1.00,
		ScaleY:      1.04,
		OffY:        8.0,
		MinAccuracy: 90.0,
	},
	// ----------------------------------------------------------------
	// TEST-IMG: test_image.jpg  初期テスト画像 263手
	// eval_image と同一 NormL パイプライン  目標 92.52%  基準 >=90.0%
	// ----------------------------------------------------------------
	{
		ID:                "TEST-IMG",
		Name:              "test_image.jpg (初期テスト 263手 正面)",
		ImagePath:         "test_image.jpg",
		GTPath:            "test_ground_truth.json",
		EvalImagePipeline: true,
		// eval_image/main.go の srcCorners と同一値
		Corners:     [4]Point{{448, 545}, {1588, 595}, {1552, 1955}, {112, 1695}},
		MinAccuracy: 90.0,
	},
}

// ─────────────────── main ───────────────────

func main() {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("  Go Kifu Vision ─ 全自動リグレッションテストスイート")
	fmt.Printf("  実行日時: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 80))

	allPassed := true
	var results []*TestResult

	for i, tc := range testCases {
		fmt.Printf("\n[%d/%d] ▶ %s\n", i+1, len(testCases), tc.Name)
		res, err := runTestCase(tc)
		if err != nil {
			fmt.Printf("  ❌ エラー: %v\n", err)
			allPassed = false
			continue
		}
		results = append(results, res)

		mark := "✅ PASS"
		if !res.Passed {
			mark = "❌ FAIL  ← リグレッション検知"
			allPassed = false
		}
		fmt.Printf("  %s | %5.2f%% (基準 >=%4.1f%%) | %d/361 | %v\n",
			mark, res.Accuracy, res.MinAccuracy, res.Matches, res.Duration.Round(time.Millisecond))
		fmt.Printf("       黒: Rec %5.1f%%, Prec %5.1f%%  |  白: Rec %5.1f%%, Prec %5.1f%%  |  空: Rec %5.1f%%\n",
			res.BlackRec, res.BlackPrec, res.WhiteRec, res.WhitePrec, res.EmptyRec)
	}

	// ─── サマリー ───
	fmt.Printf("\n%s\n", strings.Repeat("─", 80))
	fmt.Println("  総合サマリー")
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("%-12s  %-40s  %9s  %9s  %6s\n", "ID", "テストケース", "正解率", "基準", "判定")
	fmt.Println(strings.Repeat("─", 80))
	for _, r := range results {
		judge := "PASS"
		if !r.Passed {
			judge = "FAIL ⚠"
		}
		fmt.Printf("%-12s  %-40s  %6.2f%%    >=%4.1f%%  %s\n",
			r.ID, r.Name, r.Accuracy, r.MinAccuracy, judge)
	}
	fmt.Println(strings.Repeat("─", 80))

	if allPassed {
		fmt.Println("🎉 全テストが基準をクリア。リグレッションなし。")
		os.Exit(0)
	} else {
		fmt.Println("⚠️  一部テストで精度低下（リグレッション）を検知しました。")
		os.Exit(1)
	}
}
