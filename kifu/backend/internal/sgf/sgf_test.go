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
