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

func TestFirstAnimatedFrameRejectsMalformedChunks(t *testing.T) {
	riff := func(payload []byte) []byte {
		header := append([]byte("RIFF"), 0, 0, 0, 0)
		header = append(header, []byte("WEBP")...)
		return append(header, payload...)
	}
	cases := map[string][]byte{
		"truncated container": []byte("RIFF"),
		"not webp":            append([]byte("RIFFxxxx"), []byte("AVI ")...),
		// A chunk header that claims far more data than the file holds must be
		// rejected rather than producing an out-of-range slice.
		"chunk longer than file": riff(append([]byte("ANMF"), 0xFF, 0xFF, 0xFF, 0xFF)),
		"chunk overruns end":     riff(append([]byte("ANMF"), 0x40, 0x00, 0x00, 0x00)),
		"no frame":               riff(append([]byte("VP8 "), 0x00, 0x00, 0x00, 0x00)),
		"frame header too short": riff(append(append([]byte("ANMF"), 0x04, 0x00, 0x00, 0x00), 1, 2, 3, 4)),
	}
	for name, data := range cases {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("%s: panicked: %v", name, recovered)
				}
			}()
			if _, err := firstAnimatedFrame(data); err == nil {
				t.Errorf("%s: expected an error", name)
			}
		}()
	}
}

func TestStickerPNGRejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.webp")
	if err := os.WriteFile(path, make([]byte, 16), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, maxStickerBytes+1); err != nil {
		t.Skipf("cannot create a sparse file here: %v", err)
	}
	// The file must be refused on its reported size, without being read.
	if _, err := StickerPNG(path); err == nil {
		t.Fatal("an oversized sticker was accepted")
	}
}
