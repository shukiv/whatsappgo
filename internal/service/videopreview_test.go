package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/store"
)

func encodedImage(t *testing.T, width, height int, asPNG bool) string {
	t.Helper()
	return encodedFrame(t, image.NewRGBA(image.Rect(0, 0, width, height)), asPNG)
}

// A JPEG within the pixel limit but past the byte limit: noise does not
// compress.
func oversizedJPEG(t *testing.T) string {
	t.Helper()
	frame := image.NewRGBA(image.Rect(0, 0, maxVideoThumbnailSide, maxVideoThumbnailSide))
	state := uint32(1)
	for i := range frame.Pix {
		state = state*1664525 + 1013904223
		frame.Pix[i] = byte(state >> 24)
	}
	encoded := encodedFrame(t, frame, false)
	if decoded, _ := base64.StdEncoding.DecodeString(encoded); len(decoded) <= maxVideoThumbnailBytes {
		t.Fatalf("the oversized fixture is only %d bytes", len(decoded))
	}
	return encoded
}

func encodedFrame(t *testing.T, frame *image.RGBA, asPNG bool) string {
	t.Helper()
	var buffer bytes.Buffer
	var err error
	if asPNG {
		err = png.Encode(&buffer, frame)
	} else {
		err = jpeg.Encode(&buffer, frame, &jpeg.Options{Quality: 90})
	}
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func previewService(t *testing.T) (*Service, *attachmentGateway) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	gw := &attachmentGateway{}
	svc := New(st, gw, events.New())
	t.Cleanup(svc.Close)
	return svc, gw
}

// A phone shows a video's still and length before downloading it, from what
// the message carries; the window sends them with the video.
func TestSendMediaCarriesTheVideoPreview(t *testing.T) {
	svc, gw := previewService(t)
	thumbnail := encodedImage(t, 160, 90, false)
	params := fmt.Sprintf(`{"chat_jid":"123@lid","path":"/fixture/clip.mp4","thumbnail":%q,"seconds":12,"width":1280,"height":720}`, thumbnail)
	if _, err := svc.Handle(context.Background(), "message.send_media", json.RawMessage(params)); err != nil {
		t.Fatal(err)
	}
	got := gw.request.Video
	if base64.StdEncoding.EncodeToString(got.Thumbnail) != thumbnail || got.Seconds != 12 || got.Width != 1280 || got.Height != 720 {
		t.Fatalf("the preview did not reach WhatsApp intact: %+v", got)
	}

	gif := fmt.Sprintf(`{"chat_jid":"123@lid","path":"/fixture/loop.mp4","gif":true,"thumbnail":%q,"seconds":3}`, thumbnail)
	if _, err := svc.Handle(context.Background(), "message.send_media", json.RawMessage(gif)); err != nil {
		t.Fatalf("a GIF is a video and may carry a preview: %v", err)
	}
	if !gw.request.GIF || gw.request.Video.Seconds != 3 {
		t.Fatalf("the GIF's preview was lost: %+v", gw.request)
	}

	if _, err := svc.Handle(context.Background(), "message.send_media", json.RawMessage(`{"chat_jid":"123@lid","path":"/fixture/clip.mp4"}`)); err != nil {
		t.Fatal(err)
	}
	if video := gw.request.Video; len(video.Thumbnail) != 0 || video.Seconds != 0 || video.Width != 0 || video.Height != 0 {
		t.Fatalf("a send without a preview invented one: %+v", video)
	}
}

// Whatever the preview says goes to every recipient's phone, so what is not a
// small JPEG and plausible numbers never leaves the service.
func TestSendMediaRefusesABadVideoPreview(t *testing.T) {
	svc, gw := previewService(t)
	jpegThumb := encodedImage(t, 160, 90, false)
	oversized := oversizedJPEG(t)
	for name, extra := range map[string]string{
		"png thumbnail":        fmt.Sprintf(`"thumbnail":%q`, encodedImage(t, 160, 90, true)),
		"not an image":         fmt.Sprintf(`"thumbnail":%q`, base64.StdEncoding.EncodeToString([]byte("not a picture"))),
		"not base64":           `"thumbnail":"%%%"`,
		"too many bytes":       fmt.Sprintf(`"thumbnail":%q`, oversized),
		"too many pixels":      fmt.Sprintf(`"thumbnail":%q`, encodedImage(t, maxVideoThumbnailSide+1, 10, false)),
		"negative seconds":     `"seconds":-1`,
		"more than a day":      fmt.Sprintf(`"seconds":%d`, maxVideoSeconds+1),
		"width without height": `"width":1280`,
		"height without width": `"height":720`,
		"negative width":       `"width":-1,"height":720`,
		"too wide":             fmt.Sprintf(`"width":%d,"height":720`, maxVideoSide+1),
		"with document":        fmt.Sprintf(`"document":true,"thumbnail":%q`, jpegThumb),
		"with voice":           `"voice":true,"seconds":4`,
		"with sticker":         `"sticker":true,"width":512,"height":512`,
	} {
		gw.request = gateway.MediaRequest{}
		params := `{"chat_jid":"123@lid","path":"/fixture/clip.mp4",` + extra + `}`
		if _, err := svc.Handle(context.Background(), "message.send_media", json.RawMessage(params)); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if gw.request.Path != "" {
			t.Errorf("%s: reached WhatsApp: %+v", name, gw.request)
		}
	}
}

// The window sends these fields only to a service that lists them, because an
// older one refuses a field it does not know and the video with it.
func TestDiscoveryListsTheVideoPreviewFields(t *testing.T) {
	for _, described := range apiMethods {
		if described.Name != "message.send_media" {
			continue
		}
		for _, field := range []string{"thumbnail", "seconds", "width", "height"} {
			if _, ok := described.Params[field]; !ok {
				t.Errorf("rpc.discover does not list %s for message.send_media", field)
			}
		}
		if !strings.Contains(described.Summary, "thumbnail") {
			t.Errorf("summary does not mention the thumbnail: %q", described.Summary)
		}
		return
	}
	t.Fatal("message.send_media is not described")
}
