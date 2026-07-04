package sgf

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

type starPoint struct {
	x, y int
}

func getStarPoints(size int) []starPoint {
	if size == 19 {
		idxs := []int{3, 9, 15}
		pts := []starPoint{}
		for _, x := range idxs {
			for _, y := range idxs {
				pts = append(pts, starPoint{x, y})
			}
		}
		return pts
	} else if size == 13 {
		return []starPoint{
			{3, 3}, {3, 9}, {9, 3}, {9, 9}, {6, 6},
		}
	} else if size == 9 {
		return []starPoint{
			{2, 2}, {2, 6}, {6, 2}, {6, 6}, {4, 4},
		}
	}
	return nil
}

func setPixel(img *image.RGBA, x, y int, col color.Color) {
	if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
		img.Set(x, y, col)
	}
}

func drawLine(img *image.RGBA, x1, y1, x2, y2 int, thickness int, col color.Color) {
	if x1 == x2 {
		// Vertical line
		startX := x1 - thickness/2
		for x := startX; x < startX+thickness; x++ {
			for y := y1; y <= y2; y++ {
				setPixel(img, x, y, col)
			}
		}
	} else if y1 == y2 {
		// Horizontal line
		startY := y1 - thickness/2
		for y := startY; y < startY+thickness; y++ {
			for x := x1; x <= x2; x++ {
				setPixel(img, x, y, col)
			}
		}
	}
}

func drawCircle(img *image.RGBA, cx, cy, r int, col color.Color) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r {
				setPixel(img, x, y, col)
			}
		}
	}
}

func drawStone(img *image.RGBA, cx, cy, r int, isBlack bool) {
	if isBlack {
		// Black stone: radial gradient towards dark grey/black
		for y := cy - r; y <= cy+r; y++ {
			for x := cx - r; x <= cx+r; x++ {
				distSq := (x-cx)*(x-cx) + (y-cy)*(y-cy)
				if distSq <= r*r {
					// Highlight point at top-left
					hx, hy := cx-r/3, cy-r/3
					distToHighlight := math.Sqrt(float64((x-hx)*(x-hx) + (y-hy)*(y-hy)))
					ratio := distToHighlight / (float64(r) * 1.5)
					if ratio > 1.0 {
						ratio = 1.0
					}
					cVal := uint8(55 - ratio*45)
					setPixel(img, x, y, color.RGBA{cVal, cVal, cVal, 255})
				}
			}
		}
	} else {
		// White stone: gradient white to light grey with a faint shadow border
		for y := cy - r; y <= cy+r; y++ {
			for x := cx - r; x <= cx+r; x++ {
				distSq := (x-cx)*(x-cx) + (y-cy)*(y-cy)
				if distSq <= r*r {
					hx, hy := cx-r/3, cy-r/3
					distToHighlight := math.Sqrt(float64((x-hx)*(x-hx) + (y-hy)*(y-hy)))
					ratio := distToHighlight / (float64(r) * 1.5)
					if ratio > 1.0 {
						ratio = 1.0
					}
					cVal := uint8(255 - ratio*40)
					setPixel(img, x, y, color.RGBA{cVal, cVal, cVal, 255})
				} else if distSq <= (r+1)*(r+1) {
					// Outer soft shadow border
					setPixel(img, x, y, color.RGBA{140, 140, 140, 255})
				}
			}
		}
	}
}

// GenerateBoardImage creates a 1200x630 OGP image of the final board state
func GenerateBoardImage(grid [][]int, size int) image.Image {
	width, height := 1200, 630
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 1. Draw overall dark background
	bgCol := color.RGBA{27, 21, 18, 255} // Rich dark brown
	draw.Draw(img, img.Bounds(), &image.Uniform{bgCol}, image.Point{}, draw.Src)

	// 2. Draw go board container
	// Center the board horizontally. Size of board will be 580x580.
	boardSize := 580
	boardX := (width - boardSize) / 2
	boardY := (height - boardSize) / 2
	boardRect := image.Rect(boardX, boardY, boardX+boardSize, boardY+boardSize)
	boardCol := color.RGBA{243, 213, 159, 255} // Light wood color
	draw.Draw(img, boardRect, &image.Uniform{boardCol}, image.Point{}, draw.Src)

	// Draw dark border around the board
	borderColor := color.RGBA{40, 30, 20, 255}
	drawLine(img, boardX, boardY, boardX+boardSize, boardY, 3, borderColor)                     // Top
	drawLine(img, boardX, boardY+boardSize, boardX+boardSize, boardY+boardSize, 3, borderColor) // Bottom
	drawLine(img, boardX, boardY, boardX, boardY+boardSize, 3, borderColor)                     // Left
	drawLine(img, boardX+boardSize, boardY, boardX+boardSize, boardY+boardSize, 3, borderColor) // Right

	// 3. Draw grid lines
	margin := 25
	playableSize := boardSize - margin*2
	step := float64(playableSize) / float64(size-1)

	lineColor := color.RGBA{50, 40, 30, 255}

	// Vertical lines
	for i := 0; i < size; i++ {
		x := boardX + margin + int(math.Round(float64(i)*step))
		y1 := boardY + margin
		y2 := boardY + boardSize - margin
		drawLine(img, x, y1, x, y2, 1, lineColor)
	}

	// Horizontal lines
	for i := 0; i < size; i++ {
		y := boardY + margin + int(math.Round(float64(i)*step))
		x1 := boardX + margin
		x2 := boardX + boardSize - margin
		drawLine(img, x1, y, x2, y, 1, lineColor)
	}

	// 4. Draw star points (Hoshi)
	starPoints := getStarPoints(size)
	for _, pt := range starPoints {
		cx := boardX + margin + int(math.Round(float64(pt.x)*step))
		cy := boardY + margin + int(math.Round(float64(pt.y)*step))
		drawCircle(img, cx, cy, 4, lineColor)
	}

	// 5. Draw stones
	// Radius should scale based on step size
	stoneRadius := int(math.Round(step * 0.48))
	if stoneRadius < 4 {
		stoneRadius = 4
	}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			stoneVal := grid[y][x]
			if stoneVal == 0 {
				continue
			}
			cx := boardX + margin + int(math.Round(float64(x)*step))
			cy := boardY + margin + int(math.Round(float64(y)*step))
			drawStone(img, cx, cy, stoneRadius, stoneVal == 1)
		}
	}

	return img
}
