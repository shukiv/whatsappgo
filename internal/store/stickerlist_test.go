package store

import (
	"context"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
)

func stickerAt(id, chat, sha string, at int64) model.Message {
	return model.Message{
		ID: id, ChatJID: chat, Kind: "sticker", Timestamp: at,
		MediaPath: "/tmp/" + id + ".webp", MediaSize: 1024,
	}
}

// The picker lists stickers, not messages. One sticker sent in three
// conversations is one sticker, and the row kept is the most recent, because
// that is the one the reader last saw.
func TestTheSameStickerIsListedOnce(t *testing.T) {
	ctx := context.Background()
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	same := "aaaa"
	for _, m := range []model.Message{
		stickerAt("s1", "1@s.whatsapp.net", same, 100),
		stickerAt("s2", "2@s.whatsapp.net", same, 300),
		stickerAt("s3", "3@s.whatsapp.net", same, 200),
		stickerAt("other", "1@s.whatsapp.net", "bbbb", 400),
	} {
		if err := st.UpsertMessage(ctx, m, "", false); err != nil {
			t.Fatal(err)
		}
	}
	for id, sha := range map[string]string{"s1": same, "s2": same, "s3": same, "other": "bbbb"} {
		chat := "1@s.whatsapp.net"
		switch id {
		case "s2":
			chat = "2@s.whatsapp.net"
		case "s3":
			chat = "3@s.whatsapp.net"
		}
		if err := st.SetMediaSha(ctx, chat, id, sha); err != nil {
			t.Fatal(err)
		}
	}

	page, err := st.ListAllSharedMessages(ctx, "stickers", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 {
		t.Fatalf("want one row per distinct sticker, got %d: %#v", len(page.Messages), page.Messages)
	}
	seen := map[string]bool{}
	for _, m := range page.Messages {
		seen[m.ID] = true
	}
	// 300 is the newest of the three copies.
	if !seen["s2"] || !seen["other"] || seen["s1"] || seen["s3"] {
		t.Fatalf("the newest copy was not the one kept: %#v", page.Messages)
	}
}

// A sticker whose file has not been identified is never merged: guessing that
// two unknowns are the same would hide a sticker that is not a duplicate.
func TestUnidentifiedStickersAreNeverMerged(t *testing.T) {
	ctx := context.Background()
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	for _, m := range []model.Message{
		stickerAt("u1", "1@s.whatsapp.net", "", 100),
		stickerAt("u2", "1@s.whatsapp.net", "", 200),
	} {
		if err := st.UpsertMessage(ctx, m, "", false); err != nil {
			t.Fatal(err)
		}
	}
	page, err := st.ListAllSharedMessages(ctx, "stickers", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 {
		t.Fatalf("unidentified stickers were merged: %#v", page.Messages)
	}
}
