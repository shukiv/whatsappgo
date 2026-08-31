package whatsapp

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, 8, 6))
	source.Set(0, 0, color.RGBA{R: 37, G: 211, B: 102, A: 255})
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, source, nil); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestThumbnailFromMessageReadsEveryMediaKind(t *testing.T) {
	preview := []byte{1, 2, 3}
	cases := []struct {
		name    string
		message *waE2E.Message
		want    []byte
		ext     string
	}{
		{"image", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{JPEGThumbnail: preview}}, preview, ".jpg"},
		{"video", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{JPEGThumbnail: preview}}, preview, ".jpg"},
		{"sticker", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{PngThumbnail: preview}}, preview, ".png"},
		{"document", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{JPEGThumbnail: preview}}, preview, ".jpg"},
		{"text", &waE2E.Message{Conversation: proto.String("hello")}, nil, ""},
		{"nil", nil, nil, ""},
	}
	for _, tc := range cases {
		data, ext := thumbnailFromMessage(tc.message)
		if !bytes.Equal(data, tc.want) || ext != tc.ext {
			t.Errorf("%s: got (%v, %q), want (%v, %q)", tc.name, data, ext, tc.want, tc.ext)
		}
	}
}

func TestCacheThumbnailWritesPrivateFile(t *testing.T) {
	c := &Client{mediaDir: t.TempDir()}
	msg := model.Message{ChatJID: "alice@s.whatsapp.net", ID: "photo-1", Kind: "image"}
	path := c.cacheThumbnail(msg, &waE2E.Message{ImageMessage: &waE2E.ImageMessage{JPEGThumbnail: jpegBytes(t)}})
	if path == "" {
		t.Fatal("inline preview was not cached")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("thumbnail permissions are %v, want 0600", info.Mode().Perm())
	}
	if filepath.Ext(path) != ".jpg" {
		t.Fatalf("unexpected extension: %q", path)
	}
	if again := c.cacheThumbnail(msg, &waE2E.Message{ImageMessage: &waE2E.ImageMessage{JPEGThumbnail: jpegBytes(t)}}); again != path {
		t.Fatalf("second call returned %q, want the cached %q", again, path)
	}
	sticker := c.cacheThumbnail(model.Message{ChatJID: "alice@s.whatsapp.net", ID: "sticker-1"},
		&waE2E.Message{StickerMessage: &waE2E.StickerMessage{PngThumbnail: pngBytes(t)}})
	if filepath.Ext(sticker) != ".png" {
		t.Fatalf("sticker preview stored as %q, want a .png file", sticker)
	}
}

func TestCacheThumbnailRejectsUnusablePayloads(t *testing.T) {
	c := &Client{mediaDir: t.TempDir()}
	msg := model.Message{ChatJID: "alice@s.whatsapp.net", ID: "photo-2"}
	if path := c.cacheThumbnail(msg, &waE2E.Message{ImageMessage: &waE2E.ImageMessage{JPEGThumbnail: []byte("not an image")}}); path != "" {
		t.Fatalf("undecodable preview was cached at %q", path)
	}
	if path := c.cacheThumbnail(msg, &waE2E.Message{ImageMessage: &waE2E.ImageMessage{JPEGThumbnail: make([]byte, maxInlineThumbnail+1)}}); path != "" {
		t.Fatalf("oversized preview was cached at %q", path)
	}
	if path := c.cacheThumbnail(msg, &waE2E.Message{Conversation: proto.String("hi")}); path != "" {
		t.Fatalf("text message produced a preview at %q", path)
	}
}

func TestWithCachedThumbnailAttachesPreviewOnce(t *testing.T) {
	c := &Client{mediaDir: t.TempDir()}
	msg := model.Message{ChatJID: "alice@s.whatsapp.net", ID: "photo-3", Kind: "image"}
	raw := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{JPEGThumbnail: jpegBytes(t)}}
	withPreview := c.withCachedThumbnail(msg, raw)
	if withPreview.MediaThumbnail == "" {
		t.Fatal("preview path was not attached to the message")
	}
	existing := model.Message{ChatJID: msg.ChatJID, ID: msg.ID, MediaThumbnail: "/already/set.jpg"}
	if got := c.withCachedThumbnail(existing, raw); got.MediaThumbnail != "/already/set.jpg" {
		t.Fatalf("existing preview was replaced by %q", got.MediaThumbnail)
	}
}

func TestBackfillThumbnailsExtractsPreviewsFromStoredPayloads(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	c := &Client{store: st, mediaDir: t.TempDir()}

	payloads := map[string]*waE2E.Message{
		"photo-1": {ImageMessage: &waE2E.ImageMessage{JPEGThumbnail: jpegBytes(t)}},
		"video-1": {VideoMessage: &waE2E.VideoMessage{JPEGThumbnail: jpegBytes(t)}},
		"photo-2": {ImageMessage: &waE2E.ImageMessage{}},
	}
	kinds := map[string]string{"photo-1": "image", "video-1": "video", "photo-2": "image"}
	for id, raw := range payloads {
		if err := st.UpsertMessage(ctx, model.Message{ID: id, ChatJID: "alice@s.whatsapp.net", Timestamp: 1, Kind: kinds[id], Status: "received"}, "Alice", false); err != nil {
			t.Fatal(err)
		}
		encoded, err := proto.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SaveMediaPayload(ctx, "alice@s.whatsapp.net", id, encoded); err != nil {
			t.Fatal(err)
		}
	}

	c.backfillThumbnails()

	page, err := st.ListMessages(ctx, "alice@s.whatsapp.net", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	previews := map[string]string{}
	for _, message := range page.Messages {
		previews[message.ID] = message.MediaThumbnail
	}
	if previews["photo-1"] == "" || previews["video-1"] == "" {
		t.Fatalf("stored media did not receive previews: %#v", previews)
	}
	// A message whose payload carries no preview must be left alone rather
	// than marked with a path that cannot be displayed.
	if previews["photo-2"] != "" {
		t.Fatalf("message without an inline preview got %q", previews["photo-2"])
	}
	if _, done, err := st.Metadata(ctx, thumbnailBackfillMetadataKey); err != nil || !done {
		t.Fatalf("backfill was not recorded as complete: done=%v err=%v", done, err)
	}
	// The scan must not repeat once it has been recorded.
	if err := st.UpdateMediaThumbnail(ctx, "alice@s.whatsapp.net", "photo-1", ""); err != nil {
		t.Fatal(err)
	}
	c.backfillThumbnails()
	message, err := st.GetMessage(ctx, "alice@s.whatsapp.net", "photo-1")
	if err != nil {
		t.Fatal(err)
	}
	if message.MediaThumbnail != "" {
		t.Fatal("completed backfill ran a second time")
	}
}

func TestBackfillLinkPreviewsRepairsHistoricalYouTubeCard(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	const chat = "george@lid"
	const messageID = "youtube-old"
	if err := st.UpsertMessage(ctx, model.Message{
		ID: messageID, ChatJID: chat, Timestamp: 1, Kind: "text", Status: "received",
		Body: "https://youtu.be/ub1O8H02j4E?si=test", LinkURL: "https://youtu.be/ub1O8H02j4E?si=test",
		LinkTitle: "- YouTube", LinkDescription: "Generic fallback",
	}, "George", false); err != nil {
		t.Fatal(err)
	}
	c := &Client{
		store: st, mediaDir: t.TempDir(),
		resolveLinkPreview: func(context.Context, string) (model.LinkPreview, error) {
			return model.LinkPreview{URL: "https://youtu.be/ub1O8H02j4E?si=test", Title: "Real video", Description: "Channel", Thumbnail: jpegBytes(t), ThumbnailMIME: "image/jpeg"}, nil
		},
	}
	c.backfillLinkPreviews()
	message, err := st.GetMessage(ctx, chat, messageID)
	if err != nil {
		t.Fatal(err)
	}
	if message.LinkTitle != "Real video" || message.LinkDescription != "Channel" || message.LinkThumbnail == "" {
		t.Fatalf("historical YouTube preview was not repaired: %#v", message)
	}
	if _, err := os.Stat(message.LinkThumbnail); err != nil {
		t.Fatalf("cached thumbnail is unavailable: %v", err)
	}
	if _, done, err := st.Metadata(ctx, linkPreviewBackfillMetadataKey); err != nil || !done {
		t.Fatalf("link preview backfill was not recorded: done=%v err=%v", done, err)
	}
}
