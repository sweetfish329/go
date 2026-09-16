// Package main はリポジトリ内の全対局画像・SGF正解データに対する
// 画像認識精度の回帰テスト（リグレッションテスト）を一括実行するランナーツール。
//
// パイプラインは NormR/NormB 型（チャンネル分割）に統一済み。
// NormL 型（eval_image 旧パイプライン）との後方互換は不要。
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

// TestCase は 1 件の認識精度テストを表す。
// パイプラインは NormR/NormB 型（チャンネル分割 + ピッチ勾配 + 台形補正）に統一。
type TestCase struct {
	ID          string
	Name        string
	ImagePath   string
	GTPath      string
	MinAccuracy float64

	// ワープ設定
	Corners    [4]Point // TL(A19), TR(T19), BR(T1), BL(A1)  正立座標
	DstW       int
	DstH       int
	Margin     float64

	// ピッチ勾配 (行 0→18 の格子間隔を線形補間)
	PitchStart float64
	PitchEnd   float64

	// 台形シアー補正
	TopOffX   float64
	BotOffX   float64
	TopOffY   float64
	BotOffY   float64
	TopScaleX float64
	BotScaleX float64

	// グローバル Y オフセット (TopOffY/BotOffY が 0 のとき有効)
	ScaleY float64
	OffY   float64
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

// loadUpright は JPEG を読み込み EXIF Orientation 6（スマホ縦持ち）を正立化する。
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

// ─────────────────── Pipeline: NormR/NormB 型（唯一のパイプライン） ───────────────────

func runTestCase(tc TestCase) (*TestResult, error) {
	gt, err := loadGT(tc.GTPath)
	if err != nil {
		return nil, fmt.Errorf("GT load error [%s]: %w", tc.ID, err)
	}

	start := time.Now()

	img, err := loadUpright(findFile(tc.ImagePath))
	if err != nil {
		return nil, fmt.Errorf("image load error [%s]: %w", tc.ID, err)
	}

	dstW, dstH := tc.DstW, tc.DstH
	margin := tc.Margin
	warped := warpWithMargin(img, tc.Corners, dstW, dstH, margin)

	boardW := float64(dstW) - 2*margin
	boardH := float64(dstH) - 2*margin

	// ピッチ勾配 Y 方向
	ps, pe := tc.PitchStart, tc.PitchEnd
	if ps == 0 && pe == 0 {
		ps, pe = boardH/18, boardH/18
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

	// グリッド点生成 + サンプリング
	type pd struct {
		R, G, B      float64
		Chroma       float64
		NormR, NormB float64
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
			// シアー型: cy = margin + baseY[y] + currOffY
			cy = margin + baseY[y] + offY + tc.OffY
		} else {
			// スケール型: cy = center + (base - center) * scaleY
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
			cr := 0.500*r - 0.418688*g - 0.081312*b + 128.0
			cb := -0.168736*r - 0.331264*g + 0.500*b + 128.0
			chroma := math.Hypot(cr-128.0, cb-128.0)

			data[y][x] = pd{R: r, G: g, B: b, Chroma: chroma}
		}
	}

	// 局所背景適応 (近傍 3 交点の R / B 各中央値)
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
			data[y][x].NormR = data[y][x].R / bgR
			data[y][x].NormB = data[y][x].B / bgB
		}
	}

	// 石判定 (NormR/NormB チャンネル分割 + Chroma無彩色判定)
	matches := 0
	var conf [3][3]int
	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			d := data[y][x]
			rmb := d.R - d.B
			stone := int8(0)

			// 白石
			if rmb < 32 && d.R >= 85 && (d.NormB > 1.12 || (d.NormR > 1.04 && rmb < 20)) {
				stone = 2
			} else if d.R >= 135 && rmb <= 22 {
				stone = 2
			}
			// 黒石 (無彩色制約 Chroma で木目影を排除)
			if stone == 0 && rmb < 35 {
				if d.R < 75 && (d.NormR < 0.90 || d.NormB < 0.90) && (d.Chroma == 0 || d.Chroma < 22.0) {
					stone = 1
				} else if d.NormB < 0.82 && d.NormR < 0.88 && (d.Chroma == 0 || d.Chroma < 18.0) {
					stone = 1
				} else if d.B < 45 && d.R < 75 && rmb < 25 {
					stone = 1
				}
			}

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
	accuracy := float64(matches) / 361 * 100

	return &TestResult{
		ID: tc.ID, Name: tc.Name,
		Accuracy: accuracy, MinAccuracy: tc.MinAccuracy, Passed: accuracy >= tc.MinAccuracy,
		Matches: matches, Total: 361,
		BlackRec: bR, BlackPrec: bP,
		WhiteRec: wR, WhitePrec: wP,
		EmptyRec: eR, EmptyPrec: eP,
		Duration: time.Since(start),
	}, nil
}

// ─────────────────── テストケース定義テーブル ───────────────────

var testCases = []TestCase{
	// ----------------------------------------------------------------
	// TEST-3-1: test3-1.jpg  王座戦予選 204手 通常俯角正面視点
	// 正式最高精度 94.46% (make eval-test3)  本ランナー基準 >=92.0%
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
		MinAccuracy: 92.0,
	},
	// ----------------------------------------------------------------
	// TEST-2-1: test2-1.jpg  第14期竜星戦 282手 正面視点
	// 正式最高精度 92.24%  本ランナー基準 >=90.0%
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
		TopScaleX:   1.02,
		BotScaleX:   1.00,
		ScaleY:      1.04,
		OffY:        8.0,
		MinAccuracy: 90.0,
	},
	// ----------------------------------------------------------------
	// TEST-IMG: test_image.jpg  初期テスト画像 263手 正面視点
	// NormR/NormB 型に統一（旧 NormL 型との後方互換不要）
	// 最適化済みパラメータ: TopOffX=5 BotOffX=-5 TopOffY=20 BotOffY=5
	//   TopScaleX=0.95 BotScaleX=0.97 PitchStart=35 PitchEnd=41
	// 達成精度 90.58%  基準 >=90.0%
	// ----------------------------------------------------------------
	{
		ID:          "TEST-IMG",
		Name:        "test_image.jpg (初期テスト 263手 正面)",
		ImagePath:   "web/test_image.jpg",
		GTPath:      "test_ground_truth.json",
		Corners:     [4]Point{{448, 545}, {1588, 595}, {1552, 1955}, {112, 1695}},
		DstW:        680,
		DstH:        730,
		Margin:      40.0,
		PitchStart:  34.0,
		PitchEnd:    40.0,
		TopOffX:     3.0,
		BotOffX:     -5.0,
		TopOffY:     18.0,
		BotOffY:     7.0,
		TopScaleX:   0.96,
		BotScaleX:   0.96,
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
	fmt.Printf("%-12s  %-42s  %8s  %8s  %6s\n", "ID", "テストケース", "正解率", "基準", "判定")
	fmt.Println(strings.Repeat("─", 80))
	for _, r := range results {
		judge := "PASS"
		if !r.Passed {
			judge = "FAIL ⚠"
		}
		fmt.Printf("%-12s  %-42s  %5.2f%%   >=%4.1f%%  %s\n",
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
