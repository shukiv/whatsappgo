package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/store"
)

// A status update reaches a client as an ordinary message whose chat is the
// status broadcast address. A program that answers an incoming message by
// replying to the chat it came from must not publish a status update to its
// contacts, so the send methods refuse that destination outright.
func TestSendMethodsRefuseTheStatusBroadcastAddress(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gw := &fakeGateway{}
	svc := New(st, gw, events.New())
	defer svc.Close()

	calls := map[string]string{
		"message.send":         `{"chat_jid":"status@broadcast","text":"hello"}`,
		"message.send_media":   `{"chat_jid":"status@broadcast","path":"/tmp/whatsappgo-test.png"}`,
		"message.send_contact": `{"chat_jid":"status@broadcast","contact":{"name":"Alice","phone":"+15551234567"}}`,
		"message.forward":      `{"chat_jid":"alice@lid","message_id":"m1","to_chat_jid":"status@broadcast"}`,
		"sticker.send":         `{"chat_jid":"alice@lid","message_id":"m1","to_chat_jid":"status@broadcast"}`,
	}
	for method, params := range calls {
		_, err := svc.Handle(ctx, method, json.RawMessage(params))
		if err == nil {
			t.Fatalf("%s accepted the status broadcast address", method)
		}
		if !strings.Contains(err.Error(), "status.post") {
			t.Fatalf("%s refusal does not point at status.post: %v", method, err)
		}
	}
	if gw.sentChat != "" || gw.sentText != "" {
		t.Fatalf("a refused send still reached the gateway: %#v", gw)
	}
}

func TestSendMethodsRefuseChannelAddresses(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gw := &fakeGateway{}
	svc := New(st, gw, events.New())
	defer svc.Close()
	if _, err := svc.Handle(ctx, "message.send",
		json.RawMessage(`{"chat_jid":"123456@newsletter","text":"hello"}`)); err == nil {
		t.Fatal("message.send accepted a channel address")
	}
	if gw.sentChat != "" {
		t.Fatalf("a refused send still reached the gateway: %#v", gw)
	}
}

// Replying to a status is addressed to the person who posted it, and only the
// quoted message belongs to the broadcast address. That must keep working.
func TestSendStillAcceptsAStatusReplyAddressedToAPerson(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gw := &fakeGateway{}
	svc := New(st, gw, events.New())
	defer svc.Close()
	if _, err := svc.Handle(ctx, "message.send",
		json.RawMessage(`{"chat_jid":"alice@lid","text":"nice photo","reply_to":"status-1","reply_chat_jid":"status@broadcast"}`)); err != nil {
		t.Fatal(err)
	}
	if gw.sentChat != "alice@lid" || gw.sentReplyChat != "status@broadcast" {
		t.Fatalf("status reply was misrouted: %#v", gw)
	}
}
