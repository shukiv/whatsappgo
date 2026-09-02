// Package mediaformat adapts WhatsApp media to formats the desktop renderer
// can display without optional Qt image plugins.
package mediaformat

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/webp"
)

const maxStickerBytes = 16 << 20

// StickerPNG returns a PNG cache path for a WebP sticker. Static WebP files
// are decoded directly. For animated stickers, Qt's missing WebP plugin means
// animation is unavailable, so the first frame is used as a stable fallback.
// The source file is retained because it remains the durable WhatsApp media.
func StickerPNG(path string) (string, error) {
	if path == "" || !strings.EqualFold(filepath.Ext(path), ".webp") {
		return path, nil
	}
	target := strings.TrimSuffix(path, filepath.Ext(path)) + ".png"
	if info, err := os.Stat(target); err == nil && info.Size() > 0 {
		return target, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > maxStickerBytes {
		return "", errors.New("sticker is too large to convert")
	}
	decoded, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		frame, frameErr := firstAnimatedFrame(data)
		if frameErr != nil {
			return "", err
		}
		decoded, err = webp.Decode(bytes.NewReader(frame))
		if err != nil {
			return "", err
		}
	}
	return writePNG(target, decoded)
}

func writePNG(target string, source image.Image) (string, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), "sticker-*.png")
	if err != nil {
		return "", err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := png.Encode(temporary, source); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return "", err
	}
	return target, nil
}

// firstAnimatedFrame turns the first ANMF payload into a standalone WebP.
// ANMF starts with a 16-byte placement/timing header followed by ordinary
// ALPH+VP8 or VP8L chunks. A small VP8X canvas header makes those chunks
// independently decodable.
func firstAnimatedFrame(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, errors.New("invalid WebP container")
	}
	for offset := 12; offset+8 <= len(data); {
		length := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		end := offset + 8 + length
		if length < 0 || end > len(data) {
			return nil, errors.New("invalid WebP chunk")
		}
		if string(data[offset:offset+4]) == "ANMF" {
			payload := data[offset+8 : end]
			if len(payload) < 24 {
				return nil, errors.New("invalid animated WebP frame")
			}
			widthMinusOne := payload[6:9]
			heightMinusOne := payload[9:12]
			chunks := append([]byte(nil), payload[16:]...)
			flags := byte(0)
			if bytes.Contains(chunks, []byte("ALPH")) || bytes.Contains(chunks, []byte("VP8L")) {
				flags = 1 << 4
			}
			vp8x := []byte{'V', 'P', '8', 'X', 10, 0, 0, 0, flags, 0, 0, 0,
				widthMinusOne[0], widthMinusOne[1], widthMinusOne[2],
				heightMinusOne[0], heightMinusOne[1], heightMinusOne[2]}
			body := append([]byte("WEBP"), vp8x...)
			body = append(body, chunks...)
			result := make([]byte, 8, 8+len(body))
			copy(result, "RIFF")
			binary.LittleEndian.PutUint32(result[4:8], uint32(len(body)))
			return append(result, body...), nil
		}
		offset = end + length%2
	}
	return nil, errors.New("animated WebP has no frame")
}
