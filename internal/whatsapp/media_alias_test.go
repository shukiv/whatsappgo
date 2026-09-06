package whatsapp

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/shukiv/whatsappgo/internal/mediastore"
	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestDownloadMediaRestoresPreMergeAttachment(t *testing.T) {
	ctx := context.Background()
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	media, err := mediastore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = media.Close() })
	const phone, lid = "15550000001@s.whatsapp.net", "10001@lid"
	msg := model.Message{ID: "old-photo", ChatJID: phone, Timestamp: 1, Kind: "image", MediaMIME: "image/jpeg", Status: "received"}
	if err := st.UpsertMessage(ctx, msg, "Contact", false); err != nil {
		t.Fatal(err)
	}
	want := []byte("durable attachment bytes")
	if err := media.Put(ctx, phone, msg.ID, mediastore.Info{MIME: msg.MediaMIME}, bytes.NewReader(want)); err != nil {
		t.Fatal(err)
	}
	raw, err := proto.Marshal(&waE2E.Message{ImageMessage: &waE2E.ImageMessage{DirectPath: proto.String("/expired")}})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveMediaPayload(ctx, phone, msg.ID, raw); err != nil {
		t.Fatal(err)
	}
	if err := st.LinkChatAliases(ctx, lid, phone); err != nil {
		t.Fatal(err)
	}
	client := &Client{store: st, media: media, mediaDir: t.TempDir(),
		downloadToFile: func(context.Context, whatsmeow.DownloadableMessage, whatsmeow.File) error {
			return errors.New("unexpected network download")
		},
	}
	got, err := client.DownloadMedia(ctx, lid, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(got.MediaPath)
	if err != nil || !bytes.Equal(data, want) {
		t.Fatalf("restored %q: %v", data, err)
	}
	if _, found, err := media.Lookup(ctx, phone, msg.ID); err != nil || !found {
		t.Fatalf("original archive lost: %v", err)
	}
}
