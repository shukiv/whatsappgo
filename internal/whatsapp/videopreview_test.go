package whatsapp

import (
	"bytes"
	"testing"

	"go.mau.fi/whatsmeow"

	"github.com/shukiv/whatsappgo/internal/gateway"
)

// A video sent without a still, a length and a size is a grey box on the
// phone. With them, the recipient - and this account's own phone - reads them
// back the way it reads any received video.
func TestVideoPreviewGoesOnTheVideoMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	thumbnail := jpegSized(t, 160, 90)
	payload := buildMediaPayload("video", "video/mp4", "clip.mp4", "", whatsmeow.UploadResponse{}, nil, false)
	applyVideoPreview(payload.VideoMessage, gateway.VideoPreview{Thumbnail: thumbnail, Seconds: 12, Width: 1280, Height: 720})

	if got, ext := thumbnailFromMessage(payload); !bytes.Equal(got, thumbnail) || ext != ".jpg" {
		t.Fatalf("the thumbnail cannot be read back from the message: %d bytes, %q", len(got), ext)
	}
	video := payload.GetVideoMessage()
	if video.GetSeconds() != 12 || video.GetWidth() != 1280 || video.GetHeight() != 720 {
		t.Fatalf("length or size missing: %d s, %dx%d", video.GetSeconds(), video.GetWidth(), video.GetHeight())
	}
}

// Nothing known is nothing sent: a zero would tell the phone the video is no
// seconds long or no pixels wide.
func TestEmptyVideoPreviewLeavesTheFieldsOut(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	payload := buildMediaPayload("video", "video/mp4", "clip.mp4", "", whatsmeow.UploadResponse{}, nil, false)
	applyVideoPreview(payload.VideoMessage, gateway.VideoPreview{})
	video := payload.GetVideoMessage()
	if video.JPEGThumbnail != nil || video.Seconds != nil || video.Width != nil || video.Height != nil {
		t.Fatalf("an empty preview set fields: %v", video)
	}

	payload = buildMediaPayload("video", "video/mp4", "clip.mp4", "", whatsmeow.UploadResponse{}, nil, false)
	applyVideoPreview(payload.VideoMessage, gateway.VideoPreview{Width: 1280})
	if video := payload.GetVideoMessage(); video.Width != nil || video.Height != nil {
		t.Fatalf("half a size was sent: %v", video)
	}
}
