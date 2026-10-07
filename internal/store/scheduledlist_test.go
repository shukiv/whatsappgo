package store

import (
	"context"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
)

// A conversation whose only content is a message waiting to go still belongs
// in the chat list: the reader left work there and nothing else records it.
func TestAChatWithOnlyAScheduledMessageIsListed(t *testing.T) {
	ctx := context.Background()
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const chat = "15551234567@s.whatsapp.net"
	if err := st.UpsertChat(ctx, model.Chat{JID: chat, Title: "Alice"}); err != nil {
		t.Fatal(err)
	}
	listed, err := st.ListChats(ctx, 50, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("a silent chat was listed before anything was scheduled: %#v", listed)
	}

	if err := st.ScheduleMessage(ctx, model.ScheduledMessage{
		ID: "s1", ChatJID: chat, Text: "morning", SendAt: 9_000, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	listed, err = st.ListChats(ctx, 50, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("a chat holding a scheduled message was not listed: %#v", listed)
	}
	if listed[0].JID != chat || listed[0].ScheduledCount != 1 {
		t.Fatalf("the row does not say a message is waiting: %#v", listed[0])
	}
}

// Once it has gone it is an ordinary message, and the count stops counting it.
func TestASentScheduledMessageStopsCounting(t *testing.T) {
	ctx := context.Background()
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const chat = "15551234567@s.whatsapp.net"
	if err := st.UpsertChat(ctx, model.Chat{JID: chat, Title: "Alice", Pinned: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.ScheduleMessage(ctx, model.ScheduledMessage{
		ID: "s1", ChatJID: chat, Text: "morning", SendAt: 9_000, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkScheduledSent(ctx, "s1", "m1", 10_000); err != nil {
		t.Fatal(err)
	}
	listed, err := st.ListChats(ctx, 50, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("the pinned chat disappeared: %#v", listed)
	}
	if listed[0].ScheduledCount != 0 {
		t.Fatalf("a message that has gone is still counted as waiting: %#v", listed[0])
	}
}

// Being listed is not enough: the row has to be where the reader will see it.
// A chat whose only content is a message scheduled a moment ago sorts above a
// conversation that last spoke a week earlier, the same way sending would.
func TestAScheduledChatSortsByWhenItWasScheduled(t *testing.T) {
	ctx := context.Background()
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const stale = "15550000001@s.whatsapp.net"
	const scheduled = "15550000002@s.whatsapp.net"
	if err := st.UpsertMessage(ctx, model.Message{
		ID: "old", ChatJID: stale, Body: "last week", Kind: "text", Timestamp: 1_000,
	}, "Bob", false); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertChat(ctx, model.Chat{JID: scheduled, Title: "Alice"}); err != nil {
		t.Fatal(err)
	}
	if err := st.ScheduleMessage(ctx, model.ScheduledMessage{
		ID: "s1", ChatJID: scheduled, Text: "morning", SendAt: 90_000, CreatedAt: 5_000,
	}); err != nil {
		t.Fatal(err)
	}

	listed, err := st.ListChats(ctx, 50, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("want both chats listed, got %#v", listed)
	}
	if listed[0].JID != scheduled {
		t.Fatalf("the chat with the waiting message sorted below the stale one: %#v", listed)
	}
}

// Scheduling the first thing ever said to somebody has to make a chat for it,
// or the conversation has nowhere to appear in the list.
func TestSchedulingToAnUnknownChatCreatesItsRow(t *testing.T) {
	ctx := context.Background()
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const chat = "15557654321@s.whatsapp.net"
	if err := st.ScheduleMessage(ctx, model.ScheduledMessage{
		ID: "s1", ChatJID: chat, Text: "hello", SendAt: 9_000, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	listed, err := st.ListChats(ctx, 50, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].JID != chat {
		t.Fatalf("a chat that exists only because of a scheduled message was not listed: %#v", listed)
	}
	if listed[0].ScheduledCount != 1 {
		t.Fatalf("the row does not say a message is waiting: %#v", listed[0])
	}
	if listed[0].Title == "" {
		t.Fatalf("the row has no name to show: %#v", listed[0])
	}
}
