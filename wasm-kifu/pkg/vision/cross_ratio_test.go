package vision

import (
	"math"
	"testing"
)

func TestExtrapolateCoordViaCrossRatio_Affine(t *testing.T) {
	// 等間隔 (正面視点): step = 20px
	// index 0 -> 100, index 18 -> 460
	known := []GridLineWithIndex{
		{Index: 3, Coord: 160},
		{Index: 9, Coord: 280},
		{Index: 15, Coord: 400},
	}

	// 消失点が無限遠
	c0, err := ExtrapolateCoordViaCrossRatio(known, math.MaxFloat64, 0.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(c0-100.0) > 1e-3 {
		t.Errorf("expected 100.0, got %.3f", c0)
	}

	c18, err := ExtrapolateCoordViaCrossRatio(known, math.MaxFloat64, 18.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(c18-460.0) > 1e-3 {
		t.Errorf("expected 460.0, got %.3f", c18)
	}
}

func TestExtrapolateCoordViaCrossRatio_Perspective(t *testing.T) {
	// 透視投影モデル:
	// 盤面上の座標 Y_board = t (0 <= t <= 18)
	// カメラ原点からの消失点 V = -100.0
	// 画像上の座標 y = V * t / (t - d) などの射影写像
	// ここでは直接 1D 射影変換写像 f(t) = (a*t + b) / (c*t + d) を定義
	// a=50, b=200, c=0.05, d=1
	f := func(t float64) float64 {
		return (50.0*t + 200.0) / (0.05*t + 1.0)
	}
	// 消失点 (t -> inf): V = a / c = 50 / 0.05 = 1000.0
	vCoord := 1000.0

	// 内部の線 (手前・奥が遮蔽されていて、3, 7, 11, 14 しか見えない状況)
	known := []GridLineWithIndex{
		{Index: 3, Coord: f(3)},
		{Index: 7, Coord: f(7)},
		{Index: 11, Coord: f(11)},
		{Index: 14, Coord: f(14)},
	}

	true0 := f(0)
	true18 := f(18)

	c0, err := ExtrapolateCoordViaCrossRatio(known, vCoord, 0.0)
	if err != nil {
		t.Fatalf("extrapolate 0 error: %v", err)
	}
	if math.Abs(c0-true0) > 1e-2 {
		t.Errorf("Perspective extrapolation c0 mismatch: got %.4f, want %.4f", c0, true0)
	}

	c18, err := ExtrapolateCoordViaCrossRatio(known, vCoord, 18.0)
	if err != nil {
		t.Fatalf("extrapolate 18 error: %v", err)
	}
	if math.Abs(c18-true18) > 1e-2 {
		t.Errorf("Perspective extrapolation c18 mismatch: got %.4f, want %.4f", c18, true18)
	}
}

func TestEstimateVanishingPoint(t *testing.T) {
	// 2本の直線が (200, -300) で交わる
	l1 := LineSegment{P1: Point{X: 100, Y: 100}, P2: Point{X: 150, Y: -100}}
	l2 := LineSegment{P1: Point{X: 300, Y: 100}, P2: Point{X: 250, Y: -100}}

	vp, err := EstimateVanishingPoint([]LineSegment{l1, l2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if math.Abs(vp.X-200.0) > 1e-2 || math.Abs(vp.Y-(-300.0)) > 1e-2 {
		t.Errorf("Vanishing point mismatch: got (%.2f, %.2f), want (200, -300)", vp.X, vp.Y)
	}
}
