package store

import (
	"context"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
)

func TestEditedLinkPreviewDoesNotOutliveItsBody(t *testing.T) {
	for _, incoming := range []bool{false, true} {
		name := "local"
		if incoming {
			name = "incoming"
		}
		t.Run(name, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			original := model.Message{ID: "m", ChatJID: "chat@lid", Timestamp: 100,
				Kind: "text", Body: "https://old.example/", LinkURL: "https://old.example/",
				LinkTitle: "Old", LinkDescription: "Old description", LinkThumbnail: "/fixture/old.jpg"}
			if err := s.UpsertMessage(ctx, original, "Chat", false); err != nil {
				t.Fatal(err)
			}
			if incoming {
				edited := original
				edited.Body, edited.Edited = "No link anymore", true
				edited.LinkURL, edited.LinkTitle, edited.LinkDescription, edited.LinkThumbnail = "", "", "", ""
				if err := s.UpsertMessage(ctx, edited, "Chat", false); err != nil {
					t.Fatal(err)
				}
			} else if err := s.EditMessage(ctx, original.ChatJID, original.ID, "No link anymore"); err != nil {
				t.Fatal(err)
			}
			check := func() {
				t.Helper()
				m, err := s.GetMessage(ctx, original.ChatJID, original.ID)
				if err != nil {
					t.Fatal(err)
				}
				if m.Body != "No link anymore" || !m.Edited || m.LinkURL != "" || m.LinkTitle != "" || m.LinkDescription != "" || m.LinkThumbnail != "" {
					t.Fatalf("edited message retained/restored an obsolete card: %#v", m)
				}
			}
			check()
			// History can redeliver the original after the edit has landed.
			if err := s.UpsertMessage(ctx, original, "Chat", false); err != nil {
				t.Fatal(err)
			}
			check()
			changed, err := s.UpdateLinkPreviewForBody(ctx, original.ChatJID, original.ID, original.Body,
				original.LinkURL, original.LinkTitle, original.LinkDescription, "/fixture/upgraded.jpg")
			if err != nil || changed {
				t.Fatalf("a delayed preview updated an edited message: changed=%v err=%v", changed, err)
			}
			check()
		})
	}
}
