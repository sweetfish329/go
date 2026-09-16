// Package vision は碁盤画像の多次元特徴量解析および幾何認識ロジックを提供する。
package vision

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

// GetExifOrientation は JPEG ファイルの APP1 (Exif) セグメントから TIFF Orientation タグ (0x0112) を解析して返す。
// Exif が存在しないか Orientation が未設定の場合は 1 (通常) を返す。
func GetExifOrientation(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 1
	}
	defer f.Close()

	header := make([]byte, 64*1024)
	n, err := f.Read(header)
	if err != nil || n < 4 {
		return 1
	}

	data := header[:n]
	if data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}

	idx := 2
	for idx+4 <= len(data) {
		if data[idx] != 0xFF {
			break
		}
		marker := data[idx+1]
		if marker == 0xDA || marker == 0xD9 { // SOS または EOI
			break
		}
		length := int(data[idx+2])<<8 | int(data[idx+3])
		if length < 2 || idx+2+length > len(data) {
			break
		}

		if marker == 0xE1 && length >= 14 { // APP1 Exif
			exifData := data[idx+4 : idx+2+length]
			if len(exifData) >= 6 && string(exifData[:6]) == "Exif\x00\x00" {
				tiff := exifData[6:]
				if len(tiff) >= 8 {
					bigEndian := tiff[0] == 'M' && tiff[1] == 'M'
					read16 := func(b []byte) uint16 {
						if bigEndian {
							return uint16(b[0])<<8 | uint16(b[1])
						}
						return uint16(b[1])<<8 | uint16(b[0])
					}
					read32 := func(b []byte) uint32 {
						if bigEndian {
							return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
						}
						return uint32(b[3])<<24 | uint32(b[2])<<16 | uint32(b[1])<<8 | uint32(b[0])
					}
					ifdOffset := int(read32(tiff[4:8]))
					if ifdOffset >= 0 && ifdOffset < len(tiff)-2 {
						numEntries := int(read16(tiff[ifdOffset:]))
						pos := ifdOffset + 2
						for e := 0; e < numEntries && pos+12 <= len(tiff); e++ {
							tag := read16(tiff[pos:])
							if tag == 0x0112 { // Orientation tag
								return int(read16(tiff[pos+8:]))
							}
							pos += 12
						}
					}
				}
			}
		}
		idx += 2 + length
	}
	return 1
}

// LoadImageUpright は画像ファイルを読み込み、EXIF Orientation に応じて正立画像に自動回転して返す。
func LoadImageUpright(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	raw, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	orient := GetExifOrientation(path)
	b := raw.Bounds()
	w, h := b.Dx(), b.Dy()

	// Orientation 6: 時計回り90度回転 (スマホ縦持ち撮影時の標準)
	if orient == 6 {
		upright := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				upright.Set(h-1-y, x, raw.At(x, y))
			}
		}
		return upright, nil
	}
	// Orientation 8: 反時計回り90度回転
	if orient == 8 {
		upright := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				upright.Set(y, w-1-x, raw.At(x, y))
			}
		}
		return upright, nil
	}
	// Orientation 3: 180度回転
	if orient == 3 {
		upright := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				upright.Set(w-1-x, h-1-y, raw.At(x, y))
			}
		}
		return upright, nil
	}

	return raw, nil
}
