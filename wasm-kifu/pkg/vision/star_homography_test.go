package vision

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestStarHomographyRefinement(t *testing.T) {
	w, h := 600, 600
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// 背景: 木目色 (R=210, G=170, B=110)
	woodColor := color.RGBA{R: 210, G: 170, B: 110, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, woodColor)
		}
	}

	// 真の格子4隅: (60, 60), (540, 60), (540, 540), (60, 540)
	// 格子間隔: 480 / 18 = 26.666...
	step := 480.0 / 18.0
	starColor := color.RGBA{R: 15, G: 15, B: 15, A: 255}

	// 9つの星を描画 (半径 2px の黒丸)
	for _, sc := range standardStarCoords {
		sx := 60.0 + float64(sc[0])*step
		sy := 60.0 + float64(sc[1])*step
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				if dx*dx+dy*dy <= 4 {
					img.Set(int(math.Round(sx))+dx, int(math.Round(sy))+dy, starColor)
				}
			}
		}
	}

	// 意図的にマージン誤差（外枠天板を検出したために約15px外側にずれている初期4隅）
	initialCorners := [4]Point{
		{X: 45, Y: 45},   // TL (真値は 60, 60)
		{X: 555, Y: 45},  // TR (真値は 540, 60)
		{X: 555, Y: 555}, // BR (真値は 540, 540)
		{X: 45, Y: 555},  // BL (真値は 60, 540)
	}

	refiner := NewStarHomographyRefiner()
	refined, count := refiner.RefineCornersWithStarPoints(img, initialCorners)

	if count != 9 {
		t.Errorf("Expected 9 stars detected, got %d", count)
	}

	trueCorners := [4]Point{
		{X: 60, Y: 60},
		{X: 540, Y: 60},
		{X: 540, Y: 540},
		{X: 60, Y: 540},
	}

	for i := 0; i < 4; i++ {
		dist := math.Hypot(refined[i].X-trueCorners[i].X, refined[i].Y-trueCorners[i].Y)
		if dist > 2.0 {
			t.Errorf("Refined corner %d error too large: got (%.2f, %.2f), want (%.2f, %.2f), dist=%.2f px",
				i, refined[i].X, refined[i].Y, trueCorners[i].X, trueCorners[i].Y, dist)
		}
	}
}

func TestStarHomography_TestImage(t *testing.T) {
	img, err := LoadImageUpright("../../web/test_image.jpg")
	if err != nil {
		t.Skip("test_image.jpg not found, skipping")
	}

	gtCorners := [4]Point{
		{X: 448, Y: 545},  // TL
		{X: 1588, Y: 595}, // TR
		{X: 1552, Y: 1955},// BR
		{X: 112, Y: 1695}, // BL
	}

	refiner := NewStarHomographyRefiner()
	starsGT := refiner.DetectStarPoints(img, gtCorners)
	t.Logf("Detected %d stars with GT corners:", len(starsGT))
	for _, s := range starsGT {
		t.Logf("  Star (%d,%d) at (%.1f, %.1f) conf=%.2f", s.GridX, s.GridY, s.ImgX, s.ImgY, s.Confidence)
	}

	hFromTrueStars, err := FindHomographyLeastSquares(starsGT)
	if err != nil {
		t.Fatalf("FindHomographyLeastSquares failed: %v", err)
	}
	cornersFromTrueStars := [4]Point{
		ProjectPoint(hFromTrueStars, 0, 0),
		ProjectPoint(hFromTrueStars, 1, 0),
		ProjectPoint(hFromTrueStars, 1, 1),
		ProjectPoint(hFromTrueStars, 0, 1),
	}
	t.Logf("Corners from True Stars:")
	for i, c := range cornersFromTrueStars {
		dist := math.Hypot(c.X-gtCorners[i].X, c.Y-gtCorners[i].Y)
		t.Logf("  Corner %d: (%.1f, %.1f) vs GT (%.1f, %.1f) -> Error = %.1f px",
			i, c.X, c.Y, gtCorners[i].X, gtCorners[i].Y, dist)
	}


	res, err := DetectBoardCorners(img)
	if err != nil {
		t.Fatalf("DetectBoardCorners failed: %v", err)
	}
	t.Logf("Initial RawOuter: %v", res.RawOuter)
	t.Logf("Initial Corners: %v", res.Corners)

	starsInit := refiner.DetectStarPoints(img, res.RawOuter)
	t.Logf("Detected %d stars with res.RawOuter:", len(starsInit))
	for _, s := range starsInit {
		t.Logf("  Star (%d,%d) at (%.1f, %.1f) conf=%.2f", s.GridX, s.GridY, s.ImgX, s.ImgY, s.Confidence)
	}
}

