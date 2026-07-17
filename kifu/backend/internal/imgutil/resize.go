package imgutil

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
)

// ResizeAndCompressToJPEG decodes an image, resizes it to the specified width (maintaining aspect ratio),
// and compresses it as a JPEG with the specified quality.
func ResizeAndCompressToJPEG(srcData []byte, width int, quality int) ([]byte, error) {
	var srcImg image.Image
	var err error

	// Detect format and decode
	reader := bytes.NewReader(srcData)
	contentType := detectContentType(srcData)
	switch contentType {
	case "image/png":
		srcImg, err = png.Decode(reader)
	case "image/jpeg":
		srcImg, err = jpeg.Decode(reader)
	default:
		// Fallback decode
		srcImg, _, err = image.Decode(reader)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := srcImg.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("invalid image bounds: %v", bounds)
	}

	// Calculate height to maintain aspect ratio
	height := (srcH * width) / srcW
	if height <= 0 {
		height = 1
	}

	dstImg := image.NewRGBA(image.Rect(0, 0, width, height))
	// CatmullRom is high quality but relatively fast
	draw.CatmullRom.Scale(dstImg, dstImg.Bounds(), srcImg, bounds, draw.Over, nil)

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, dstImg, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, fmt.Errorf("failed to encode jpeg: %w", err)
	}

	return buf.Bytes(), nil
}

func detectContentType(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return "image/png"
	}
	if len(data) >= 3 && bytes.Equal(data[:3], []byte("\xff\xd8\xff")) {
		return "image/jpeg"
	}
	return ""
}
