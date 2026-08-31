package sgf

import (
	"testing"
)

func TestParseBasic(t *testing.T) {
	sgfStr := `(;GM[1]FF[4]SZ[19]KM[6.5]PB[Honinbo Shusaku]PW[Honinbo Shuwa];B[qd];W[dc])`
	root, err := Parse(sgfStr)
	if err != nil {
		t.Fatalf("Failed to parse basic SGF: %v", err)
	}

	if root == nil {
		t.Fatal("Root node is nil")
	}

	// Verify root properties
	meta := ExtractMetadata(root)
	if meta.Size != 19 {
		t.Errorf("Expected size 19, got %d", meta.Size)
	}
	if meta.Komi != 6.5 {
		t.Errorf("Expected komi 6.5, got %f", meta.Komi)
	}
	if meta.BlackPlayer != "Honinbo Shusaku" {
		t.Errorf("Expected Black Player 'Honinbo Shusaku', got %q", meta.BlackPlayer)
	}
	if meta.WhitePlayer != "Honinbo Shuwa" {
		t.Errorf("Expected White Player 'Honinbo Shuwa', got %q", meta.WhitePlayer)
	}

	// Verify tree structure
	if len(root.Children()) != 1 {
		t.Fatalf("Expected 1 child for root, got %d", len(root.Children()))
	}

	bMove := root.Children()[0]
	if val, ok := bMove.GetValue("B"); !ok || val != "qd" {
		t.Errorf("Expected B[qd], got B[%s]", val)
	}

	if len(bMove.Children()) != 1 {
		t.Fatalf("Expected 1 child for B move, got %d", len(bMove.Children()))
	}

	wMove := bMove.Children()[0]
	if val, ok := wMove.GetValue("W"); !ok || val != "dc" {
		t.Errorf("Expected W[dc], got W[%s]", val)
	}
}

func TestParseBranches(t *testing.T) {
	sgfStr := `(;GM[1]SZ[19]
;B[qd]
(;W[dc];B[pq])
(;W[od];B[oc]))`

	root, err := Parse(sgfStr)
	if err != nil {
		t.Fatalf("Failed to parse SGF with branches: %v", err)
	}

	// Root -> B[qd]
	if len(root.Children()) != 1 {
		t.Fatalf("Expected 1 child for root, got %d", len(root.Children()))
	}
	bMove := root.Children()[0]

	// B[qd] -> should have 2 children (branches)
	if len(bMove.Children()) != 2 {
		t.Fatalf("Expected 2 children (branches) for B[qd], got %d", len(bMove.Children()))
	}

	branch1 := bMove.Children()[0] // W[dc]
	branch2 := bMove.Children()[1] // W[od]

	if val, ok := branch1.GetValue("W"); !ok || val != "dc" {
		t.Errorf("Branch 1: Expected W[dc], got W[%s]", val)
	}
	if val, ok := branch2.GetValue("W"); !ok || val != "od" {
		t.Errorf("Branch 2: Expected W[od], got W[%s]", val)
	}
}

func TestParseInvalid(t *testing.T) {
	invalidSgfs := []string{
		"",
		"   ",
		"GM[1]",   // No node definition
		";GM[1])", // Missing opening parenthesis
	}

	for i, sgfStr := range invalidSgfs {
		_, err := Parse(sgfStr)
		if err == nil {
			t.Errorf("Expected error for invalid SGF case %d, but got nil", i)
		}
	}
}

func TestCleanDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"YYYY-MM-DD", "2026-03-19", "2026-03-19"},
		{"YYYY/MM/DD", "2026/03/19", "2026-03-19"},
		{"YYYY-MM-DD with time", "2026-03-19 02:02:54", "2026-03-19"},
		{"YYYYMMDD", "20260319", "2026-03-19"},
		{"Year only YYYY", "2026", "2026-01-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanDate(tt.input)
			if got != tt.expected {
				t.Errorf("cleanDate(%q) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}

	// Empty string should return non-empty valid date (today's date)
	emptyDate := cleanDate("")
	if len(emptyDate) != 10 {
		t.Errorf("cleanDate(\"\") should return YYYY-MM-DD format, got %q", emptyDate)
	}
}

func TestGenerateBoardImage(t *testing.T) {
	sgfStr := `(;GM[1]FF[4]SZ[19]KM[6.5]PB[Honinbo Shusaku]PW[Honinbo Shuwa];B[qd];W[dc];B[pq];W[cp])`
	root, err := Parse(sgfStr)
	if err != nil {
		t.Fatalf("Failed to parse SGF: %v", err)
	}

	meta := ExtractMetadata(root)
	lastNode := root.GetEnd()
	board := lastNode.Board()

	img := GenerateBoardImage(board, root, meta.Size)
	if img == nil {
		t.Fatal("GenerateBoardImage returned nil")
	}

	bounds := img.Bounds()
	if bounds.Dx() != 1200 || bounds.Dy() != 630 {
		t.Errorf("Expected 1200x630 image dimensions, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
