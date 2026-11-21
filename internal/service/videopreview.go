package service

import (
	"bytes"
	"errors"
	"fmt"
	"image/jpeg"

	"github.com/shukiv/whatsappgo/internal/gateway"
)

const (
	// The thumbnails phones attach to videos are two to three kilobytes; the
	// whole message has to stay small, so a much larger one is a mistake.
	maxVideoThumbnailBytes = 64 << 10
	maxVideoThumbnailSide  = 1024
	maxVideoSeconds        = 24 * 60 * 60
	maxVideoSide           = 16384
)

// videoPreview checks what message.send_media says a video looks like before
// it is handed to WhatsApp, which would otherwise pass any of it to every
// recipient's phone.
func (p sendMediaParams) videoPreview() (gateway.VideoPreview, error) {
	preview := gateway.VideoPreview{Thumbnail: p.Thumbnail, Seconds: p.Seconds, Width: p.Width, Height: p.Height}
	if len(p.Thumbnail) == 0 && p.Seconds == 0 && p.Width == 0 && p.Height == 0 {
		return preview, nil
	}
	if p.Document || p.Voice || p.Sticker {
		return gateway.VideoPreview{}, errors.New("thumbnail, seconds, width and height describe a video; they cannot go with document, voice or sticker")
	}
	if len(p.Thumbnail) > 0 {
		if len(p.Thumbnail) > maxVideoThumbnailBytes {
			return gateway.VideoPreview{}, fmt.Errorf("thumbnail must be at most %d KiB", maxVideoThumbnailBytes>>10)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(p.Thumbnail))
		if err != nil {
			return gateway.VideoPreview{}, errors.New("thumbnail must be a JPEG image")
		}
		if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxVideoThumbnailSide || cfg.Height > maxVideoThumbnailSide {
			return gateway.VideoPreview{}, fmt.Errorf("thumbnail must fit within %d by %d pixels", maxVideoThumbnailSide, maxVideoThumbnailSide)
		}
	}
	if p.Seconds < 0 || p.Seconds > maxVideoSeconds {
		return gateway.VideoPreview{}, fmt.Errorf("seconds must be between 0 and %d", maxVideoSeconds)
	}
	if p.Width < 0 || p.Height < 0 || p.Width > maxVideoSide || p.Height > maxVideoSide {
		return gateway.VideoPreview{}, fmt.Errorf("width and height must be between 0 and %d", maxVideoSide)
	}
	if (p.Width == 0) != (p.Height == 0) {
		return gateway.VideoPreview{}, errors.New("width and height go together")
	}
	return preview, nil
}
