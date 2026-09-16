package vision

import (
	"math"
	"testing"
)

func TestDivisionModelRoundTrip(t *testing.T) {
	w, h := 1000.0, 1000.0
	k1 := -0.15 // 樽型歪み

	cam := NewDivisionModelCamera(w, h, k1)

	testPoints := []Point{
		{X: 500, Y: 500}, // 中心
		{X: 500, Y: 100}, // 上辺
		{X: 100, Y: 500}, // 左辺
		{X: 900, Y: 900}, // コーナー
		{X: 250, Y: 750}, // 任意点
	}

	for i, pt := range testPoints {
		pu := cam.UndistortPoint(pt)
		pd := cam.DistortPoint(pu)

		errDist := math.Hypot(pd.X-pt.X, pd.Y-pt.Y)
		if errDist > 1e-4 {
			t.Errorf("Point %d round-trip error too large: got (%.4f, %.4f), want (%.4f, %.4f), err=%.6f",
				i, pd.X, pd.Y, pt.X, pt.Y, errDist)
		}
	}
}

func TestDivisionModelStraightening(t *testing.T) {
	w, h := 1000.0, 1000.0
	k1 := -0.20

	cam := NewDivisionModelCamera(w, h, k1)

	// 本来は直線 Y = 200 上にある3点 (x=200, 500, 800)
	trueLine := []Point{
		{X: 200, Y: 200},
		{X: 500, Y: 200},
		{X: 800, Y: 200},
	}

	// 樽型歪みにより弓なりに曲がった観測点
	var distortedLine []Point
	for _, p := range trueLine {
		distortedLine = append(distortedLine, cam.DistortPoint(p))
	}

	// 歪んだ中間点 (X=500) は直線から逸脱している
	dMid := distortedLine[1]
	straightMidY := (distortedLine[0].Y + distortedLine[2].Y) / 2.0
	sagittaDistorted := math.Abs(dMid.Y - straightMidY)
	if sagittaDistorted < 5.0 {
		t.Errorf("Expected significant curvature in distorted points, got sagitta %.2f", sagittaDistorted)
	}

	// 不歪化を適用すると、真の直線 Y=200 に戻る
	var restoredLine []Point
	for _, p := range distortedLine {
		restoredLine = append(restoredLine, cam.UndistortPoint(p))
	}

	for i, p := range restoredLine {
		errDist := math.Hypot(p.X-trueLine[i].X, p.Y-trueLine[i].Y)
		if errDist > 1e-3 {
			t.Errorf("Restored point %d mismatch: got (%.3f, %.3f), want (%.3f, %.3f)",
				i, p.X, p.Y, trueLine[i].X, trueLine[i].Y)
		}
	}
}
