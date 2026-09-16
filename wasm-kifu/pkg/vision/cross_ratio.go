package vision

import (
	"fmt"
	"math"
	"sort"
)

// LineSegment は2次元平面上の線分または直線を表す
type LineSegment struct {
	P1 Point
	P2 Point
}

// IntersectLines は2直線の交点を計算する (平行な場合は nil)
func IntersectLines(l1, l2 LineSegment) *Point {
	x1, y1 := l1.P1.X, l1.P1.Y
	x2, y2 := l1.P2.X, l1.P2.Y
	x3, y3 := l2.P1.X, l2.P1.Y
	x4, y4 := l2.P2.X, l2.P2.Y

	denom := (x1-x2)*(y3-y4) - (y1-y2)*(x3-x4)
	if math.Abs(denom) < 1e-6 {
		return nil
	}

	px := ((x1*y2-y1*x2)*(x3-x4) - (x1-x2)*(x3*y4-y3*x4)) / denom
	py := ((x1*y2-y1*x2)*(y3-y4) - (y1-y2)*(x3*y4-y3*x4)) / denom
	return &Point{X: px, Y: py}
}

// EstimateVanishingPoint は直線群から消失点 (Vanishing Point) をロバストに推定する
func EstimateVanishingPoint(lines []LineSegment) (*Point, error) {
	if len(lines) < 2 {
		return nil, fmt.Errorf("at least 2 lines required for vanishing point estimation")
	}

	var intersections []Point
	for i := 0; i < len(lines); i++ {
		for j := i + 1; j < len(lines); j++ {
			pt := IntersectLines(lines[i], lines[j])
			if pt != nil {
				// 有効な交点 (極端に遠すぎる発散値は除外)
				if math.Hypot(pt.X, pt.Y) < 1e6 {
					intersections = append(intersections, *pt)
				}
			}
		}
	}

	if len(intersections) == 0 {
		return nil, fmt.Errorf("parallel lines, vanishing point at infinity")
	}

	// 中央値 (Median) を採用して外れ値に頑健な消失点を決定
	var xs, ys []float64
	for _, pt := range intersections {
		xs = append(xs, pt.X)
		ys = append(ys, pt.Y)
	}
	sort.Float64s(xs)
	sort.Float64s(ys)

	medX := xs[len(xs)/2]
	medY := ys[len(ys)/2]
	return &Point{X: medX, Y: medY}, nil
}

// GridLineWithIndex は格子インデックス (0〜18) と観測された交点座標
type GridLineWithIndex struct {
	Index float64 // 物理格子インデックス (0.0 〜 18.0)
	Coord float64 // 1D投影軸上の観測座標 (例: X または Y)
}

// ExtrapolateCoordViaCrossRatio は複比 (Cross-Ratio) の射影保存則を用いて、
// 既知の格子線位置と消失点から、遮蔽された外周端線 (targetIndex: 0 または 18) の座標を数学的に厳密外挿する。
//
// 1次元射影座標において、消失点を V、任意の2本の格子線を (t1, c1), (t2, c2) とすると、
// 任意の未知インデックス targetIndex に対する投影座標 cTarget は複比保存則により解ける。
func ExtrapolateCoordViaCrossRatio(known []GridLineWithIndex, vCoord float64, targetIndex float64) (float64, error) {
	if len(known) < 2 {
		return 0, fmt.Errorf("at least 2 known lines required for cross-ratio extrapolation")
	}

	// ソート
	sorted := make([]GridLineWithIndex, len(known))
	copy(sorted, known)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Index < sorted[j].Index
	})

	// インデックス間隔が最大のペア (t1, c1) と (t2, c2) を選択して数値安定性を最大化
	t1, c1 := sorted[0].Index, sorted[0].Coord
	t2, c2 := sorted[len(sorted)-1].Index, sorted[len(sorted)-1].Coord

	if math.Abs(t2-t1) < 1e-4 || math.Abs(c2-c1) < 1e-4 {
		return 0, fmt.Errorf("lines are too close to each other")
	}

	// 消失点が無限遠の場合 (正射影 / 等間隔)
	if math.Abs(vCoord) > 1e5 || math.IsInf(vCoord, 0) {
		step := (c2 - c1) / (t2 - t1)
		return c1 + (targetIndex-t1)*step, nil
	}

	// 複比モデル:
	// CR(t1, t2; target, inf) = (target - t1) / (target - t2)
	// CR(c1, c2; cTarget, V) = ((cTarget - c1) * (V - c2)) / ((cTarget - c2) * (V - c1))
	// 両者が等しいことを用いて cTarget を解く:
	// K = (target - t1) / (target - t2)
	// R = (V - c2) / (V - c1)
	// ((cTarget - c1) / (cTarget - c2)) * R = K
	// (cTarget - c1) / (cTarget - c2) = K / R  (= M)
	// cTarget - c1 = M * (cTarget - c2)
	// cTarget * (1 - M) = c1 - M * c2
	// cTarget = (c1 - M * c2) / (1 - M)

	if math.Abs(targetIndex-t2) < 1e-4 {
		return c2, nil
	}
	if math.Abs(targetIndex-t1) < 1e-4 {
		return c1, nil
	}

	k := (targetIndex - t1) / (targetIndex - t2)
	denomV := vCoord - c1
	if math.Abs(denomV) < 1e-4 {
		denomV = 1e-4
	}
	r := (vCoord - c2) / denomV

	if math.Abs(r) < 1e-6 {
		// 消失点と重なる特異点フォールバック
		step := (c2 - c1) / (t2 - t1)
		return c1 + (targetIndex-t1)*step, nil
	}

	m := k / r
	if math.Abs(1.0-m) < 1e-6 {
		// 分母ゼロ回避
		step := (c2 - c1) / (t2 - t1)
		return c1 + (targetIndex-t1)*step, nil
	}

	cTarget := (c1 - m*c2) / (1.0 - m)
	return cTarget, nil
}

// ExtrapolateMissingCornersFromInteriorLines は対局者の手や石で4隅が遮蔽された場合に、
// 観測された内側の格子線群から消失点と複比を用いて欠損した4隅 [TL, TR, BR, BL] を数学的に復元する
func ExtrapolateMissingCornersFromInteriorLines(
	horizLines []GridLineWithIndex, vertLines []GridLineWithIndex,
	vHoriz, vVert *Point,
) ([4]Point, error) {
	if len(horizLines) < 2 || len(vertLines) < 2 {
		return [4]Point{}, fmt.Errorf("insufficient lines to extrapolate corners")
	}

	// 水平線の 0番 (天辺) と 18番 (底辺) を外挿
	vY := math.MaxFloat64
	if vHoriz != nil {
		vY = vHoriz.Y
	}
	y0, err0 := ExtrapolateCoordViaCrossRatio(horizLines, vY, 0.0)
	y18, err18 := ExtrapolateCoordViaCrossRatio(horizLines, vY, 18.0)
	if err0 != nil || err18 != nil {
		return [4]Point{}, fmt.Errorf("failed to extrapolate horizontal bounds")
	}

	// 垂直線の 0番 (左辺) と 18番 (右辺) を外挿
	vX := math.MaxFloat64
	if vVert != nil {
		vX = vVert.X
	}
	x0, errX0 := ExtrapolateCoordViaCrossRatio(vertLines, vX, 0.0)
	x18, errX18 := ExtrapolateCoordViaCrossRatio(vertLines, vX, 18.0)
	if errX0 != nil || errX18 != nil {
		return [4]Point{}, fmt.Errorf("failed to extrapolate vertical bounds")
	}

	return [4]Point{
		{X: x0, Y: y0},   // TL
		{X: x18, Y: y0},  // TR
		{X: x18, Y: y18}, // BR
		{X: x0, Y: y18},  // BL
	}, nil
}
