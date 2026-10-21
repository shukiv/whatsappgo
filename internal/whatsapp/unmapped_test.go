package whatsapp

import (
	"context"
	"testing"
	"time"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/notify"
	"github.com/shukiv/whatsappgo/internal/store"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func receivingClient(t *testing.T) (*Client, *store.Store, *[]gateway.Event) {
	t.Helper()
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	// baseCtx and a notifier are what the receiving path needs to run at all:
	// without them an incoming message panics in the notification goroutine.
	c := &Client{store: st, subs: make(map[uint64]func(gateway.Event)),
		baseCtx: context.Background(), notifier: notify.Noop{}}
	var seen []gateway.Event
	c.Subscribe(func(evt gateway.Event) { seen = append(seen, evt) })
	return c, st, &seen
}

func incoming(id string, msg *waE2E.Message) *waEvents.Message {
	return &waEvents.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{
			Chat:   types.NewJID("123", types.DefaultUserServer),
			Sender: types.NewJID("123", types.DefaultUserServer),
		}, ID: id, PushName: "Alice", Timestamp: time.UnixMilli(1000)},
		Message: msg,
	}
}

func named(events []gateway.Event, name string) *gateway.Event {
	for i := range events {
		if events[i].Name == name {
			return &events[i]
		}
	}
	return nil
}

// The guard that catches a release where reading a new message shape breaks
// reading the ordinary ones.
func TestAPlainMessageStillArrives(t *testing.T) {
	c, st, seen := receivingClient(t)
	c.handleMessage(incoming("plain1", &waE2E.Message{Conversation: proto.String("hello")}))
	upsert := named(*seen, "message.upsert")
	if upsert == nil {
		t.Fatalf("a plain text message produced no upsert: %#v", *seen)
	}
	if named(*seen, "message.unmapped") != nil {
		t.Fatalf("a plain text message was reported as unreadable: %#v", *seen)
	}
	stored, err := st.GetMessage(context.Background(), types.NewJID("123", types.DefaultUserServer).String(), "plain1")
	if err != nil {
		t.Fatalf("a plain text message was not stored: %v", err)
	}
	if stored.Body != "hello" || stored.Kind != "text" {
		t.Fatalf("stored = %#v", stored)
	}
}

// A tap must reach a caller as both the message the reader sees and the event
// that names the choice. This is what failed on a live phone: the tap was
// delivered and acknowledged, and then nothing came out of the daemon at all.
func TestATapReachesTheSocket(t *testing.T) {
	c, st, seen := receivingClient(t)
	// The question this answers has to be one this account asked, or the answer
	// arrives marked unverified rather than acted upon.
	if err := st.UpsertMessage(context.Background(), model.Message{
		ID: "prompt1", ChatJID: types.NewJID("123", types.DefaultUserServer).String(),
		SenderJID: "me@s.whatsapp.net", Timestamp: 1, Kind: "text", Body: "Do you see them?",
		FromMe: true, Status: "sent",
	}, "", false); err != nil {
		t.Fatal(err)
	}
	c.handleMessage(incoming("tap1", &waE2E.Message{TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{
		SelectedID:          proto.String("yes"),
		SelectedDisplayText: proto.String("Yes, I see them"),
		ContextInfo:         replyTo("prompt1"),
	}}))
	if named(*seen, "message.upsert") == nil {
		t.Fatalf("the tap never arrived as a message: %#v", *seen)
	}
	tapped := named(*seen, "message.tapped")
	if tapped == nil {
		t.Fatalf("the tap was not reported: %#v", *seen)
	}
	data, ok := tapped.Data.(map[string]string)
	if !ok {
		t.Fatalf("tap data = %#v", tapped.Data)
	}
	for key, want := range map[string]string{
		"selected_id": "yes", "selected_title": "Yes, I see them",
		"message_id": "prompt1", "tap_id": "tap1", "verified": "1",
	} {
		if data[key] != want {
			t.Fatalf("%s = %q, want %q", key, data[key], want)
		}
	}
}

// Everything in an answer is written by whoever sent it, including which
// question it claims to answer. A caller may be waiting on one of these to
// approve something, so an answer that cannot be tied to a question this
// account asked in this conversation must not arrive looking like one that can.
func TestATapIsOnlyVerifiedAgainstAQuestionThisAccountAsked(t *testing.T) {
	chat := types.NewJID("123", types.DefaultUserServer).String()
	for _, tc := range []struct {
		name    string
		prompt  *model.Message
		replyTo string
		want    string
	}{
		{"answers a question this account asked", &model.Message{
			ID: "prompt1", ChatJID: chat, SenderJID: chat, Timestamp: 1, Kind: "text",
			Body: "Approve?", FromMe: true, Status: "sent",
		}, "prompt1", "1"},
		// Someone writing into the conversation can name a message this account
		// never sent, or one it only received.
		{"answers a message this account received", &model.Message{
			ID: "theirs1", ChatJID: chat, SenderJID: chat, Timestamp: 1, Kind: "text",
			Body: "Approve?", Status: "received",
		}, "theirs1", "0"},
		{"answers a message nobody here has", nil, "unknown1", "0"},
		{"answers nothing at all", nil, "", "0"},
	} {
		c, st, seen := receivingClient(t)
		if tc.prompt != nil {
			if err := st.UpsertMessage(context.Background(), *tc.prompt, "Alice", false); err != nil {
				t.Fatal(err)
			}
		}
		var context_ *waE2E.ContextInfo
		if tc.replyTo != "" {
			context_ = replyTo(tc.replyTo)
		}
		c.handleMessage(incoming("tap-"+tc.name, &waE2E.Message{
			TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{
				SelectedID: proto.String("yes"), SelectedDisplayText: proto.String("Approve"),
				ContextInfo: context_,
			}}))
		tapped := named(*seen, "message.tapped")
		if tapped == nil {
			t.Fatalf("%s: the tap was not reported at all", tc.name)
		}
		data, _ := tapped.Data.(map[string]string)
		if data["verified"] != tc.want {
			t.Fatalf("%s: verified = %q, want %q", tc.name, data["verified"], tc.want)
		}
	}
}

// A question asked in another conversation is not an invitation to answer here.
func TestATapCannotBorrowAQuestionFromAnotherChat(t *testing.T) {
	c, st, seen := receivingClient(t)
	elsewhere := types.NewJID("999", types.DefaultUserServer).String()
	if err := st.UpsertMessage(context.Background(), model.Message{
		ID: "prompt1", ChatJID: elsewhere, SenderJID: elsewhere, Timestamp: 1, Kind: "text",
		Body: "Approve?", FromMe: true, Status: "sent",
	}, "Someone", false); err != nil {
		t.Fatal(err)
	}
	c.handleMessage(incoming("tap1", &waE2E.Message{
		TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{
			SelectedID: proto.String("yes"), ContextInfo: replyTo("prompt1"),
		}}))
	tapped := named(*seen, "message.tapped")
	if tapped == nil {
		t.Fatal("the tap was not reported at all")
	}
	data, _ := tapped.Data.(map[string]string)
	if data["verified"] != "0" {
		t.Fatalf("a question from another conversation was accepted: %#v", data)
	}
}

// A message this client cannot read must say so. Dropping it in silence is how
// a tapped button vanished with nothing in the log to show it had arrived.
func TestAMessageThisClientCannotReadIsReported(t *testing.T) {
	c, _, seen := receivingClient(t)
	c.handleMessage(incoming("odd1", &waE2E.Message{EncEventResponseMessage: &waE2E.EncEventResponseMessage{}}))
	unmapped := named(*seen, "message.unmapped")
	if unmapped == nil {
		t.Fatalf("an unreadable message was dropped in silence: %#v", *seen)
	}
	data, ok := unmapped.Data.(map[string]string)
	if !ok {
		t.Fatalf("report data = %#v", unmapped.Data)
	}
	if data["fields"] != "encEventResponseMessage" {
		t.Fatalf("the report does not name what arrived: %q", data["fields"])
	}
	if data["message_id"] != "odd1" {
		t.Fatalf("message_id = %q", data["message_id"])
	}
}

// Key exchange is machinery, not a message that failed to be read. Reporting
// it would fill the log on every conversation and bury the real ones.
func TestTransportEnvelopesAreNotReported(t *testing.T) {
	c, _, seen := receivingClient(t)
	c.handleMessage(incoming("key1", &waE2E.Message{
		SenderKeyDistributionMessage: &waE2E.SenderKeyDistributionMessage{},
	}))
	if named(*seen, "message.unmapped") != nil {
		t.Fatalf("key distribution was reported as an unreadable message: %#v", *seen)
	}
}
