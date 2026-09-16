package vision

import (
	"math"
	"testing"
)

func TestRationalProjectivePitch_EqualPitch(t *testing.T) {
	boardH := 540.0
	pitch := RationalProjectivePitch(boardH, 30.0, 30.0)

	if len(pitch) != 19 {
		t.Fatalf("expected 19 elements, got %d", len(pitch))
	}
	if pitch[0] != 0.0 {
		t.Errorf("pitch[0] = %.1f, want 0.0", pitch[0])
	}
	if math.Abs(pitch[18]-boardH) > 1e-4 {
		t.Errorf("pitch[18] = %.1f, want %.1f", pitch[18], boardH)
	}
	// 均等間隔の検証 (各マス 30px)
	for i := 0; i < 18; i++ {
		d := pitch[i+1] - pitch[i]
		if math.Abs(d-30.0) > 1e-4 {
			t.Errorf("row %d delta = %.4f, want 30.0", i, d)
		}
	}
}

func TestRationalProjectivePitch_PerspectiveGradient(t *testing.T) {
	boardH := 600.0
	// 奥 28px, 手前 38px
	pitch := RationalProjectivePitch(boardH, 28.0, 38.0)

	if pitch[0] != 0.0 {
		t.Errorf("pitch[0] = %.1f, want 0.0", pitch[0])
	}
	if math.Abs(pitch[18]-boardH) > 1e-4 {
		t.Errorf("pitch[18] = %.1f, want %.1f", pitch[18], boardH)
	}

	// 各マスの間隔が単調増加しているか検証
	var prevDelta float64
	for i := 0; i < 18; i++ {
		d := pitch[i+1] - pitch[i]
		if i > 0 && d <= prevDelta {
			t.Errorf("row %d delta = %.4f not increasing from prev %.4f", i, d, prevDelta)
		}
		prevDelta = d
	}
}
