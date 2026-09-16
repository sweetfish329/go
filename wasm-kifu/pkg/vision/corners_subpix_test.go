package vision

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestRefineCornerSubPixFörstner(t *testing.T) {
	w, h := 100, 100
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// 背景: 白 (240, 240, 240)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}

	// 真の交点 (50.0, 50.0) を中心とする太さ1pxの直交黒線
	// X方向ライン (y=50)
	for x := 20; x <= 80; x++ {
		img.Set(x, 50, color.RGBA{R: 20, G: 20, B: 20, A: 255})
	}
	// Y方向ライン (x=50)
	for y := 20; y <= 80; y++ {
		img.Set(50, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
	}

	// 意図的に1.5ピクセルずらした初期点 (51.5, 48.5)
	initPt := Point{X: 51.5, Y: 48.5}
	params := DefaultFörstnerParams()
	refined := RefineCornerSubPixFörstner(img, initPt, params)

	errDist := math.Hypot(refined.X-50.0, refined.Y-50.0)
	if errDist > 0.25 {
		t.Errorf("Sub-pixel refinement error too large: got (%.3f, %.3f), want (50, 50), error=%.3f px",
			refined.X, refined.Y, errDist)
	}
}

func TestRefineCornersSubPix_SyntheticBoard(t *testing.T) {
	w, h := 200, 200
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 160, B: 100, A: 255})
		}
	}

	// 四角形グリッド (30, 30) から (170, 170)
	for x := 30; x <= 170; x++ {
		img.Set(x, 30, color.RGBA{R: 10, G: 10, B: 10, A: 255})
		img.Set(x, 170, color.RGBA{R: 10, G: 10, B: 10, A: 255})
	}
	for y := 30; y <= 170; y++ {
		img.Set(30, y, color.RGBA{R: 10, G: 10, B: 10, A: 255})
		img.Set(170, y, color.RGBA{R: 10, G: 10, B: 10, A: 255})
	}

	// 意図的にずらした粗い4隅
	coarseCorners := [4]Point{
		{X: 31.8, Y: 28.5},
		{X: 168.2, Y: 31.2},
		{X: 171.5, Y: 168.8},
		{X: 28.6, Y: 171.3},
	}

	refined := RefineCornersSubPix(img, coarseCorners)

	trueCorners := [4]Point{
		{X: 30, Y: 30},
		{X: 170, Y: 30},
		{X: 170, Y: 170},
		{X: 30, Y: 170},
	}

	for i := 0; i < 4; i++ {
		dist := math.Hypot(refined[i].X-trueCorners[i].X, refined[i].Y-trueCorners[i].Y)
		if dist > 0.4 {
			t.Errorf("Corner %d refinement error: got (%.2f, %.2f), want (%.2f, %.2f), dist=%.2f px",
				i, refined[i].X, refined[i].Y, trueCorners[i].X, trueCorners[i].Y, dist)
		}
	}
}
