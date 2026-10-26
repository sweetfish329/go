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

func TestCompressToJPEG(t *testing.T) {
	// Image larger than maxWidth (1600 > 1200) -> downscale to 1200
	srcImgLarge := createTestImage(1600, 800)
	var bufLarge bytes.Buffer
	if err := png.Encode(&bufLarge, srcImgLarge); err != nil {
		t.Fatalf("Failed to encode large PNG: %v", err)
	}

	compressedLarge, err := CompressToJPEG(bufLarge.Bytes(), 1200, 85)
	if err != nil {
		t.Fatalf("CompressToJPEG failed for large image: %v", err)
	}
	decodedLarge, err := jpeg.Decode(bytes.NewReader(compressedLarge))
	if err != nil {
		t.Fatalf("Failed to decode compressed large image: %v", err)
	}
	if decodedLarge.Bounds().Dx() != 1200 || decodedLarge.Bounds().Dy() != 600 {
		t.Errorf("Expected 1200x600, got %dx%d", decodedLarge.Bounds().Dx(), decodedLarge.Bounds().Dy())
	}

	// Image smaller than maxWidth (800 < 1200) -> keep original size 800x400
	srcImgSmall := createTestImage(800, 400)
	var bufSmall bytes.Buffer
	if err := png.Encode(&bufSmall, srcImgSmall); err != nil {
		t.Fatalf("Failed to encode small PNG: %v", err)
	}

	compressedSmall, err := CompressToJPEG(bufSmall.Bytes(), 1200, 85)
	if err != nil {
		t.Fatalf("CompressToJPEG failed for small image: %v", err)
	}
	decodedSmall, err := jpeg.Decode(bytes.NewReader(compressedSmall))
	if err != nil {
		t.Fatalf("Failed to decode compressed small image: %v", err)
	}
	if decodedSmall.Bounds().Dx() != 800 || decodedSmall.Bounds().Dy() != 400 {
		t.Errorf("Expected 800x400, got %dx%d", decodedSmall.Bounds().Dx(), decodedSmall.Bounds().Dy())
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
