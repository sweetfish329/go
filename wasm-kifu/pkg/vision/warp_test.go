package vision

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestWarpSize(t *testing.T) {
	// 縦長アスペクト (avgH >= avgW)
	portraitCorners := [4]Point{
		{X: 100, Y: 100},
		{X: 300, Y: 100},
		{X: 300, Y: 400},
		{X: 100, Y: 400},
	}
	dstW, dstH, margin := WarpSize(portraitCorners)
	if dstW != 680 || dstH != 732 {
		t.Errorf("expected portrait 680x732, got %dx%d", dstW, dstH)
	}
	if margin != 40.0 {
		t.Errorf("expected margin 40.0, got %.1f", margin)
	}

	// 横長アスペクト (横からの撮影: 上下辺が主に垂直 dy > dx, 左右辺が主に水平 dx > dy)
	landscapeCorners := [4]Point{
		{X: 100, Y: 100},
		{X: 100, Y: 400},
		{X: 250, Y: 400},
		{X: 250, Y: 100},
	}
	dstW, dstH, _ = WarpSize(landscapeCorners)
	if dstW != 732 || dstH != 680 {
		t.Errorf("expected landscape 732x680, got %dx%d", dstW, dstH)
	}
}

func TestWarpPerspectiveWithMargin(t *testing.T) {
	// 200x200の入力画像 (中心部に白、外側は黒)
	w, h := 200, 200
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x >= 50 && x < 150 && y >= 50 && y < 150 {
				src.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			} else {
				src.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255})
			}
		}
	}

	// srcCorners を [50,50], [150,50], [150,150], [50,150] とする
	srcCorners := [4]Point{
		{X: 50, Y: 50},
		{X: 150, Y: 50},
		{X: 150, Y: 150},
		{X: 50, Y: 150},
	}

	dstW, dstH := 100, 100
	margin := 10.0

	warped := WarpPerspectiveWithMargin(src, srcCorners, dstW, dstH, margin)
	if warped.Bounds().Dx() != dstW || warped.Bounds().Dy() != dstH {
		t.Fatalf("unexpected warped bounds: %v", warped.Bounds())
	}

	// 中心 (50, 50) は白領域 (255, 255, 255)
	centerColor := warped.RGBAAt(50, 50)
	if centerColor.R < 200 || centerColor.G < 200 || centerColor.B < 200 {
		t.Errorf("expected white at center, got %v", centerColor)
	}

	// マージンより外側の角 (0, 0) は元画像の (50, 50) より外側（黒領域）
	cornerColor := warped.RGBAAt(2, 2)
	if cornerColor.R > 50 {
		t.Errorf("expected black outside margin, got %v", cornerColor)
	}
}

func TestExtractBoardFeatures(t *testing.T) {
	dstW := 680
	dstH := 732
	margin := 40.0

	warped := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	// 全体を榧色 (木目: R=180, G=140, B=90) で塗りつぶす
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			warped.Set(x, y, color.RGBA{R: 180, G: 140, B: 90, A: 255})
		}
	}

	// 中央天元 (9, 9) 付近に黒石 (R=30, G=30, B=30) を配置
	boardW := float64(dstW) - 2*margin
	boardH := float64(dstH) - 2*margin
	tengenX := int(margin + 9*(boardW/18.0) + 0.5)
	tengenY := int(margin + 9*(boardH/18.0) + 0.5)

	// 半径25pxで天元交点付近を確実にカバー
	for dy := -25; dy <= 25; dy++ {
		for dx := -25; dx <= 25; dx++ {
			if dx*dx+dy*dy <= 25*25 {
				warped.Set(tengenX+dx, tengenY+dy, color.RGBA{R: 30, G: 30, B: 30, A: 255})
			}
		}
	}

	features := ExtractBoardFeatures(warped, dstW, dstH)
	if features == nil {
		t.Fatalf("expected non-nil BoardFeaturesData")
	}

	// 天元 (idx = 9*19 + 9 = 180)
	tengenFeat := features.Features[9*BoardSize+9]
	if tengenFeat.L > 60.0 {
		t.Errorf("expected dark luminance at tengen, got %.1f", tengenFeat.L)
	}

	// 角 (0, 0) は木目色のはず (RMB > 50)
	cornerFeat := features.Features[0]
	if math.Abs(cornerFeat.R-180) > 10 || math.Abs(cornerFeat.B-90) > 10 {
		t.Errorf("expected wood color at corner (0,0), got R=%.1f, B=%.1f", cornerFeat.R, cornerFeat.B)
	}
	if cornerFeat.RMB < 60.0 {
		t.Errorf("expected high woodness (RMB) at corner, got %.1f", cornerFeat.RMB)
	}
}
