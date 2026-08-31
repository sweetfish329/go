package imgutil

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"testing"
)

func createTestImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{200, 100, 50, 255}}, image.Point{}, draw.Src)
	return img
}

func TestResizeAndCompressToJPEG_PNG(t *testing.T) {
	srcImg := createTestImage(800, 600)
	var buf bytes.Buffer
	if err := png.Encode(&buf, srcImg); err != nil {
		t.Fatalf("Failed to encode test PNG: %v", err)
	}

	resizedBytes, err := ResizeAndCompressToJPEG(buf.Bytes(), 400, 80)
	if err != nil {
		t.Fatalf("ResizeAndCompressToJPEG failed for PNG: %v", err)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(resizedBytes))
	if err != nil {
		t.Fatalf("Failed to decode resized JPEG: %v", err)
	}

	bounds := decoded.Bounds()
	if bounds.Dx() != 400 {
		t.Errorf("Expected width 400, got %d", bounds.Dx())
	}
	if bounds.Dy() != 300 {
		t.Errorf("Expected height 300, got %d", bounds.Dy())
	}
}

func TestResizeAndCompressToJPEG_JPEG(t *testing.T) {
	srcImg := createTestImage(1000, 500)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, srcImg, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("Failed to encode test JPEG: %v", err)
	}

	resizedBytes, err := ResizeAndCompressToJPEG(buf.Bytes(), 200, 75)
	if err != nil {
		t.Fatalf("ResizeAndCompressToJPEG failed for JPEG: %v", err)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(resizedBytes))
	if err != nil {
		t.Fatalf("Failed to decode resized JPEG: %v", err)
	}

	bounds := decoded.Bounds()
	if bounds.Dx() != 200 {
		t.Errorf("Expected width 200, got %d", bounds.Dx())
	}
	if bounds.Dy() != 100 {
		t.Errorf("Expected height 100, got %d", bounds.Dy())
	}
}

func TestResizeAndCompressToJPEG_Invalid(t *testing.T) {
	invalidData := []byte("this is not an image")
	_, err := ResizeAndCompressToJPEG(invalidData, 200, 75)
	if err == nil {
		t.Error("Expected error for invalid image data, got nil")
	}
}
