package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/store"
)

type promptGateway struct {
	gateway.Unavailable
	sent  gateway.ButtonPromptRequest
	calls int
}

func (g *promptGateway) SendButtonPrompt(_ context.Context, req gateway.ButtonPromptRequest) (model.Message, error) {
	g.sent = req
	g.calls++
	return model.Message{ID: "prompt1", ChatJID: req.ChatJID, Timestamp: 1000, Kind: "text", Body: req.Prompt.Text, FromMe: true, Status: "sent"}, nil
}

func promptService(t *testing.T, gw gateway.Gateway) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	svc := New(st, gw, events.New())
	t.Cleanup(func() {
		svc.Close()
		st.Close()
	})
	return svc, st
}

func TestSendingAPromptStoresTheQuestion(t *testing.T) {
	gw := &promptGateway{}
	svc, st := promptService(t, gw)
	result, err := svc.Handle(context.Background(), "message.send_buttons",
		json.RawMessage(`{"chat_jid":"alice@lid","text":" Approve the transfer? ","footer":"ref 4821","buttons":[{"id":" yes ","title":"Approve"},{"id":"no","title":"Reject"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if gw.calls != 1 {
		t.Fatalf("gateway calls = %d", gw.calls)
	}
	// The prompt must reach the gateway checked and trimmed, not as it arrived.
	if gw.sent.Prompt.Text != "Approve the transfer?" || len(gw.sent.Prompt.Buttons) != 2 || gw.sent.Prompt.Buttons[0].ID != "yes" {
		t.Fatalf("prompt reached the gateway unchecked: %#v", gw.sent.Prompt)
	}
	msg, ok := result.(model.Message)
	if !ok || msg.ID != "prompt1" {
		t.Fatalf("result = %#v", result)
	}
	stored, err := st.GetMessage(context.Background(), "alice@lid", "prompt1")
	if err != nil {
		t.Fatalf("the prompt was not stored: %v", err)
	}
	if stored.Body != "Approve the transfer?" {
		t.Fatalf("stored body = %q", stored.Body)
	}
}

func TestAPromptThatCouldNotBeAnsweredNeverReachesWhatsApp(t *testing.T) {
	gw := &promptGateway{}
	svc, _ := promptService(t, gw)
	for _, params := range []string{
		`{"chat_jid":"alice@lid","text":"Approve?","buttons":[{"id":"a","title":"A"},{"id":"b","title":"B"},{"id":"c","title":"C"},{"id":"d","title":"D"}]}`,
		`{"chat_jid":"alice@lid","text":"Approve?","buttons":[{"id":"yes","title":"Approve"},{"id":"yes","title":"Reject"}]}`,
		`{"chat_jid":"alice@lid","text":"   ","buttons":[{"id":"yes","title":"Approve"}]}`,
		`{"chat_jid":"alice@lid","text":"Approve?","buttons":[]}`,
	} {
		_, err := svc.Handle(context.Background(), "message.send_buttons", json.RawMessage(params))
		if err == nil {
			t.Fatalf("accepted %s", params)
		}
		if strings.Contains(err.Error(), "unavailable") {
			t.Fatalf("refused for the wrong reason: %v", err)
		}
	}
	if gw.calls != 0 {
		t.Fatalf("a refused prompt reached the gateway %d times", gw.calls)
	}
}

func TestAGatewayWithoutPromptsSaysSo(t *testing.T) {
	svc, _ := promptService(t, &fakeGateway{})
	_, err := svc.Handle(context.Background(), "message.send_buttons",
		json.RawMessage(`{"chat_jid":"alice@lid","text":"Approve?","buttons":[{"id":"yes","title":"Approve"}]}`))
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("err = %v", err)
	}
}
