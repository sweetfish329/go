package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"

	"github.com/sweetfish329/go/wasm-kifu/pkg/vision"
)

// Bresenham's line algorithm with thickness
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, col color.RGBA, thickness int) {
	dx := math.Abs(float64(x1 - x0))
	dy := math.Abs(float64(y1 - y0))
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy

	drawThickPixel := func(cx, cy int) {
		r := thickness / 2
		for ty := -r; ty <= r; ty++ {
			for tx := -r; tx <= r; tx++ {
				px := cx + tx
				py := cy + ty
				if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
					img.SetRGBA(px, py, col)
				}
			}
		}
	}

	for {
		drawThickPixel(x0, y0)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// 破線描画
func drawDashedLine(img *image.RGBA, x0, y0, x1, y1 int, col color.RGBA, thickness, dashLen, gapLen int) {
	totalDist := math.Hypot(float64(x1-x0), float64(y1-y0))
	if totalDist < 1e-3 {
		return
	}
	vx := float64(x1-x0) / totalDist
	vy := float64(y1-y0) / totalDist

	curDist := 0.0
	drawing := true
	for curDist < totalDist {
		segLen := float64(dashLen)
		if !drawing {
			segLen = float64(gapLen)
		}
		nextDist := math.Min(totalDist, curDist+segLen)
		if drawing {
			sx := int(math.Round(float64(x0) + vx*curDist))
			sy := int(math.Round(float64(y0) + vy*curDist))
			ex := int(math.Round(float64(x0) + vx*nextDist))
			ey := int(math.Round(float64(y0) + vy*nextDist))
			drawLine(img, sx, sy, ex, ey, col, thickness)
		}
		curDist = nextDist
		drawing = !drawing
	}
}

// 塗りつぶし円描画
func drawCircle(img *image.RGBA, cx, cy, radius int, fillCol, strokeCol color.RGBA, strokeWidth int) {
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			dist := math.Hypot(float64(dx), float64(dy))
			px := cx + dx
			py := cy + dy
			if px < 0 || px >= img.Bounds().Dx() || py < 0 || py >= img.Bounds().Dy() {
				continue
			}
			if dist <= float64(radius) {
				if dist >= float64(radius-strokeWidth) && strokeWidth > 0 {
					img.SetRGBA(px, py, strokeCol)
				} else {
					img.SetRGBA(px, py, fillCol)
				}
			}
		}
	}
}

func RenderStage2DebugImage(srcImg image.Image, res *vision.CornerDetectionResult, outPath string) error {
	b := srcImg.Bounds()
	w, h := b.Dx(), b.Dy()

	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), srcImg, b.Min, draw.Src)

	// 1. 木枠外縁 (青色破線)
	blueDash := color.RGBA{R: 52, G: 152, B: 219, A: 255}
	raw := res.RawOuter
	for i := 0; i < 4; i++ {
		p1 := raw[i]
		p2 := raw[(i+1)%4]
		drawDashedLine(out, int(p1.X), int(p1.Y), int(p2.X), int(p2.Y), blueDash, 6, 24, 24)
	}

	// 2. 星 (金色丸)
	gold := color.RGBA{R: 241, G: 196, B: 15, A: 255}
	dark := color.RGBA{R: 44, G: 62, B: 80, A: 255}
	for _, s := range res.Stars {
		drawCircle(out, int(s.ImgX), int(s.ImgY), 16, gold, dark, 3)
	}

	// 3. 1線四隅 (赤色実線)
	red := color.RGBA{R: 231, G: 76, B: 60, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	corners := res.Corners
	for i := 0; i < 4; i++ {
		p1 := corners[i]
		p2 := corners[(i+1)%4]
		drawLine(out, int(p1.X), int(p1.Y), int(p2.X), int(p2.Y), red, 8)
	}

	// 4. コーナー番号バッジ
	for i := 0; i < 4; i++ {
		p := corners[i]
		drawCircle(out, int(p.X), int(p.Y), 32, red, white, 4)
		// 簡単な十字・番号マーク
		drawLine(out, int(p.X)-12, int(p.Y), int(p.X)+12, int(p.Y), white, 4)
		drawLine(out, int(p.X), int(p.Y)-12, int(p.X), int(p.Y)+12, white, 4)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, out)
}

func main() {
	imgPath := "../test_image.jpg"
	img, err := vision.LoadImageUpright(imgPath)
	if err != nil {
		fmt.Printf("Load err: %v\n", err)
		return
	}

	res, err := vision.DetectBoardCorners(img)
	if err != nil {
		fmt.Printf("Detect err: %v\n", err)
		return
	}

	outPath := "../test_debug_stage2.png"
	err = RenderStage2DebugImage(img, res, outPath)
	if err != nil {
		fmt.Printf("Render err: %v\n", err)
		return
	}
	fmt.Printf("Successfully rendered debug image to %s\n", outPath)
	fmt.Printf("Corners:\n")
	for i, c := range res.Corners {
		fmt.Printf("  [%d] (%.1f, %.1f)\n", i, c.X, c.Y)
	}
	fmt.Printf("RawOuter:\n")
	for i, c := range res.RawOuter {
		fmt.Printf("  [%d] (%.1f, %.1f)\n", i, c.X, c.Y)
	}
	fmt.Printf("Stars: %d detected\n", len(res.Stars))
}
