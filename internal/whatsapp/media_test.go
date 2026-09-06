package whatsapp

import (
	"context"
	"errors"
	"os"
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

func TestDocumentChoiceControlsUploadAndWireMessageType(t *testing.T) {
	for _, mimeType := range []string{"image/jpeg", "video/mp4", "audio/ogg", "application/pdf"} {
		kind, uploadType := classifyMedia(mimeType, true)
		if kind != "document" || uploadType != whatsmeow.MediaDocument {
			t.Fatalf("%s was not uploaded as a document: %s %s", mimeType, kind, uploadType)
		}
		payload := buildMediaPayload(kind, mimeType, "original.file", "caption", whatsmeow.UploadResponse{}, nil, false)
		if payload.GetDocumentMessage() == nil || payload.GetDocumentMessage().GetMimetype() != mimeType || payload.GetDocumentMessage().GetFileName() != "original.file" {
			t.Fatalf("%s lost document semantics: %v", mimeType, payload)
		}
	}
	for _, tc := range []struct{ mime, kind string }{
		{"image/jpeg", "image"}, {"video/mp4", "video"}, {"audio/ogg", "audio"}, {"application/pdf", "document"},
	} {
		kind, _ := classifyMedia(tc.mime, false)
		if kind != tc.kind {
			t.Fatalf("normal %s changed type to %s", tc.mime, kind)
		}
	}
}

func TestMediaCursorRoundTrip(t *testing.T) {
	cursor := localstore.MessageCursor{Timestamp: 1788103290123, MessageID: "3EB0F1388807E98A42"}
	if got := parseMediaCursor(formatMediaCursor(cursor)); got != cursor {
		t.Fatalf("cursor did not survive a round trip: %#v", got)
	}
	// A missing or damaged marker must restart the scan rather than skip work.
	for _, broken := range []string{"", "nonsense", "notanumber|id", "123"} {
		if got := parseMediaCursor(broken); got != (localstore.MessageCursor{}) {
			t.Fatalf("%q produced %#v, want an empty cursor", broken, got)
		}
	}
	// An id containing the separator keeps everything after the first one.
	odd := localstore.MessageCursor{Timestamp: 5, MessageID: "a|b"}
	if got := parseMediaCursor(formatMediaCursor(odd)); got != odd {
		t.Fatalf("separator in the id broke the cursor: %#v", got)
	}
}

func TestExpiredMediaPathIsRefreshedAndPersisted(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	msg := model.Message{ID: "old-photo", ChatJID: "marta@lid", SenderJID: "marta@lid", Timestamp: 1, Kind: "image", MediaMIME: "image/jpeg", Status: "received"}
	if err := st.UpsertMessage(ctx, msg, "Marta", false); err != nil {
		t.Fatal(err)
	}
	raw := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		DirectPath: proto.String("/expired"), MediaKey: []byte("media-key"),
		FileSHA256: make([]byte, 32), FileEncSHA256: make([]byte, 32),
	}}
	encoded, err := proto.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveMediaPayload(ctx, msg.ChatJID, msg.ID, encoded); err != nil {
		t.Fatal(err)
	}
	requestedRetry := false
	c := &Client{
		store: st, mediaDir: t.TempDir(),
		downloadToFile: func(context.Context, whatsmeow.DownloadableMessage, whatsmeow.File) error {
			return whatsmeow.ErrMediaDownloadFailedWith403
		},
		requestMediaRetryPath: func(_ context.Context, got model.Message, media whatsmeow.DownloadableMessage) (string, error) {
			requestedRetry = true
			if got.ID != msg.ID || media.GetDirectPath() != "/expired" {
				t.Fatalf("wrong retry request: %#v %q", got, media.GetDirectPath())
			}
			return "/refreshed", nil
		},
		downloadWithPathToFile: func(_ context.Context, path string, _ whatsmeow.DownloadableMessage, file whatsmeow.File) error {
			if path != "/refreshed" {
				t.Fatalf("download used %q", path)
			}
			_, err := file.Write([]byte("restored image"))
			return err
		},
	}
	result, err := c.downloadMedia(ctx, msg, raw.GetImageMessage(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if !requestedRetry || result.MediaPath == "" {
		t.Fatalf("expired media was not recovered: requested=%v result=%#v", requestedRetry, result)
	}
	if data, err := os.ReadFile(result.MediaPath); err != nil || string(data) != "restored image" {
		t.Fatalf("wrong recovered file: %q err=%v", data, err)
	}
	payload, available, err := st.MediaPayload(ctx, msg.ChatJID, msg.ID)
	if err != nil || !available {
		t.Fatalf("updated payload missing: available=%v err=%v", available, err)
	}
	var stored waE2E.Message
	if err := proto.Unmarshal(payload, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.GetImageMessage().GetDirectPath() != "/refreshed" {
		t.Fatalf("refreshed path was not persisted: %q", stored.GetImageMessage().GetDirectPath())
	}
	if !isExpiredMediaDownload(whatsmeow.ErrMediaDownloadFailedWith403) || !isExpiredMediaDownload(whatsmeow.ErrMediaDownloadFailedWith404) || !isExpiredMediaDownload(whatsmeow.ErrMediaDownloadFailedWith410) || isExpiredMediaDownload(errors.New("other")) {
		t.Fatal("expired media errors were not classified correctly")
	}
}
