package whatsapp

import (
	"context"
	"strings"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

func TestForwardUndownloadedMediaDoesNotSendCaptionAsText(t *testing.T) {
	for _, kind := range []string{"image", "video", "audio", "document", "sticker"} {
		for _, caption := range []string{"caption must stay with attachment", ""} {
			t.Run(kind+"/"+caption, func(t *testing.T) {
				st, err := localstore.OpenMemory()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = st.Close() })
				ctx := context.Background()
				msg := model.Message{ID: "attachment", ChatJID: "123@lid", Kind: kind, Body: caption, Timestamp: 1}
				if err := st.UpsertMessage(ctx, msg, "Alice", false); err != nil {
					t.Fatal(err)
				}
				// No WhatsApp connection: this path must refuse before trying to
				// send anything, especially the caption as a successful text forward.
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("undownloaded attachment reached the text-send path: %v", r)
					}
				}()
				c := &Client{store: st}
				_, err = c.ForwardMessage(ctx, msg.ChatJID, msg.ID, "456@lid")
				if err == nil || !strings.Contains(err.Error(), "download") {
					t.Fatalf("want a download-required error, got %v", err)
				}
			})
		}
	}
}
