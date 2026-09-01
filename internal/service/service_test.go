package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/store"
)

func TestStatusesListGroupsActiveUpdatesBySender(t *testing.T) {
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := New(st, &fakeGateway{}, events.New())
	defer svc.Close()
	ctx := context.Background()
	now := time.Now().UnixMilli()

	if err := st.UpsertChat(ctx, model.Chat{JID: "alice@lid", Title: "Alice Address Book", AvatarPath: "/tmp/alice.jpg"}); err != nil {
		t.Fatal(err)
	}
	updates := []model.Message{
		{ID: "alice-2", ChatJID: "status@broadcast", SenderJID: "alice@lid", SenderName: "A", Timestamp: now - 1_000, Kind: "image", MediaPath: "/tmp/two.jpg", Status: "received"},
		{ID: "bob-1", ChatJID: "status@broadcast", SenderJID: "bob@lid", SenderName: "Bob Push Name", Timestamp: now - 2_000, Kind: "text", Body: "hello", Status: "received"},
		{ID: "alice-1", ChatJID: "status@broadcast", SenderJID: "alice@lid", SenderName: "A", Timestamp: now - 3_000, Kind: "text", Body: "first", Status: "received"},
		{ID: "deleted", ChatJID: "status@broadcast", SenderJID: "bob@lid", SenderName: "Bob Push Name", Timestamp: now - 4_000, Kind: "image", Revoked: true, Status: "received"},
		{ID: "expired", ChatJID: "status@broadcast", SenderJID: "old@lid", SenderName: "Expired", Timestamp: now - int64(25*time.Hour/time.Millisecond), Kind: "text", Body: "old", Status: "received"},
	}
	for _, update := range updates {
		if err := st.UpsertMessage(ctx, update, "Status", false); err != nil {
			t.Fatal(err)
		}
	}

	result, err := svc.Handle(ctx, "statuses.list", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	groups := result.([]model.StatusGroup)
	if len(groups) != 2 {
		t.Fatalf("expected two active senders, got %#v", groups)
	}
	if groups[0].SenderJID != "alice@lid" || groups[0].SenderName != "Alice Address Book" || groups[0].AvatarPath != "/tmp/alice.jpg" {
		t.Fatalf("contact identity was not resolved: %#v", groups[0])
	}
	if len(groups[0].Items) != 2 || groups[0].Items[0].ID != "alice-1" || groups[0].Items[1].ID != "alice-2" {
		t.Fatalf("story playback order is wrong: %#v", groups[0].Items)
	}
	if groups[1].SenderJID != "bob@lid" || groups[1].SenderName != "Bob Push Name" || len(groups[1].Items) != 1 {
		t.Fatalf("push-name fallback or revoked filtering is wrong: %#v", groups[1])
	}
}

type fakeGateway struct {
	gateway.Unavailable
	sentText        string
	sentPreview     model.LinkPreview
	historyChat     string
	refreshedChat   string
	downloadMessage string
	resolvedPhone   string
}

func (f *fakeGateway) Subscribe(func(gateway.Event)) func() { return func() {} }
func (f *fakeGateway) SendText(_ context.Context, req gateway.TextRequest) (model.Message, error) {
	f.sentText = req.Text
	f.sentPreview = req.Preview
	return model.Message{ID: "out-1", ChatJID: req.ChatJID, SenderJID: "me@s.whatsapp.net", Timestamp: 10, Kind: "text", Body: req.Text, FromMe: true, Status: "sent", ReplyTo: req.ReplyTo, LinkURL: req.Preview.URL, LinkTitle: req.Preview.Title, LinkDescription: req.Preview.Description}, nil
}

func (f *fakeGateway) RequestHistory(_ context.Context, chat string, _ int) error {
	f.historyChat = chat
	return nil
}

func (f *fakeGateway) RefreshHistory(_ context.Context, chat string, _ int) error {
	f.refreshedChat = chat
	return nil
}

func (f *fakeGateway) DownloadMedia(_ context.Context, chat, messageID string) (model.Message, error) {
	f.downloadMessage = messageID
	return model.Message{ID: messageID, ChatJID: chat, Kind: "document", MediaPath: "/tmp/report.pdf"}, nil
}

func (f *fakeGateway) ResolvePhone(_ context.Context, phone string) (model.Chat, error) {
	f.resolvedPhone = phone
	return model.Chat{JID: "123@lid", Title: "123"}, nil
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
	result, err := svc.Handle(context.Background(), "message.send", json.RawMessage(`{"chat_jid":"alice@s.whatsapp.net","text":"hello https://example.com","reply_to":"","link_preview":{"url":"https://example.com","title":"Example"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if gw.sentText != "hello https://example.com" {
		t.Fatalf("gateway received %q", gw.sentText)
	}
	if gw.sentPreview.URL != "https://example.com" || gw.sentPreview.Title != "Example" {
		t.Fatalf("gateway lost link preview: %#v", gw.sentPreview)
	}
	if result.(model.Message).ID != "out-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
	page, err := st.ListMessages(context.Background(), "alice@s.whatsapp.net", 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].Body != "hello https://example.com" || page.Messages[0].LinkTitle != "Example" {
		t.Fatalf("message not persisted: %#v", page)
	}
}

func TestContactInfoMethodsExposeLocalHistory(t *testing.T) {
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gw := &fakeGateway{}
	svc := New(st, gw, events.New())
	defer svc.Close()
	ctx := context.Background()
	if err := st.UpsertMessage(ctx, model.Message{ID: "p1", ChatJID: "15551234567@s.whatsapp.net", Timestamp: 1, Kind: "image", MediaPath: "/tmp/p.jpg", Status: "received"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Handle(ctx, "chat.info", json.RawMessage(`{"chat_jid":"15551234567@s.whatsapp.net"}`))
	if err != nil {
		t.Fatal(err)
	}
	info := result.(model.ChatInfo)
	if info.Phone != "15551234567" || info.MediaCount != 1 {
		t.Fatalf("unexpected info: %#v", info)
	}
	result, err = svc.Handle(ctx, "chat.shared", json.RawMessage(`{"chat_jid":"15551234567@s.whatsapp.net","category":"media","limit":10}`))
	if err != nil {
		t.Fatal(err)
	}
	page := result.(model.SharedMessagePage)
	if len(page.Messages) != 1 || page.Messages[0].ID != "p1" {
		t.Fatalf("unexpected shared page: %#v", page)
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
	if _, err := svc.Handle(context.Background(), "history.refresh", json.RawMessage(`{"chat_jid":"alice@s.whatsapp.net","limit":50}`)); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Handle(context.Background(), "message.download", json.RawMessage(`{"chat_jid":"alice@s.whatsapp.net","message_id":"doc-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if gw.historyChat != "alice@s.whatsapp.net" || gw.refreshedChat != "alice@s.whatsapp.net" || gw.downloadMessage != "doc-1" {
		t.Fatalf("gateway calls missing: history=%q refresh=%q download=%q", gw.historyChat, gw.refreshedChat, gw.downloadMessage)
	}
	if result.(model.Message).MediaPath != "/tmp/report.pdf" {
		t.Fatalf("unexpected download result: %#v", result)
	}
}

func TestContactSaveResolvesAndStoresLocalLabel(t *testing.T) {
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gw := &fakeGateway{}
	svc := New(st, gw, events.New())
	defer svc.Close()
	result, err := svc.Handle(context.Background(), "contact.save", json.RawMessage(`{"phone":"15551234567","name":"Alice Bot"}`))
	if err != nil {
		t.Fatal(err)
	}
	chat := result.(model.Chat)
	if gw.resolvedPhone != "15551234567" || chat.JID != "123@lid" || chat.Title != "Alice Bot" {
		t.Fatalf("unexpected saved contact: phone=%q chat=%#v", gw.resolvedPhone, chat)
	}
	stored, err := st.GetChat(context.Background(), "123@lid")
	if err != nil || stored.Title != "Alice Bot" {
		t.Fatalf("local label not stored: chat=%#v err=%v", stored, err)
	}
	if err := st.ApplyChatSnapshot(context.Background(), model.Chat{JID: "123@lid", Title: "Remote Profile Name"}); err != nil {
		t.Fatal(err)
	}
	stored, err = st.GetChat(context.Background(), "123@lid")
	if err != nil || stored.Title != "Alice Bot" {
		t.Fatalf("remote sync replaced local label: chat=%#v err=%v", stored, err)
	}
}

func TestDiscoveryListsCompleteAutomationSurface(t *testing.T) {
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := New(st, &fakeGateway{}, events.New())
	defer svc.Close()
	result, err := svc.Handle(context.Background(), "rpc.discover", nil)
	if err != nil {
		t.Fatal(err)
	}
	discovery := result.(map[string]any)
	methods := discovery["methods"].([]MethodDescription)
	seen := make(map[string]bool, len(methods))
	for _, method := range methods {
		if seen[method.Name] {
			t.Fatalf("duplicate method %q", method.Name)
		}
		seen[method.Name] = true
	}
	for _, required := range []string{"rpc.discover", "message.send", "message.send_media", "messages.list", "contact.save", "chat.set_read"} {
		if !seen[required] {
			t.Fatalf("discovery omitted %q", required)
		}
	}
}
