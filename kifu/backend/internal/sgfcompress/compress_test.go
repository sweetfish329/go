package sgfcompress

import (
	"testing"
)

func TestCompressAndDecompress(t *testing.T) {
	sgfText := `(;GM[1]FF[4]CA[UTF-8]AP[Sabaki:0.52.2]SZ[19]KM[6.5]RU[Japanese]PW[WhitePlayer]PB[BlackPlayer];B[pd];W[dp];B[pp];W[dd];B[fc];W[cf];B[jd];W[qf];B[nc];W[qn];B[nq];W[rp];B[qq];W[qk];B[qi];W[ok];B[oi];W[pm];B[cq];W[cp];B[dq];W[ep];B[eq];W[fq];B[fp];W[gq];B[fo];W[dn];B[en];W[dm];B[em];W[el];B[fl];W[ek];B[fk];W[ej];B[fj];W[ei];B[fi];W[eh];B[fh];W[eg];B[fg];W[ef];B[ff];W[fe];B[ge];W[fd];B[gd];W[gc];B[hc];W[gb];B[hb];W[fb];B[ec];W[eb];B[ed];W[ee];B[de];W[cd];B[ce];W[be];B[df];W[dg];B[cg];W[bg];B[ch];W[bh];B[ci];W[bi];B[cj];W[bj];B[ck];W[bk];B[cl];W[bl];B[cm];W[bm];B[cn];W[bn];B[co];W[bo];B[do];W[bq];B[br];W[ar];B[bp];W[ap];B[bs];W[as];B[cr];W[aq])`

	compressed, err := Compress(sgfText)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if len(compressed) >= len(sgfText) {
		t.Errorf("Expected compressed length (%d) to be less than original (%d)", len(compressed), len(sgfText))
	}

	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if decompressed != sgfText {
		t.Errorf("Decompressed text did not match original.\nGot: %s\nWant: %s", decompressed, sgfText)
	}
}

func TestLegacyPlainTextCompatibility(t *testing.T) {
	legacyPlainText := "(;GM[1]SZ[19]KM[6.5];B[dp];W[pd])"

	// Decompress should transparently return plain text if not zstd compressed
	decompressed, err := Decompress([]byte(legacyPlainText))
	if err != nil {
		t.Fatalf("Decompress failed on plain text: %v", err)
	}

	if decompressed != legacyPlainText {
		t.Errorf("Decompressed text did not match legacy plain text.\nGot: %s\nWant: %s", decompressed, legacyPlainText)
	}
}

func TestEmptyData(t *testing.T) {
	compressed, err := Compress("")
	if err != nil {
		t.Fatalf("Compress failed on empty string: %v", err)
	}
	if len(compressed) != 0 {
		t.Errorf("Expected empty byte slice, got length %d", len(compressed))
	}

	decompressed, err := Decompress([]byte{})
	if err != nil {
		t.Fatalf("Decompress failed on empty slice: %v", err)
	}
	if decompressed != "" {
		t.Errorf("Expected empty string, got: %s", decompressed)
	}
}
