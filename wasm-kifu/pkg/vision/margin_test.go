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
	w, h := 600, 600
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// 木目背景 (R=180, G=130, B=80)
	woodColor := color.RGBA{R: 180, G: 130, B: 80, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, woodColor)
		}
	}

	// 黒い外枠格子線 (幅 3px: y=49..51, y=549..551, x=49..51, x=549..551)
	blackColor := color.RGBA{R: 30, G: 30, B: 30, A: 255}
	for x := 48; x <= 552; x++ {
		for dy := -1; dy <= 1; dy++ {
			img.Set(x, 50+dy, blackColor)
			img.Set(x, 550+dy, blackColor)
		}
	}
	for y := 48; y <= 552; y++ {
		for dx := -1; dx <= 1; dx++ {
			img.Set(50+dx, y, blackColor)
			img.Set(550+dx, y, blackColor)
		}
	}

	rawOuter := [4]Point{
		{X: 30, Y: 30},   // TL
		{X: 570, Y: 30},  // TR
		{X: 570, Y: 570}, // BR
		{X: 30, Y: 570},  // BL
	}

	margins := MeasureDynamicWoodMargins(img, rawOuter)
	// 期待マージン: 50 - 30 = 20px
	for i, m := range margins {
		if m < 17.0 || m > 23.0 {
			t.Errorf("margin[%d] = %.1f, expected approx 20.0", i, m)
		}
	}
}
