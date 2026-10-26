package sgfcompress

import (
	"bytes"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

// zstdMagic is the 4-byte little-endian magic number for Zstandard frames (0xFD2FB528).
var zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}

var (
	encoder *zstd.Encoder
	decoder *zstd.Decoder
)

func init() {
	var err error
	encoder, err = zstd.NewWriter(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create zstd encoder: %v", err))
	}
	decoder, err = zstd.NewReader(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create zstd decoder: %v", err))
	}
}

// Compress compresses an SGF string into zstd compressed bytes.
func Compress(sgf string) ([]byte, error) {
	if len(sgf) == 0 {
		return []byte{}, nil
	}
	return encoder.EncodeAll([]byte(sgf), make([]byte, 0, len(sgf)/2)), nil
}

// Decompress decompresses zstd-compressed bytes back into an SGF string.
// If the data is not zstd-compressed (e.g. legacy plain text SGF), it returns the string directly.
func Decompress(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	// Check if data starts with zstd magic header
	if len(data) >= 4 && bytes.Equal(data[:4], zstdMagic) {
		decompressed, err := decoder.DecodeAll(data, nil)
		if err != nil {
			return "", fmt.Errorf("failed to decompress zstd data: %w", err)
		}
		return string(decompressed), nil
	}

	// Fallback to legacy plain text SGF
	return string(data), nil
}
