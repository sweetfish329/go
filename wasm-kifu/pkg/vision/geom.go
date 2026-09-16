package vision

import (
	"math"
	"sort"
)

// Point は2次元幾何座標を表す
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// CornerDetectionResult は四隅自動認識の結果
type CornerDetectionResult struct {
	Corners    [4]Point `json:"corners"`  // [TL, TR, BR, BL] (19路格子四隅)
	RawOuter   [4]Point `json:"rawOuter"` // 天板外隅
	Area       float64  `json:"area"`
	Confidence float64  `json:"confidence"`
}

// LineHV は直線方程式を表す
// 水平系: y = c * x + d (IsHorizontal = true)
// 垂直系: x = a * y + b (IsHorizontal = false)
type LineHV struct {
	Slope        float64 // c または a
	Intercept    float64 // d または b
	IsHorizontal bool
}

// IntersectSlopeLines は水平系直線 (y = c*x + d) と垂直系直線 (x = a*y + b) の交点を計算する
func IntersectSlopeLines(horizC, horizD, vertA, vertB float64) Point {
	denom := 1.0 - vertA*horizC
	if math.Abs(denom) < 1e-6 {
		denom = 1e-6
	}
	y := (vertB*horizC + horizD) / denom
	x := vertA*y + vertB
	return Point{X: x, Y: y}
}

// IntersectTwoGeneralLines は2直線 A1*x + B1*y + C1 = 0 と A2*x + B2*y + C2 = 0 の交点を計算する
func IntersectTwoGeneralLines(a1, b1, c1, a2, b2, c2 float64, fallback Point) Point {
	det := a1*b2 - a2*b1
	if math.Abs(det) < 1e-6 {
		return fallback
	}
	x := (b1*c2 - b2*c1) / det
	y := (a2*c1 - a1*c2) / det
	return Point{X: x, Y: y}
}

// OrderCorners は4隅の座標を時計回り [左上(TL), 右上(TR), 右下(BR), 左下(BL)] に正準整列する
func OrderCorners(pts [4]Point) [4]Point {
	cx, cy := 0.0, 0.0
	for _, p := range pts {
		cx += p.X
		cy += p.Y
	}
	cx /= 4.0
	cy /= 4.0

	type ptAngle struct {
		p Point
		a float64
	}
	angles := make([]ptAngle, 4)
	for i, p := range pts {
		angles[i] = ptAngle{p: p, a: math.Atan2(p.Y-cy, p.X-cx)}
	}
	sort.Slice(angles, func(i, j int) bool {
		return angles[i].a < angles[j].a
	})

	minSumIdx := 0
	minSum := angles[0].p.X + angles[0].p.Y
	for i := 1; i < 4; i++ {
		s := angles[i].p.X + angles[i].p.Y
		if s < minSum {
			minSum = s
			minSumIdx = i
		}
	}

	return [4]Point{
		angles[minSumIdx].p,
		angles[(minSumIdx+1)%4].p,
		angles[(minSumIdx+2)%4].p,
		angles[(minSumIdx+3)%4].p,
	}
}

// PolygonArea は靴紐の公式 (Shoelace formula) で多角形の面積を計算する
func PolygonArea(pts []Point) float64 {
	n := len(pts)
	if n < 3 {
		return 0
	}
	area := 0.0
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += pts[i].X * pts[j].Y
		area -= pts[j].X * pts[i].Y
	}
	return math.Abs(area) / 2.0
}

// CalculateCornerError は予測された4隅と正解4隅の頂点間平均ユークリッド距離 (MAE, ピクセル) を計算する
func CalculateCornerError(pred, gt [4]Point) float64 {
	total := 0.0
	for i := 0; i < 4; i++ {
		total += math.Hypot(pred[i].X-gt[i].X, pred[i].Y-gt[i].Y)
	}
	return total / 4.0
}

// CalculateQuadIoU は2つの四角形（予測と正解）の交差比 (IoU, %) を計算する
func CalculateQuadIoU(pred, gt [4]Point, imgW, imgH int) float64 {
	scale := 8
	gw := (imgW + scale - 1) / scale
	gh := (imgH + scale - 1) / scale

	maskPred := rasterizeQuad(pred, gw, gh, float64(scale))
	maskGT := rasterizeQuad(gt, gw, gh, float64(scale))

	intersection := 0
	union := 0
	for y := 0; y < gh; y++ {
		for x := 0; x < gw; x++ {
			p := maskPred[y][x]
			g := maskGT[y][x]
			if p && g {
				intersection++
			}
			if p || g {
				union++
			}
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union) * 100.0
}

// pointInConvexQuad は凸四角形の内外判定を行う
func pointInConvexQuad(pt Point, quad [4]Point) bool {
	cross := func(a, b, c Point) float64 {
		return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
	}
	d1 := cross(quad[0], quad[1], pt)
	d2 := cross(quad[1], quad[2], pt)
	d3 := cross(quad[2], quad[3], pt)
	d4 := cross(quad[3], quad[0], pt)

	hasNeg := (d1 < 0) || (d2 < 0) || (d3 < 0) || (d4 < 0)
	hasPos := (d1 > 0) || (d2 > 0) || (d3 > 0) || (d4 > 0)
	return !(hasNeg && hasPos)
}

func rasterizeQuad(quad [4]Point, gw, gh int, scale float64) [][]bool {
	mask := make([][]bool, gh)
	for y := 0; y < gh; y++ {
		mask[y] = make([]bool, gw)
	}

	minX, maxX := 1e18, -1e18
	minY, maxY := 1e18, -1e18
	for _, p := range quad {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	gxMin := int(math.Max(0, math.Floor(minX/scale)))
	gxMax := int(math.Min(float64(gw-1), math.Ceil(maxX/scale)))
	gyMin := int(math.Max(0, math.Floor(minY/scale)))
	gyMax := int(math.Min(float64(gh-1), math.Ceil(maxY/scale)))

	for gy := gyMin; gy <= gyMax; gy++ {
		ptY := (float64(gy) + 0.5) * scale
		for gx := gxMin; gx <= gxMax; gx++ {
			ptX := (float64(gx) + 0.5) * scale
			if pointInConvexQuad(Point{X: ptX, Y: ptY}, quad) {
				mask[gy][gx] = true
			}
		}
	}
	return mask
}

// ApplyMarginInset は木枠外隅から内側の19路真格子隅へインセット逆算補正を行う (NotebookLM 幾何モデル)
func ApplyMarginInset(corners [4]Point, insetX, insetY float64) [4]Point {
	tl, tr, br, bl := corners[0], corners[1], corners[2], corners[3]

	vTop := Point{X: tr.X - tl.X, Y: tr.Y - tl.Y}
	vBot := Point{X: br.X - bl.X, Y: br.Y - bl.Y}
	vLeft := Point{X: bl.X - tl.X, Y: bl.Y - tl.Y}
	vRight := Point{X: br.X - tr.X, Y: br.Y - tr.Y}

	return [4]Point{
		{X: tl.X + insetX*vTop.X + insetY*vLeft.X, Y: tl.Y + insetX*vTop.Y + insetY*vLeft.Y},
		{X: tr.X - insetX*vTop.X + insetY*vRight.X, Y: tr.Y - insetX*vTop.Y + insetY*vRight.Y},
		{X: br.X - insetX*vBot.X - insetY*vRight.X, Y: br.Y - insetX*vBot.Y - insetY*vRight.Y},
		{X: bl.X + insetX*vBot.X - insetY*vLeft.X, Y: bl.Y + insetX*vBot.Y - insetY*vLeft.Y},
	}
}
