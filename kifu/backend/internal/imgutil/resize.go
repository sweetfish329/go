package imgutil

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
)

// CompressToJPEG decodes an image, downscales it to maxWidth if its width exceeds maxWidth (maintaining aspect ratio),
// and compresses it as a JPEG with the specified quality.
func CompressToJPEG(srcData []byte, maxWidth int, quality int) ([]byte, error) {
	srcImg, err := decodeImage(srcData)
	if err != nil {
		return nil, err
	}

	bounds := srcImg.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("invalid image bounds: %v", bounds)
	}

	targetW := srcW
	targetH := srcH
	if maxWidth > 0 && srcW > maxWidth {
		targetW = maxWidth
		targetH = (srcH * maxWidth) / srcW
		if targetH <= 0 {
			targetH = 1
		}
	}

	var finalImg image.Image = srcImg
	if targetW != srcW || targetH != srcH {
		dstImg := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
		draw.CatmullRom.Scale(dstImg, dstImg.Bounds(), srcImg, bounds, draw.Over, nil)
		finalImg = dstImg
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, finalImg, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, fmt.Errorf("failed to encode jpeg: %w", err)
	}

	return buf.Bytes(), nil
}

// ResizeAndCompressToJPEG decodes an image, resizes it to the specified width (maintaining aspect ratio),
// and compresses it as a JPEG with the specified quality.
func ResizeAndCompressToJPEG(srcData []byte, width int, quality int) ([]byte, error) {
	srcImg, err := decodeImage(srcData)
	if err != nil {
		return nil, err
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

func decodeImage(srcData []byte) (image.Image, error) {
	reader := bytes.NewReader(srcData)
	contentType := detectContentType(srcData)
	switch contentType {
	case "image/png":
		img, err := png.Decode(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to decode png: %w", err)
		}
		return img, nil
	case "image/jpeg":
		img, err := jpeg.Decode(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to decode jpeg: %w", err)
		}
		return img, nil
	default:
		img, _, err := image.Decode(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %w", err)
		}
		return img, nil
	}
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
