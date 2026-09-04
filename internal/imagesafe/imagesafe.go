// Package imagesafe decodes pictures that arrived from somewhere else.
//
// Every picture this application decodes was chosen by another party: the
// thumbnail inside a message, a sticker, the preview image of a linked page.
// Two things about Go's image decoders make that dangerous without care.
//
// The first is size. A decoder allocates width × height × bytes-per-pixel from
// the numbers in the file header, before reading any pixel data, so a
// few-kilobyte file can declare a canvas of tens of thousands of pixels a side
// and ask for gigabytes. Running out of memory is fatal in Go: it is not a
// panic that can be recovered, it stops the process. Checking the header first
// costs nothing and removes that entirely.
//
// The second is robustness. Image decoders parse hostile binary formats and
// have historically panicked on malformed input. A panic in one of the
// daemon's goroutines takes down the whole daemon, losing every account, so a
// decode that fails must fail as an error about one picture.
package imagesafe

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// Sensible ceilings for the pictures this application handles. A thumbnail is
// a preview a few hundred pixels a side, and no real photograph shared in a
// chat approaches thirty megapixels.
const (
	MaxThumbnailPixels = 4 << 20
	MaxPicturePixels   = 30 << 20
)

// ErrTooLarge reports a picture whose declared canvas is larger than the
// caller is prepared to decode.
var ErrTooLarge = errors.New("image dimensions are implausibly large")

// EnsureDecodable reads only the header and reports whether decoding the
// picture would stay within maxPixels.
func EnsureDecodable(data []byte, maxPixels int) error {
	if len(data) == 0 {
		return errors.New("image is empty")
	}
	config, _, err := decodeConfig(data)
	if err != nil {
		return err
	}
	if config.Width <= 0 || config.Height <= 0 {
		return errors.New("image has no size")
	}
	// Multiplied as int64 so the product cannot wrap on a 32-bit build.
	if int64(config.Width)*int64(config.Height) > int64(maxPixels) {
		return fmt.Errorf("%w: %dx%d", ErrTooLarge, config.Width, config.Height)
	}
	return nil
}

func decodeConfig(data []byte) (config image.Config, format string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("image header is malformed: %v", recovered)
		}
	}()
	return image.DecodeConfig(bytes.NewReader(data))
}

// Decode checks the header, then decodes the picture. A decoder that panics on
// malformed input yields an error rather than stopping the process.
func Decode(data []byte, maxPixels int) (image.Image, error) {
	if err := EnsureDecodable(data, maxPixels); err != nil {
		return nil, err
	}
	return Recovered(func() (image.Image, error) {
		decoded, _, err := image.Decode(bytes.NewReader(data))
		return decoded, err
	})
}

// Recovered runs a decoder and turns a panic inside it into an error. Use it
// when a specific decoder has to be called directly, after EnsureDecodable.
func Recovered(decode func() (image.Image, error)) (decoded image.Image, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			decoded = nil
			err = fmt.Errorf("image could not be decoded: %v", recovered)
		}
	}()
	return decode()
}
