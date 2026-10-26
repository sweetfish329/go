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
	colCardBg   = color.RGBA{R: 244, G: 246, B: 245, A: 255} // #f4f6f5 (Card Surface)
	colBoardBg  = color.RGBA{R: 235, G: 218, B: 192, A: 255} // #ebdac0 (Milk Tea Beige)
	colDark     = color.RGBA{R: 37, G: 53, B: 48, A: 255}    // #253530 (Dark Sage / Charcoal)
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

func fillRect(img *image.RGBA, x, y, w, h int, col color.Color) {
	rect := image.Rect(x, y, x+w, y+h)
	draw.Draw(img, rect, &image.Uniform{col}, image.Point{}, draw.Src)
}

func strokeRect(img *image.RGBA, x, y, w, h int, thickness int, col color.Color) {
	// Top
	fillRect(img, x, y, w, thickness, col)
	// Bottom
	fillRect(img, x, y+h-thickness, w, thickness, col)
	// Left
	fillRect(img, x, y, thickness, h, col)
	// Right
	fillRect(img, x+w-thickness, y, thickness, h, col)
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

func drawDisk(img *image.RGBA, cx, cy int, radius float64, col color.RGBA) {
	minX := int(math.Floor(float64(cx) - radius - 1.0))
	maxX := int(math.Ceil(float64(cx) + radius + 1.0))
	minY := int(math.Floor(float64(cy) - radius - 1.0))
	maxY := int(math.Ceil(float64(cy) + radius + 1.0))

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx := float64(x - cx)
			dy := float64(y - cy)
			d := math.Hypot(dx, dy)
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

func drawStone(img *image.RGBA, cx, cy int, radius float64, isBlack bool) {
	if isBlack {
		drawDisk(img, cx, cy, radius, colDark)
	} else {
		// White stone: solid white disk with dark sage outline
		drawDisk(img, cx, cy, radius, colDark)
		drawDisk(img, cx, cy, radius-1.2, colWhite)
	}
}

func renderBoardContent(img *image.RGBA, boardX, boardY, boardSize int, board *libsgf.Board, size int) {
	// 1. Fill board wood background
	fillRect(img, boardX, boardY, boardSize, boardSize, colBoardBg)

	// 2. Grid lines
	margin := int(math.Round(float64(boardSize) * 0.048))
	playableSize := boardSize - margin*2
	step := float64(playableSize) / float64(size-1)

	// Vertical lines
	for i := 0; i < size; i++ {
		x := boardX + margin + int(math.Round(float64(i)*step))
		y1 := boardY + margin
		y2 := boardY + boardSize - margin
		drawLine(img, x, y1, x, y2, 1, colDark)
	}

	// Horizontal lines
	for i := 0; i < size; i++ {
		y := boardY + margin + int(math.Round(float64(i)*step))
		x1 := boardX + margin
		x2 := boardX + boardSize - margin
		drawLine(img, x1, y, x2, y, 1, colDark)
	}

	// 3. Star points (Hoshi)
	starPoints := getStarPoints(size)
	starRadius := math.Max(2.5, step*0.11)
	for _, pt := range starPoints {
		cx := boardX + margin + int(math.Round(float64(pt.x)*step))
		cy := boardY + margin + int(math.Round(float64(pt.y)*step))
		drawDisk(img, cx, cy, starRadius, colDark)
	}

	// 4. Stones
	stoneRadius := step * 0.47
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			stoneVal := board.State[x][y]
			if stoneVal == libsgf.EMPTY {
				continue
			}
			cx := boardX + margin + int(math.Round(float64(x)*step))
			cy := boardY + margin + int(math.Round(float64(y)*step))
			drawStone(img, cx, cy, stoneRadius, stoneVal == libsgf.BLACK)
		}
	}
}

// GenerateBoardImage creates a 1200x630 OGP image of the final board state in modern Washi Clay style.
func GenerateBoardImage(board *libsgf.Board, root *libsgf.Node, size int) image.Image {
	width, height := 1200, 630
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 1. Overall Washi Sage Gray background
	fillRect(img, 0, 0, width, height, colCanvasBg)

	// 2. Aesthetic concentric circles
	drawCircleRing(img, 100, 315, 100, 1.5, colRing)
	drawCircleRing(img, 100, 315, 180, 1.5, colRing)
	drawCircleRing(img, 100, 315, 260, 1.5, colRing)

	drawCircleRing(img, 1100, 315, 80, 1.5, colRing)
	drawCircleRing(img, 1100, 315, 160, 1.5, colRing)
	drawCircleRing(img, 1100, 315, 220, 1.5, colRing)

	// 3. Central solid shadow card
	cardSize := 590
	cardX := (width - cardSize) / 2
	cardY := (height - cardSize) / 2

	// Solid offset shadow (+10px, +10px)
	fillRect(img, cardX+10, cardY+10, cardSize, cardSize, colDark)

	// Solid white/cream surface card
	fillRect(img, cardX, cardY, cardSize, cardSize, colCardBg)
	strokeRect(img, cardX, cardY, cardSize, cardSize, 3, colDark)

	// 4. Board inside the card (6px margin)
	boardMargin := 6
	innerSize := cardSize - boardMargin*2
	renderBoardContent(img, cardX+boardMargin, cardY+boardMargin, innerSize, board, size)

	return img
}

// GenerateSquareThumbnail creates a 600x600 square thumbnail of the board state in modern Washi Clay style,
// perfectly tailored for 1:1 aspect-ratio card tiles in the kifu list.
func GenerateSquareThumbnail(board *libsgf.Board, root *libsgf.Node, size int) image.Image {
	imgSize := 600
	img := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))

	// 1. Overall Washi Sage Gray background
	fillRect(img, 0, 0, imgSize, imgSize, colCanvasBg)

	// 2. Solid shadow card in center
	cardSize := 550
	cardX := (imgSize - cardSize) / 2 // 25
	cardY := (imgSize - cardSize) / 2 // 25
	shadowOffset := 8

	// Solid offset shadow
	fillRect(img, cardX+shadowOffset, cardY+shadowOffset, cardSize, cardSize, colDark)

	// Solid white/cream surface card
	fillRect(img, cardX, cardY, cardSize, cardSize, colCardBg)
	strokeRect(img, cardX, cardY, cardSize, cardSize, 3, colDark)

	// 3. Board inside card (6px margin)
	boardMargin := 6
	innerSize := cardSize - boardMargin*2
	renderBoardContent(img, cardX+boardMargin, cardY+boardMargin, innerSize, board, size)

	return img
}
