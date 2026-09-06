package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/store"
)

type attachmentGateway struct {
	gateway.Unavailable
	request gateway.MediaRequest
}

func (g *attachmentGateway) SendMedia(_ context.Context, req gateway.MediaRequest) (model.Message, error) {
	g.request = req
	return model.Message{ID: "sent", ChatJID: req.ChatJID, Kind: "document", Body: req.Caption, Timestamp: 1}, nil
}

func TestSendMediaPreservesDocumentChoice(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gw := &attachmentGateway{}
	svc := New(st, gw, events.New())
	defer svc.Close()
	_, err = svc.Handle(context.Background(), "message.send_media", json.RawMessage(`{"chat_jid":"123@lid","path":"/fixture/photo.jpg","caption":"original file","reply_to":"quoted","document":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !gw.request.Document || gw.request.ReplyTo != "quoted" || gw.request.Caption != "original file" {
		t.Fatalf("document choice or reply was lost: %#v", gw.request)
	}
	_, err = svc.Handle(context.Background(), "message.send_media", json.RawMessage(`{"chat_jid":"123@lid","path":"/fixture/photo.jpg"}`))
	if err != nil || gw.request.Document {
		t.Fatalf("default media send was forced to a document: request=%#v err=%v", gw.request, err)
	}
	_, err = svc.Handle(context.Background(), "message.send_media", json.RawMessage(`{"chat_jid":"123@lid","path":"/fixture/voice.ogg","document":true,"voice":true}`))
	if err == nil {
		t.Fatal("conflicting document and voice options were accepted")
	}
}

func TestStarredMessagesCanBeScopedBeforeLimiting(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	for _, msg := range []model.Message{
		{ID: "older-alice", ChatJID: "alice@lid", Kind: "text", Timestamp: 1},
		{ID: "newer-bob", ChatJID: "bob@lid", Kind: "text", Timestamp: 2},
	} {
		if err := st.UpsertMessage(ctx, msg, "", false); err != nil {
			t.Fatal(err)
		}
		if err := st.SetMessageStarred(ctx, msg.ChatJID, msg.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.LinkChatAliases(ctx, "alice@lid", "123@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	svc := New(st, &fakeGateway{}, events.New())
	defer svc.Close()
	for _, tc := range []struct{ query, want string }{
		{`{"chat_jid":"alice@lid","limit":1}`, "older-alice"},
		{`{"chat_jid":"123@s.whatsapp.net","limit":1}`, "older-alice"},
		{`{"limit":1}`, "newer-bob"},
		{`{"chat_jid":"nobody@lid","limit":1}`, ""},
	} {
		result, err := svc.Handle(ctx, "messages.starred", json.RawMessage(tc.query))
		if err != nil {
			t.Fatal(err)
		}
		items := result.(map[string]any)["items"].([]model.Message)
		if tc.want == "" {
			if len(items) != 0 {
				t.Fatalf("empty conversation got other chats' stars: %#v", items)
			}
		} else if len(items) != 1 || items[0].ID != tc.want {
			t.Fatalf("query %s: want %s, got %#v", tc.query, tc.want, items)
		}
	}
}
