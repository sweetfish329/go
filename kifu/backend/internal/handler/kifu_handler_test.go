package handler

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
)

func TestPrepareKifuFromSgf(t *testing.T) {
	sgfData := `(;GM[1]FF[4]SZ[19]KM[6.5]PB[Honinbo Shusaku]PW[Honinbo Shuwa];B[qd];W[dc])`

	kifu, err := PrepareKifuFromSgf(sgfData, "Custom Title", "user-1", false)
	if err != nil {
		t.Fatalf("PrepareKifuFromSgf failed: %v", err)
	}

	if kifu.Title != "Custom Title" {
		t.Errorf("Expected title 'Custom Title', got %q", kifu.Title)
	}
	if kifu.BlackPlayer != "Honinbo Shusaku" {
		t.Errorf("Expected black player 'Honinbo Shusaku', got %q", kifu.BlackPlayer)
	}
	if kifu.WhitePlayer != "Honinbo Shuwa" {
		t.Errorf("Expected white player 'Honinbo Shuwa', got %q", kifu.WhitePlayer)
	}
	if kifu.UploadedBy == nil || *kifu.UploadedBy != "user-1" {
		t.Errorf("Expected uploaded_by 'user-1', got %v", kifu.UploadedBy)
	}
	if !kifu.IsPrivate {
		t.Error("Expected default is_private to be true")
	}
	if !kifu.HasOgp || len(kifu.OgpImage) == 0 {
		t.Error("Expected OgpImage to be auto-generated and HasOgp true")
	}
	// Verify OgpImage is valid PNG (starts with \x89PNG\r\n\x1a\n)
	if len(kifu.OgpImage) < 8 || !bytes.Equal(kifu.OgpImage[:8], []byte("\x89PNG\r\n\x1a\n")) {
		t.Error("Expected OgpImage to be in PNG format")
	}
	if len(kifu.ThumbnailImage) == 0 {
		t.Error("Expected ThumbnailImage to be auto-generated")
	}
	// Verify ThumbnailImage is valid WebP (starts with RIFF....WEBP)
	if len(kifu.ThumbnailImage) < 12 || !bytes.Equal(kifu.ThumbnailImage[:4], []byte("RIFF")) || !bytes.Equal(kifu.ThumbnailImage[8:12], []byte("WEBP")) {
		t.Error("Expected ThumbnailImage to be in WebP format")
	}
}

func TestPrepareKifuFromSgf_AutoTitle(t *testing.T) {
	sgfData := `(;GM[1]FF[4]SZ[19]KM[6.5]PB[Alpha]PW[Beta];B[qd];W[dc])`

	kifu, err := PrepareKifuFromSgf(sgfData, "", "user-1", true)
	if err != nil {
		t.Fatalf("PrepareKifuFromSgf failed: %v", err)
	}

	if kifu.Title != "Alpha vs Beta" {
		t.Errorf("Expected auto-generated title 'Alpha vs Beta', got %q", kifu.Title)
	}
	if kifu.UploadedBy != nil {
		t.Errorf("Expected uploaded_by nil for admin, got %v", kifu.UploadedBy)
	}
}

func TestBuildKifuOgpMeta(t *testing.T) {
	kifu := &model.Kifu{
		Title:       "Test Match",
		BlackPlayer: "Player A",
		WhitePlayer: "Player B",
		Result:      "B+R",
		IsPrivate:   false,
		UpdatedAt:   time.Unix(1700000000, 0),
	}

	meta := BuildKifuOgpMeta(kifu, "u-123", "k-456", "https", "example.com", "kifu_store")
	if !strings.Contains(meta, `og:title" content="Test Match | kifu_store"`) {
		t.Errorf("Expected og:title to contain 'Test Match | kifu_store', got: %s", meta)
	}
	if !strings.Contains(meta, `og:url" content="https://example.com/u/u-123/k-456"`) {
		t.Errorf("Expected og:url to contain public path, got: %s", meta)
	}
	if !strings.Contains(meta, `robots" content="index, follow"`) {
		t.Errorf("Expected robots index, follow for public kifu, got: %s", meta)
	}
}

func TestBuildSharedKifuOgpMeta(t *testing.T) {
	kifu := &model.Kifu{
		Title:       "Secret Match",
		BlackPlayer: "Player A",
		WhitePlayer: "Player B",
		Result:      "W+1.5",
		IsPrivate:   true,
		UpdatedAt:   time.Unix(1700000000, 0),
	}

	meta := BuildSharedKifuOgpMeta(kifu, "share-token-abc", "https", "example.com", "kifu_store")
	if !strings.Contains(meta, `robots" content="noindex, nofollow"`) {
		t.Errorf("Expected robots noindex, nofollow for shared kifu, got: %s", meta)
	}
	if !strings.Contains(meta, `share=share-token-abc`) {
		t.Errorf("Expected share token in URL, got: %s", meta)
	}
}
