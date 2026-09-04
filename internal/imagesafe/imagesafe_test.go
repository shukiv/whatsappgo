package imagesafe

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func realPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, width, height))
	source.Set(0, 0, color.RGBA{R: 1, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, source); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// hugeCanvasPNG is a small, structurally valid file whose header claims an
// enormous picture. A decoder that trusts the header allocates
// width*height*4 bytes for it before reading a single pixel.
func hugeCanvasPNG(t *testing.T, width, height uint32) []byte {
	t.Helper()
	data := realPNG(t, 2, 2)
	// After the 8-byte signature comes the IHDR chunk: a 4-byte length, the
	// 4-byte type, the payload, then a CRC over type and payload. The CRC has
	// to be recomputed or the decoder rejects the file before reading a size.
	const ihdrType = 12
	const ihdrPayload = 16
	binary.BigEndian.PutUint32(data[ihdrPayload:ihdrPayload+4], width)
	binary.BigEndian.PutUint32(data[ihdrPayload+4:ihdrPayload+8], height)
	length := int(binary.BigEndian.Uint32(data[8:12]))
	sum := crc32.ChecksumIEEE(data[ihdrType : ihdrType+4+length])
	binary.BigEndian.PutUint32(data[ihdrType+4+length:ihdrType+8+length], sum)
	return data
}

func TestEnsureDecodableRejectsAnImplausibleCanvas(t *testing.T) {
	err := EnsureDecodable(hugeCanvasPNG(t, 60000, 60000), MaxThumbnailPixels)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("a 60000x60000 header was accepted: %v", err)
	}
	// The same picture is fine once the caller allows that many pixels.
	if err := EnsureDecodable(realPNG(t, 64, 48), MaxThumbnailPixels); err != nil {
		t.Fatalf("an ordinary thumbnail was rejected: %v", err)
	}
}

func TestEnsureDecodableRejectsUnusableInput(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":     nil,
		"not image": []byte("this is not a picture"),
		"truncated": realPNG(t, 8, 8)[:12],
	} {
		if err := EnsureDecodable(data, MaxThumbnailPixels); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestDecodeReturnsThePicture(t *testing.T) {
	decoded, err := Decode(realPNG(t, 12, 9), MaxThumbnailPixels)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 12 || decoded.Bounds().Dy() != 9 {
		t.Fatalf("unexpected bounds: %v", decoded.Bounds())
	}
	if _, err := Decode(hugeCanvasPNG(t, 60000, 60000), MaxThumbnailPixels); err == nil {
		t.Fatal("an implausible canvas was decoded")
	}
}

func TestRecoveredTurnsAPanicIntoAnError(t *testing.T) {
	// A decoder that panics on malformed input must fail one picture rather
	// than stop the daemon.
	decoded, err := Recovered(func() (image.Image, error) {
		panic("index out of range")
	})
	if err == nil {
		t.Fatal("a panicking decoder was not reported as an error")
	}
	if decoded != nil {
		t.Fatal("a failed decode returned an image")
	}
	if _, err := Recovered(func() (image.Image, error) { return nil, errors.New("bad") }); err == nil {
		t.Fatal("an ordinary decoder error was swallowed")
	}
}
