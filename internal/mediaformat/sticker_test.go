package mediaformat

import (
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

const onePixelWebP = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="

func TestStickerPNGConvertsStaticWebP(t *testing.T) {
	// A one-pixel, lossless WebP fixture.
	data, err := base64.StdEncoding.DecodeString(onePixelWebP)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sticker.webp")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	converted, err := StickerPNG(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(converted) != ".png" {
		t.Fatalf("converted path = %q", converted)
	}
	if info, err := os.Stat(converted); err != nil || info.Size() == 0 {
		t.Fatalf("converted sticker was not written: info=%v err=%v", info, err)
	}
}

func TestStickerPNGUsesFirstAnimatedFrame(t *testing.T) {
	frame, err := base64.StdEncoding.DecodeString(onePixelWebP)
	if err != nil {
		t.Fatal(err)
	}
	framePayload := make([]byte, 16, 16+len(frame)-12)
	framePayload[12] = 100 // frame duration, little endian
	framePayload = append(framePayload, frame[12:]...)
	vp8x := []byte{'V', 'P', '8', 'X', 10, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	anim := []byte{'A', 'N', 'I', 'M', 6, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	anmf := make([]byte, 8, 8+len(framePayload))
	copy(anmf, "ANMF")
	binary.LittleEndian.PutUint32(anmf[4:8], uint32(len(framePayload)))
	anmf = append(anmf, framePayload...)
	body := append([]byte("WEBP"), vp8x...)
	body = append(body, anim...)
	body = append(body, anmf...)
	animated := make([]byte, 8, 8+len(body))
	copy(animated, "RIFF")
	binary.LittleEndian.PutUint32(animated[4:8], uint32(len(body)))
	animated = append(animated, body...)

	path := filepath.Join(t.TempDir(), "animated.webp")
	if err := os.WriteFile(path, animated, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := StickerPNG(path); err != nil {
		t.Fatal(err)
	}
}

func TestStickerPNGLeavesOtherFormatsAlone(t *testing.T) {
	const path = "/cache/sticker.png"
	converted, err := StickerPNG(path)
	if err != nil || converted != path {
		t.Fatalf("StickerPNG(%q) = %q, %v", path, converted, err)
	}
}
