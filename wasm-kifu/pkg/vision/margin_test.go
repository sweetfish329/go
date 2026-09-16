package vision

import (
	"image"
	"image/color"
	"testing"
)

func TestDynamicMarginShift(t *testing.T) {
	// 500x500 の矩形木枠
	rawOuter := [4]Point{
		{X: 50, Y: 50},   // TL
		{X: 450, Y: 50},  // TR
		{X: 450, Y: 450}, // BR
		{X: 50, Y: 450},  // BL
	}

	// 各辺 20px のマージン
	margins := [4]float64{20, 20, 20, 20}

	shifted := ApplyDynamicMarginShift(rawOuter, margins)

	expectedTL := Point{X: 70, Y: 70}
	expectedTR := Point{X: 430, Y: 70}
	expectedBR := Point{X: 430, Y: 430}
	expectedBL := Point{X: 70, Y: 430}

	eps := 1e-4
	checkPoint := func(name string, actual, expected Point) {
		if (actual.X-expected.X)*(actual.X-expected.X)+(actual.Y-expected.Y)*(actual.Y-expected.Y) > eps {
			t.Errorf("%s mismatch: got (%.2f, %.2f), expected (%.2f, %.2f)",
				name, actual.X, actual.Y, expected.X, expected.Y)
		}
	}

	checkPoint("TL", shifted[0], expectedTL)
	checkPoint("TR", shifted[1], expectedTR)
	checkPoint("BR", shifted[2], expectedBR)
	checkPoint("BL", shifted[3], expectedBL)
}

func TestMeasureDynamicWoodMargins_Synthetic(t *testing.T) {
	w, h := 200, 200
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// 木目背景 (R=180, G=130, B=80)
	woodColor := color.RGBA{R: 180, G: 130, B: 80, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, woodColor)
		}
	}

	// 黒い外枠格子線 (マージン 15px 内側: x=25, x=175, y=25, y=175)
	blackColor := color.RGBA{R: 30, G: 30, B: 30, A: 255}
	for x := 25; x <= 175; x++ {
		img.Set(x, 25, blackColor)
		img.Set(x, 175, blackColor)
	}
	for y := 25; y <= 175; y++ {
		img.Set(25, y, blackColor)
		img.Set(175, y, blackColor)
	}

	rawOuter := [4]Point{
		{X: 10, Y: 10},   // TL
		{X: 190, Y: 10},  // TR
		{X: 190, Y: 190}, // BR
		{X: 10, Y: 190},  // BL
	}

	margins := MeasureDynamicWoodMargins(img, rawOuter)
	// 期待マージン: 25 - 10 = 15px
	for i, m := range margins {
		if m < 13.0 || m > 17.0 {
			t.Errorf("margin[%d] = %.1f, expected approx 15.0", i, m)
		}
	}
}
