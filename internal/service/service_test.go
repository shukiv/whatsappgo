package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shuki/whatsappgo/internal/events"
	"github.com/shuki/whatsappgo/internal/gateway"
	"github.com/shuki/whatsappgo/internal/model"
	"github.com/shuki/whatsappgo/internal/store"
)

type fakeGateway struct {
	gateway.Unavailable
	sentText        string
	historyChat     string
	downloadMessage string
}

func (f *fakeGateway) Subscribe(func(gateway.Event)) func() { return func() {} }
func (f *fakeGateway) SendText(_ context.Context, chat, text, reply string) (model.Message, error) {
	f.sentText = text
	return model.Message{ID: "out-1", ChatJID: chat, SenderJID: "me@s.whatsapp.net", Timestamp: 10, Kind: "text", Body: text, FromMe: true, Status: "sent", ReplyTo: reply}, nil
}

func (f *fakeGateway) RequestHistory(_ context.Context, chat string, _ int) error {
	f.historyChat = chat
	return nil
}

func (f *fakeGateway) DownloadMedia(_ context.Context, chat, messageID string) (model.Message, error) {
	f.downloadMessage = messageID
	return model.Message{ID: messageID, ChatJID: chat, Kind: "document", MediaPath: "/tmp/report.pdf"}, nil
}

func TestSendMessagePersistsOutgoingResult(t *testing.T) {
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gw := &fakeGateway{}
	svc := New(st, gw, events.New())
	defer svc.Close()
	result, err := svc.Handle(context.Background(), "message.send", json.RawMessage(`{"chat_jid":"alice@s.whatsapp.net","text":"hello","reply_to":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if gw.sentText != "hello" {
		t.Fatalf("gateway received %q", gw.sentText)
	}
	if result.(model.Message).ID != "out-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
	page, err := st.ListMessages(context.Background(), "alice@s.whatsapp.net", 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].Body != "hello" {
		t.Fatalf("message not persisted: %#v", page)
	}
}

func TestRequestRejectsUnknownFields(t *testing.T) {
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := New(st, &fakeGateway{}, events.New())
	defer svc.Close()
	_, err = svc.Handle(context.Background(), "chats.list", json.RawMessage(`{"limit":10,"surprise":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestHistoryAndMediaRequestsReachGateway(t *testing.T) {
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gw := &fakeGateway{}
	svc := New(st, gw, events.New())
	defer svc.Close()
	if _, err := svc.Handle(context.Background(), "history.request", json.RawMessage(`{"chat_jid":"alice@s.whatsapp.net","limit":50}`)); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Handle(context.Background(), "message.download", json.RawMessage(`{"chat_jid":"alice@s.whatsapp.net","message_id":"doc-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if gw.historyChat != "alice@s.whatsapp.net" || gw.downloadMessage != "doc-1" {
		t.Fatalf("gateway calls missing: history=%q download=%q", gw.historyChat, gw.downloadMessage)
	}
	if result.(model.Message).MediaPath != "/tmp/report.pdf" {
		t.Fatalf("unexpected download result: %#v", result)
	}
}
