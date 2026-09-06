package store

import (
	"context"
	"reflect"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
)

func TestAliasMergePreservesRichMessages(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(map[bool]string{false: "unique", true: "duplicate"}[duplicate], func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			const phone, lid = "15550000001@s.whatsapp.net", "10001@lid"
			want := model.Message{ID: "rich", ChatJID: phone, Timestamp: 10, Kind: "audio", Body: "caption", Status: "played",
				SenderJID: phone, SenderName: "Contact", ReplyTo: "quoted", MediaMIME: "audio/ogg", MediaName: "voice.ogg",
				MediaPath: "/cached/voice.ogg", MediaThumbnail: "/cached/thumb.jpg", MediaSize: 100, MediaDuration: 42,
				AudioWaveform: []int{1, 2, 3}, LinkURL: "https://example.com", LinkTitle: "Card", LinkDescription: "Description", LinkThumbnail: "/card.jpg",
				ContactName: "Shared contact", ContactPhone: "15550000002", ContactCount: 1, Latitude: 12.3, Longitude: 45.6,
				DeliveredAt: 20, ReadAt: 30, PlayedAt: 40, ForwardingScore: 2}
			if err := s.UpsertMessage(ctx, want, "Contact", false); err != nil {
				t.Fatal(err)
			}
			if err := s.SetMessageStarred(ctx, phone, want.ID, true); err != nil {
				t.Fatal(err)
			}
			if duplicate {
				if err := s.UpsertMessage(ctx, model.Message{ID: want.ID, ChatJID: lid, Timestamp: 10, Kind: "audio", Body: "caption", Status: "received"}, "", false); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.LinkChatAliases(ctx, lid, phone); err != nil {
				t.Fatal(err)
			}
			got, err := s.GetMessage(ctx, lid, want.ID)
			if err != nil {
				t.Fatal(err)
			}
			want.ChatJID, want.Starred = lid, true
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("merged metadata:\n got %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestAliasMergePreservesDeletionsAndEdits(t *testing.T) {
	for _, changedIdentity := range []string{"phone", "lid"} {
		for _, change := range []string{"delete", "edit"} {
			t.Run(changedIdentity+"-"+change, func(t *testing.T) {
				s := newTestStore(t)
				ctx := context.Background()
				const phone, lid = "15550000001@s.whatsapp.net", "10001@lid"
				for _, jid := range []string{phone, lid} {
					if err := s.UpsertMessage(ctx, model.Message{ID: "same", ChatJID: jid, Timestamp: 10, Kind: "text", Body: "Original", Status: "received", LinkURL: "https://old.example", LinkTitle: "Old card"}, "", false); err != nil {
						t.Fatal(err)
					}
				}
				changed := phone
				if changedIdentity == "lid" {
					changed = lid
				}
				if change == "delete" {
					if err := s.MarkRevoked(ctx, changed, "same"); err != nil {
						t.Fatal(err)
					}
				} else if err := s.EditMessage(ctx, changed, "same", "Edited"); err != nil {
					t.Fatal(err)
				}
				if err := s.LinkChatAliases(ctx, lid, phone); err != nil {
					t.Fatal(err)
				}
				got, err := s.GetMessage(ctx, lid, "same")
				if err != nil {
					t.Fatal(err)
				}
				if change == "delete" && (!got.Revoked || got.Body != "" || got.Kind != "revoked") {
					t.Fatalf("deletion lost: %#v", got)
				}
				if change == "edit" && (!got.Edited || got.Body != "Edited" || got.LinkURL != "") {
					t.Fatalf("edit/card revision lost: %#v", got)
				}
			})
		}
	}
}

func TestAliasMergeKeepsLinkCardWithItsBodyAndURL(t *testing.T) {
	for _, sparse := range []bool{true, false} {
		t.Run(map[bool]string{true: "empty-body", false: "different-card"}[sparse], func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			alias := model.Message{ID: "card", ChatJID: "phone", Kind: "text", Body: "two links", LinkURL: "https://a.test", LinkTitle: "A", LinkThumbnail: "/a.jpg"}
			canonical := model.Message{ID: "card", ChatJID: "lid", Kind: "text"}
			if !sparse {
				canonical.Body = alias.Body
				canonical.LinkURL = "https://b.test"
			}
			for _, m := range []model.Message{alias, canonical} {
				if err := s.UpsertMessage(ctx, m, "", false); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.LinkChatAliases(ctx, "lid", "phone"); err != nil {
				t.Fatal(err)
			}
			got, err := s.GetMessage(ctx, "lid", "card")
			if err != nil {
				t.Fatal(err)
			}
			want := canonical
			if sparse {
				want = alias
			}
			if got.Body != alias.Body || got.LinkURL != want.LinkURL || got.LinkTitle != want.LinkTitle || got.LinkThumbnail != want.LinkThumbnail {
				t.Fatalf("mixed/lost card: %#v", got)
			}
		})
	}
}
