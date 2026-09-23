package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

// A message written for later is words nobody else has a copy of, so the tests
// are about what happens to those words: they go once, at the right time, and
// a failure to send them does not lose them.
type schedulingGateway struct {
	gateway.Unavailable
	sent []gateway.TextRequest
	fail error
}

func (g *schedulingGateway) SendText(_ context.Context, req gateway.TextRequest) (model.Message, error) {
	if g.fail != nil {
		return model.Message{}, g.fail
	}
	g.sent = append(g.sent, req)
	return model.Message{ID: "sent-" + req.ChatJID, ChatJID: req.ChatJID, Body: req.Text, FromMe: true}, nil
}

func newSchedulingService(t *testing.T, at time.Time) (*Service, *schedulingGateway, *localstore.Store) {
	t.Helper()
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	wa := &schedulingGateway{}
	s := New(st, wa, events.New())
	t.Cleanup(s.Close)
	s.clock = func() time.Time { return at }
	return s, wa, st
}

const scheduledChat = "573112522689@s.whatsapp.net"

func TestAScheduledMessageGoesOnlyWhenItsTimeArrives(t *testing.T) {
	noon := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	s, wa, st := newSchedulingService(t, noon)
	ctx := context.Background()
	if _, err := s.scheduleMessage(ctx, scheduleParams{
		ChatJID: scheduledChat, Text: "Happy birthday", SendAt: noon.Add(time.Hour).UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	if sent := s.SendDueScheduledMessages(ctx); sent != 0 || len(wa.sent) != 0 {
		t.Fatalf("a message due in an hour was sent now: %d", sent)
	}
	waiting, err := st.ScheduledMessages(ctx, scheduledChat)
	if err != nil {
		t.Fatal(err)
	}
	if len(waiting) != 1 || waiting[0].Text != "Happy birthday" {
		t.Fatalf("the message is not waiting in the queue: %#v", waiting)
	}

	s.clock = func() time.Time { return noon.Add(time.Hour) }
	if sent := s.SendDueScheduledMessages(ctx); sent != 1 {
		t.Fatalf("%d messages went when the time came, want 1", sent)
	}
	if len(wa.sent) != 1 || wa.sent[0].Text != "Happy birthday" || wa.sent[0].ChatJID != scheduledChat {
		t.Fatalf("the wrong message went: %#v", wa.sent)
	}
	if waiting, err = st.ScheduledMessages(ctx, ""); err != nil {
		t.Fatal(err)
	} else if len(waiting) != 0 {
		t.Fatalf("a message that has gone is still queued: %#v", waiting)
	}
}

// The same words must not arrive twice. Sweeps run every few seconds, and a
// row that stayed due would be sent on every one of them.
func TestAMessageThatHasGoneIsNotSentAgain(t *testing.T) {
	noon := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	s, wa, _ := newSchedulingService(t, noon)
	ctx := context.Background()
	if _, err := s.scheduleMessage(ctx, scheduleParams{
		ChatJID: scheduledChat, Text: "Only once", SendAt: noon.Add(time.Minute).UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	s.clock = func() time.Time { return noon.Add(time.Hour) }
	s.SendDueScheduledMessages(ctx)
	s.SendDueScheduledMessages(ctx)
	if len(wa.sent) != 1 {
		t.Fatalf("the message was sent %d times", len(wa.sent))
	}
}

// A message that came due while the app was closed is late. Late is what this
// program can offer; losing it is not.
func TestAMessageThatCameDueWhileClosedGoesAtTheNextStart(t *testing.T) {
	noon := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	s, wa, _ := newSchedulingService(t, noon)
	ctx := context.Background()
	if _, err := s.scheduleMessage(ctx, scheduleParams{
		ChatJID: scheduledChat, Text: "Sorry I am late", SendAt: noon.Add(time.Minute).UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	// Hours pass with nothing running.
	s.clock = func() time.Time { return noon.Add(6 * time.Hour) }
	if sent := s.SendDueScheduledMessages(ctx); sent != 1 || len(wa.sent) != 1 {
		t.Fatalf("an overdue message did not go at the next start: %d", sent)
	}
}

// Sending can fail for reasons that pass: no network, a server that refuses
// once. The words stay in the queue and the reason is kept.
func TestAMessageThatCouldNotBeSentStaysInTheQueue(t *testing.T) {
	noon := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	s, wa, st := newSchedulingService(t, noon)
	ctx := context.Background()
	if _, err := s.scheduleMessage(ctx, scheduleParams{
		ChatJID: scheduledChat, Text: "Still here", SendAt: noon.Add(time.Minute).UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	wa.fail = errors.New("websocket disconnected")
	s.clock = func() time.Time { return noon.Add(time.Hour) }
	if sent := s.SendDueScheduledMessages(ctx); sent != 0 {
		t.Fatalf("a failed send was counted as sent: %d", sent)
	}
	waiting, err := st.ScheduledMessages(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(waiting) != 1 || waiting[0].Error == "" {
		t.Fatalf("the message was dropped or the reason was lost: %#v", waiting)
	}

	// And it goes when the connection comes back.
	wa.fail = nil
	if sent := s.SendDueScheduledMessages(ctx); sent != 1 || len(wa.sent) != 1 {
		t.Fatalf("the message did not go once sending worked again: %d", sent)
	}
}

// A time that has passed, a time absurdly far away, an empty message and a
// conversation this app cannot write to are all refused where they arrive.
func TestAScheduledMessageIsCheckedBeforeItIsStored(t *testing.T) {
	noon := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	s, _, st := newSchedulingService(t, noon)
	ctx := context.Background()
	cases := []struct {
		name   string
		params scheduleParams
	}{
		{"a time that has passed", scheduleParams{ChatJID: scheduledChat, Text: "Too late", SendAt: noon.Add(-time.Minute).UnixMilli()}},
		{"a time years away", scheduleParams{ChatJID: scheduledChat, Text: "Too far", SendAt: noon.AddDate(3, 0, 0).UnixMilli()}},
		{"nothing to say", scheduleParams{ChatJID: scheduledChat, Text: "   ", SendAt: noon.Add(time.Hour).UnixMilli()}},
		{"a conversation this app cannot write to", scheduleParams{ChatJID: "status@broadcast", Text: "No", SendAt: noon.Add(time.Hour).UnixMilli()}},
	}
	for _, tc := range cases {
		if _, err := s.scheduleMessage(ctx, tc.params); err == nil {
			t.Fatalf("%s was accepted", tc.name)
		}
	}
	waiting, err := st.ScheduledMessages(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(waiting) != 0 {
		t.Fatalf("a refused message was stored anyway: %#v", waiting)
	}
}

// Cancelling is the only way back once a message is written, so it has to
// actually stop it.
func TestACancelledMessageNeverGoes(t *testing.T) {
	noon := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	s, wa, st := newSchedulingService(t, noon)
	ctx := context.Background()
	scheduled, err := s.scheduleMessage(ctx, scheduleParams{
		ChatJID: scheduledChat, Text: "Never mind", SendAt: noon.Add(time.Minute).UnixMilli(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CancelScheduledMessage(ctx, scheduled.ID); err != nil {
		t.Fatal(err)
	}
	s.clock = func() time.Time { return noon.Add(time.Hour) }
	if sent := s.SendDueScheduledMessages(ctx); sent != 0 || len(wa.sent) != 0 {
		t.Fatalf("a cancelled message was sent: %d", sent)
	}
	if err := st.CancelScheduledMessage(ctx, scheduled.ID); err == nil {
		t.Fatal("cancelling something that is not there was reported as done")
	}
}
