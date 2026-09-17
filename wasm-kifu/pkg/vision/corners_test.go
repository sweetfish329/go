package vision

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestDetectBoardCorners_SmallImageError(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	_, err := DetectBoardCorners(img)
	if err == nil {
		t.Fatalf("expected error for small image, got nil")
	}
}

func TestOrderCorners(t *testing.T) {
	// シャッフルされた4点
	pts := [4]Point{
		{X: 100, Y: 100}, // BR
		{X: 10, Y: 100},  // BL
		{X: 10, Y: 10},   // TL
		{X: 100, Y: 10},  // TR
	}

	ordered := OrderCorners(pts)

	// [TL, TR, BR, BL] の順に整列されているか
	if ordered[0].X != 10 || ordered[0].Y != 10 {
		t.Errorf("expected TL=(10, 10), got (%.0f, %.0f)", ordered[0].X, ordered[0].Y)
	}
	if ordered[1].X != 100 || ordered[1].Y != 10 {
		t.Errorf("expected TR=(100, 10), got (%.0f, %.0f)", ordered[1].X, ordered[1].Y)
	}
	if ordered[2].X != 100 || ordered[2].Y != 100 {
		t.Errorf("expected BR=(100, 100), got (%.0f, %.0f)", ordered[2].X, ordered[2].Y)
	}
	if ordered[3].X != 10 || ordered[3].Y != 100 {
		t.Errorf("expected BL=(10, 100), got (%.0f, %.0f)", ordered[3].X, ordered[3].Y)
	}
}

func TestApplyMarginInset(t *testing.T) {
	outer := [4]Point{
		{X: 0, Y: 0},
		{X: 1000, Y: 0},
		{X: 1000, Y: 1000},
		{X: 0, Y: 1000},
	}

	inset := ApplyMarginInset(outer, 0.05, 0.05) // 5% インセット

	if math.Abs(inset[0].X-50) > 1e-3 || math.Abs(inset[0].Y-50) > 1e-3 {
		t.Errorf("TL mismatch: got (%.1f, %.1f), want (50, 50)", inset[0].X, inset[0].Y)
	}
	if math.Abs(inset[1].X-950) > 1e-3 || math.Abs(inset[1].Y-50) > 1e-3 {
		t.Errorf("TR mismatch: got (%.1f, %.1f), want (950, 50)", inset[1].X, inset[1].Y)
	}
	if math.Abs(inset[2].X-950) > 1e-3 || math.Abs(inset[2].Y-950) > 1e-3 {
		t.Errorf("BR mismatch: got (%.1f, %.1f), want (950, 950)", inset[2].X, inset[2].Y)
	}
	if math.Abs(inset[3].X-50) > 1e-3 || math.Abs(inset[3].Y-950) > 1e-3 {
		t.Errorf("BL mismatch: got (%.1f, %.1f), want (50, 950)", inset[3].X, inset[3].Y)
	}
}

func TestDetectBoardCorners_SyntheticBoard(t *testing.T) {
	w, h := 600, 600
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// 背景: 暗いグレー
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 50, G: 50, B: 50, A: 255})
		}
	}

	// 中央に木目の碁盤 (榧色: R=180, G=140, B=90)
	boardRect := image.Rect(100, 100, 500, 500)
	for y := boardRect.Min.Y; y < boardRect.Max.Y; y++ {
		for x := boardRect.Min.X; x < boardRect.Max.X; x++ {
			img.Set(x, y, color.RGBA{R: 180, G: 140, B: 90, A: 255})
		}
	}

	res, err := DetectBoardCorners(img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gtCorners := [4]Point{
		{X: 100, Y: 100},
		{X: 500, Y: 100},
		{X: 500, Y: 500},
		{X: 100, Y: 500},
	}

	iou := CalculateQuadIoU(res.RawOuter, gtCorners, w, h)
	mae := CalculateCornerError(res.RawOuter, gtCorners)

	if iou < 85.0 {
		t.Errorf("expected IoU >= 85.0%%, got %.2f%%", iou)
	}
	if mae > 50.0 {
		t.Errorf("expected MAE <= 50.0px, got %.1fpx", mae)
	}
}
