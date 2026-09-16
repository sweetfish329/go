// Package main は碁盤画像認識における「四隅自動検出 (Corner Auto-Detection)」の
// 定量評価テストスイートを実行する。正解四隅座標 (GT Corners) との頂点誤差および IoU を自動検証する。
package main

import (
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sweetfish329/go/wasm-kifu/pkg/vision"
)

type CornerTestCase struct {
	ID        string
	Name      string
	ImagePath string
	GTCorners [4]vision.Point // 正解四隅 [TL, TR, BR, BL] (正立画像座標)
	MinIoU    float64         // 許容最小 IoU (%)
	MaxError  float64         // 許容最大頂点平均誤差 (px)
}

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

var cornerTestCases = []CornerTestCase{
	{
		ID:        "CORNER-3-1",
		Name:      "test3-1.jpg (王座戦予選 204手 正面俯角)",
		ImagePath: "test3-1.jpg",
		GTCorners: [4]vision.Point{
			{X: 465, Y: 555},  // TL
			{X: 1310, Y: 650}, // TR
			{X: 1235, Y: 1715},// BR
			{X: 100, Y: 1440}, // BL
		},
		MinIoU:   85.0,
		MaxError: 95.0,
	},
	{
		ID:        "CORNER-2-1",
		Name:      "test2-1.jpg (竜星戦 282手 正面)",
		ImagePath: "test2-1.jpg",
		GTCorners: [4]vision.Point{
			{X: 456, Y: 696},  // TL
			{X: 1582, Y: 696}, // TR
			{X: 1949, Y: 1848},// BR
			{X: 101, Y: 1860}, // BL
		},
		MinIoU:   84.0,
		MaxError: 95.0,
	},
	{
		ID:        "CORNER-IMG",
		Name:      "test_image.jpg (初期テスト 263手 正面)",
		ImagePath: "web/test_image.jpg",
		GTCorners: [4]vision.Point{
			{X: 448, Y: 545},  // TL
			{X: 1588, Y: 595}, // TR
			{X: 1552, Y: 1955},// BR
			{X: 112, Y: 1695}, // BL
		},
		MinIoU:   85.0,
		MaxError: 95.0,
	},
}

func main() {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("  Go Kifu Vision ─ 碁盤四隅自動認識 (Corner Auto-Detection) 定量評価テスト")
	fmt.Printf("  実行日時: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 80))

	allPassed := true

	for i, tc := range cornerTestCases {
		fmt.Printf("\n[%d/%d] ▶ %s\n", i+1, len(cornerTestCases), tc.Name)

		filePath := findFile(tc.ImagePath)
		img, err := vision.LoadImageUpright(filePath)
		orient := vision.GetExifOrientation(filePath)
		bRaw := img.Bounds()
		fmt.Printf("  Image bounds: %dx%d, EXIF orientation: %d\n", bRaw.Dx(), bRaw.Dy(), orient)

		start := time.Now()
		res, err := vision.DetectBoardCorners(img)
		duration := time.Since(start)

		starRefiner := vision.NewStarHomographyRefiner()
		stars := starRefiner.DetectStarPoints(img, res.Corners)
		fmt.Printf("  [Stars detected: %d]\n", len(stars))
		for _, s := range stars {
			fmt.Printf("    Star (%d,%d) at (%.1f, %.1f) conf=%.2f\n", s.GridX, s.GridY, s.ImgX, s.ImgY, s.Confidence)
		}

		if err != nil {
			fmt.Printf("  ❌ 四隅自動検出失敗: %v\n", err)
			allPassed = false
			continue
		}

		b := img.Bounds()
		iou := vision.CalculateQuadIoU(res.Corners, tc.GTCorners, b.Dx(), b.Dy())
		mae := vision.CalculateCornerError(res.Corners, tc.GTCorners)

		passed := iou >= tc.MinIoU && mae <= tc.MaxError
		mark := "✅ PASS"
		if !passed {
			mark = "⚠️ 要チューニング"
			allPassed = false
		}

		fmt.Printf("  %s | IoU: %5.2f%% (基準 >=%4.1f%%) | 頂点誤差(MAE): %5.1fpx (基準 <=%4.1fpx) | %v\n",
			mark, iou, tc.MinIoU, mae, tc.MaxError, duration.Round(time.Millisecond))

		cornerNames := []string{"左上(TL)", "右上(TR)", "右下(BR)", "左下(BL)"}
		for c := 0; c < 4; c++ {
			pred := res.Corners[c]
			raw := res.RawOuter[c]
			gt := tc.GTCorners[c]
			dist := math.Hypot(pred.X-gt.X, pred.Y-gt.Y)
			distRaw := math.Hypot(raw.X-gt.X, raw.Y-gt.Y)
			fmt.Printf("     [%s] 検出=(%4.0f, %4.0f)  RawOuter=(%4.0f, %4.0f) [誤差Raw=%4.1f]  正解=(%4.0f, %4.0f)  誤差=%4.1fpx\n",
				cornerNames[c], pred.X, pred.Y, raw.X, raw.Y, distRaw, gt.X, gt.Y, dist)
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("─", 80))
	fmt.Println("  四隅自動検出 総合サマリー")
	fmt.Println(strings.Repeat("─", 80))

	if allPassed {
		fmt.Println("🎉 全テストケースで四隅自動検出が基準をクリアしました！")
	} else {
		fmt.Println("ℹ️ 四隅自動検出テスト完了（一部ケースで精度向上の余地あり）")
	}
}
