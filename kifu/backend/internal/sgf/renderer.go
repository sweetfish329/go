package sgf

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	libsgf "github.com/rooklift/sgf"
)

var (
	colCanvasBg = color.RGBA{R: 220, G: 223, B: 220, A: 255} // #dcdfdc (Washi Sage Gray)
	colBoardBg  = color.RGBA{R: 235, G: 218, B: 192, A: 255} // #ebdac0 (Ash Wood / Milk Tea Beige)
	colDark     = color.RGBA{R: 37, G: 53, B: 48, A: 255}    // #253530 (Dark Sage / Charcoal)
	colLine     = color.RGBA{R: 37, G: 53, B: 48, A: 215}    // #253530 (Grid lines with 0.85 opacity)
	colWhite    = color.RGBA{R: 255, G: 255, B: 255, A: 255} // #ffffff (Pure White)
	colRing     = color.RGBA{R: 37, G: 53, B: 48, A: 20}     // rgba(37, 53, 48, 0.08) for aesthetic concentric circles
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

func blendPixel(img *image.RGBA, x, y int, src color.RGBA) {
	if x < 0 || x >= img.Bounds().Dx() || y < 0 || y >= img.Bounds().Dy() {
		return
	}
	if src.A == 255 {
		img.SetRGBA(x, y, src)
		return
	}
	if src.A == 0 {
		return
	}
	dst := img.RGBAAt(x, y)
	sa := uint32(src.A)
	da := uint32(255 - src.A)

	r := uint8((uint32(src.R)*sa + uint32(dst.R)*da) / 255)
	g := uint8((uint32(src.G)*sa + uint32(dst.G)*da) / 255)
	b := uint8((uint32(src.B)*sa + uint32(dst.B)*da) / 255)
	img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
}

func drawDisk(img *image.RGBA, cx, cy int, radius float64, col color.RGBA) {
	minX := int(math.Floor(float64(cx) - radius - 1.0))
	maxX := int(math.Ceil(float64(cx) + radius + 1.0))
	minY := int(math.Floor(float64(cy) - radius - 1.0))
	maxY := int(math.Ceil(float64(cy) + radius + 1.0))

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			d := math.Hypot(float64(x-cx), float64(y-cy))
			if d <= radius+0.5 {
				var alphaFactor float64
				if d <= radius-0.5 {
					alphaFactor = 1.0
				} else {
					alphaFactor = (radius + 0.5) - d
				}
				if alphaFactor > 0 {
					a := uint8(float64(col.A) * alphaFactor)
					blendPixel(img, x, y, color.RGBA{R: col.R, G: col.G, B: col.B, A: a})
				}
			}
		}
	}
}

func drawCircleRing(img *image.RGBA, cx, cy int, radius float64, thickness float64, col color.RGBA) {
	maxR := radius + thickness/2.0 + 1.0
	minX := int(math.Floor(float64(cx) - maxR))
	maxX := int(math.Ceil(float64(cx) + maxR))
	minY := int(math.Floor(float64(cy) - maxR))
	maxY := int(math.Ceil(float64(cy) + maxR))

	halfThick := thickness / 2.0

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx := float64(x - cx)
			dy := float64(y - cy)
			d := math.Hypot(dx, dy)
			distToRing := math.Abs(d - radius)
			if distToRing <= halfThick+0.5 {
				var alphaFactor float64
				if distToRing <= halfThick-0.5 {
					alphaFactor = 1.0
				} else {
					alphaFactor = (halfThick + 0.5) - distToRing
				}
				if alphaFactor > 0 {
					a := uint8(float64(col.A) * alphaFactor)
					blendPixel(img, x, y, color.RGBA{R: col.R, G: col.G, B: col.B, A: a})
				}
			}
		}
	}
}

func drawLine(img *image.RGBA, x1, y1, x2, y2 int, thickness int, col color.Color) {
	if x1 == x2 {
		startX := x1 - thickness/2
		for x := startX; x < startX+thickness; x++ {
			for y := y1; y <= y2; y++ {
				if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
					img.Set(x, y, col)
				}
			}
		}
	} else if y1 == y2 {
		startY := y1 - thickness/2
		for y := startY; y < startY+thickness; y++ {
			for x := x1; x <= x2; x++ {
				if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
					img.Set(x, y, col)
				}
			}
		}
	}
}

// renderBoardGridAndStones draws a pure flat go board with balanced padding, delicate grid lines, and crisp stones.
func renderBoardGridAndStones(img *image.RGBA, originX, originY, boardSize int, board *libsgf.Board, size int, padRatio float64) {
	// 1. Fill board surface
	rect := image.Rect(originX, originY, originX+boardSize, originY+boardSize)
	draw.Draw(img, rect, &image.Uniform{colBoardBg}, image.Point{}, draw.Src)

	// 2. Clean flat border around the board
	borderWidth := 2
	drawLine(img, originX, originY, originX+boardSize, originY, borderWidth*2, colDark)
	drawLine(img, originX, originY+boardSize-borderWidth, originX+boardSize, originY+boardSize-borderWidth, borderWidth*2, colDark)
	drawLine(img, originX, originY, originX, originY+boardSize, borderWidth*2, colDark)
	drawLine(img, originX+boardSize-borderWidth, originY, originX+boardSize-borderWidth, originY+boardSize, borderWidth*2, colDark)

	// 3. Grid lines & Golden ratio padding calculation
	step := float64(boardSize) / (float64(size-1) + 2.0*padRatio)
	padding := step * padRatio

	for i := 0; i < size; i++ {
		pos := float64(originX) + padding + float64(i)*step
		pYStart := int(math.Round(float64(originY) + padding))
		pYEnd := int(math.Round(float64(originY) + padding + float64(size-1)*step))
		drawLine(img, int(math.Round(pos)), pYStart, int(math.Round(pos)), pYEnd, 1, colLine)

		pYPos := float64(originY) + padding + float64(i)*step
		pXStart := int(math.Round(float64(originX) + padding))
		pXEnd := int(math.Round(float64(originX) + padding + float64(size-1)*step))
		drawLine(img, pXStart, int(math.Round(pYPos)), pXEnd, int(math.Round(pYPos)), 1, colLine)
	}

	// 4. Star points (Hoshi)
	starRadius := math.Max(2.5, step*0.11)
	for _, pt := range getStarPoints(size) {
		cx := int(math.Round(float64(originX) + padding + float64(pt.x)*step))
		cy := int(math.Round(float64(originY) + padding + float64(pt.y)*step))
		drawDisk(img, cx, cy, starRadius, colDark)
	}

	// 5. Stones (Pure flat minimalism, no move numbers or sequence clutter)
	stoneRadius := step * 0.47
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			stoneVal := board.State[x][y]
			if stoneVal == libsgf.EMPTY {
				continue
			}
			cx := int(math.Round(float64(originX) + padding + float64(x)*step))
			cy := int(math.Round(float64(originY) + padding + float64(y)*step))
			if stoneVal == libsgf.BLACK {
				drawDisk(img, cx, cy, stoneRadius, colDark)
			} else {
				drawDisk(img, cx, cy, stoneRadius, colDark)
				drawDisk(img, cx, cy, stoneRadius-1.1, colWhite)
			}
		}
	}
}

// GenerateBoardImage creates a 1200x630 OGP image of the final board state in a clean, minimalist flat style.
func GenerateBoardImage(board *libsgf.Board, root *libsgf.Node, size int) image.Image {
	width, height := 1200, 630
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 1. Overall Washi Sage Gray background
	draw.Draw(img, img.Bounds(), &image.Uniform{colCanvasBg}, image.Point{}, draw.Src)

	// 2. Aesthetic concentric circles (subtle editorial design)
	drawCircleRing(img, 100, 315, 100, 1.5, colRing)
	drawCircleRing(img, 100, 315, 180, 1.5, colRing)
	drawCircleRing(img, 100, 315, 260, 1.5, colRing)

	drawCircleRing(img, 1100, 315, 80, 1.5, colRing)
	drawCircleRing(img, 1100, 315, 160, 1.5, colRing)
	drawCircleRing(img, 1100, 315, 220, 1.5, colRing)

	// 3. Central pure flat board (580x580)
	boardSize := 580
	boardX := (width - boardSize) / 2
	boardY := (height - boardSize) / 2
	renderBoardGridAndStones(img, boardX, boardY, boardSize, board, size, 0.8)

	return img
}

// GenerateSquareThumbnail creates a 600x600 square thumbnail of the board state in a pure flat style,
// filling the entire tile card cleanly without superfluous shadows, nested borders, or clutter.
func GenerateSquareThumbnail(board *libsgf.Board, root *libsgf.Node, size int) image.Image {
	imgSize := 600
	img := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))

	// Render flat go board filling the entire 600x600 canvas with balanced 0.8 padding
	renderBoardGridAndStones(img, 0, 0, imgSize, board, size, 0.8)

	return img
}
