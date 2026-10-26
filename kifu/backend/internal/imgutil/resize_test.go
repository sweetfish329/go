package imgutil

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"
)

func createTestImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{200, 100, 50, 255}}, image.Point{}, draw.Src)
	return img
}

func TestCompressToWebP_Invalid(t *testing.T) {
	invalidData := []byte("this is not an image")
	_, err := CompressToWebP(invalidData, 200)
	if err == nil {
		t.Error("Expected error for invalid image data, got nil")
	}
}

func TestEncodeToWebP(t *testing.T) {
	srcImg := createTestImage(400, 400)
	webpBytes, err := EncodeToWebP(srcImg)
	if err != nil {
		t.Fatalf("EncodeToWebP failed: %v", err)
	}

	contentType := detectContentType(webpBytes)
	if contentType != "image/webp" {
		t.Errorf("Expected content type 'image/webp', got %q", contentType)
	}

	decoded, err := decodeImage(webpBytes)
	if err != nil {
		t.Fatalf("decodeImage failed for WebP: %v", err)
	}
	if decoded.Bounds().Dx() != 400 || decoded.Bounds().Dy() != 400 {
		t.Errorf("Expected 400x400, got %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}

func TestCompressToWebP(t *testing.T) {
	srcImgLarge := createTestImage(1200, 600)
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, srcImgLarge); err != nil {
		t.Fatalf("Failed to encode PNG: %v", err)
	}

	webpBytes, err := CompressToWebP(pngBuf.Bytes(), 600)
	if err != nil {
		t.Fatalf("CompressToWebP failed: %v", err)
	}

	decoded, err := decodeImage(webpBytes)
	if err != nil {
		t.Fatalf("decodeImage failed for compressed WebP: %v", err)
	}
	if decoded.Bounds().Dx() != 600 || decoded.Bounds().Dy() != 300 {
		t.Errorf("Expected 600x300, got %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}
