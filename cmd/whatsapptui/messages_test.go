package main

import (
	"strings"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/rpc"
)

const (
	alice   = "972500000001@s.whatsapp.net"
	channel = "120363000000000001@newsletter"
	noon    = int64(1762250400000) // 15:00 where this runs, give or take
)

func aliceChat() chatsMsg {
	return chatsMsg([]model.Chat{{JID: alice, Title: "Alice Example", UnreadCount: 2}})
}

func aliceMessages() model.MessagePage {
	return model.MessagePage{
		Messages: []model.Message{
			{ID: "m1", ChatJID: alice, SenderJID: alice, SenderName: "Alice Example",
				Timestamp: noon, Kind: "text", Body: "see you there"},
			{ID: "m2", ChatJID: alice, FromMe: true, Timestamp: noon + 60_000,
				Kind: "text", Body: "on my way", Status: "delivered", DeliveredAt: noon + 61_000},
			{ID: "m3", ChatJID: alice, SenderJID: alice, SenderName: "Alice Example",
				Timestamp: noon + 120_000, Kind: "text", Body: "I am outside"},
		},
		UnreadCount: 2, FirstUnreadID: "m1",
	}
}

// Opening a conversation is the sequence the desktop client runs: ask for the
// newest page, then tell the people in it that what they wrote has been seen.
// Both halves matter - the receipt appears on somebody else's phone - so this
// checks what was asked for and not only what was drawn.
func TestOpeningAConversationReadsItsPageAndSendsTheReceipts(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())
	current := driveFake(t, fake, aliceChat(), press("enter"))

	asked := fake.only(t, "messages.list")
	if asked.text("chat_jid") != alice || asked.number("before") != 0 || asked.number("limit") != 50 {
		t.Fatalf("the page was asked for as %v", asked.params)
	}
	receipt := fake.only(t, "chat.read")
	if receipt.text("chat_jid") != alice || receipt.text("sender_jid") != alice {
		t.Fatalf("the receipt went to %v", receipt.params)
	}
	// One receipt for each person who wrote, naming their messages and
	// carrying the newest of their timestamps.
	if ids := receipt.texts("message_ids"); strings.Join(ids, ",") != "m1,m3" {
		t.Fatalf("the receipt named %v", ids)
	}
	if receipt.number("timestamp") != noon+120_000 {
		t.Fatalf("the receipt carried %d, wanted the newest message's time", receipt.number("timestamp"))
	}

	view := current.View()
	for _, wanted := range []string{"see you there", "on my way", "I am outside", "2 unread messages", "Alice Example"} {
		if !strings.Contains(view, wanted) {
			t.Fatalf("the conversation does not show %q:\n%s", wanted, view)
		}
	}
}

// Reaching the top asks the daemon for the page before. Once the daemon has
// served everything it holds, the account is asked - once for each oldest
// message, because scrolling asked for it and nobody typed anything.
func TestReachingTheTopAsksForOlderMessagesThenForHistory(t *testing.T) {
	first := aliceMessages()
	first.HasMore, first.NextBefore, first.NextBeforeID = true, noon, "m1"
	// A page can overlap the one above it when messages share a timestamp,
	// so this one carries a message already on screen.
	older := model.MessagePage{Messages: []model.Message{
		{ID: "m0", ChatJID: alice, SenderJID: alice, Timestamp: noon - 60_000, Kind: "text", Body: "yesterday"},
		aliceMessages().Messages[0],
	}}
	fake := newFakeDaemon().answer("messages.list", first, older)

	current := driveFake(t, fake, aliceChat(), press("enter"))
	fake.forget()
	current = pump(t, current, press("g"))

	pages := fake.calls("messages.list")
	if len(pages) != 1 {
		t.Fatalf("older messages were asked for %d times: %v", len(pages), fake.calls(""))
	}
	if pages[0].number("before") != noon || pages[0].text("before_id") != "m1" {
		t.Fatalf("the older page was asked for as %v", pages[0].params)
	}
	// That page was the last the daemon holds, so the account is asked next.
	history := fake.only(t, "history.request")
	if history.text("chat_jid") != alice || history.number("limit") != 50 {
		t.Fatalf("history was asked for as %v", history.params)
	}
	if !strings.Contains(current.View(), "yesterday") {
		t.Fatalf("the older page is not on screen:\n%s", current.View())
	}
	// A conversation showing the same message twice is a fault the reader
	// sees, so an id already on screen is not added a second time.
	if twice := strings.Count(current.View(), "see you there"); twice != 1 {
		t.Fatalf("a message in both pages was drawn %d times:\n%s", twice, current.View())
	}

	// Asking again for the same oldest message would ask the account twice
	// for the same thing.
	fake.forget()
	current = pump(t, current, press("g"))
	if again := fake.calls("history.request"); len(again) != 0 {
		t.Fatalf("history was asked for a second time at the same boundary: %v", again)
	}
}

// A message arriving while the reader is looking at the conversation belongs
// in it, and is seen. Only the page that opens a chat used to send receipts,
// so a message arriving after that stayed unread until it was reopened.
func TestAMessageArrivingInTheOpenConversationIsShownAndSeen(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())
	current := driveFake(t, fake, aliceChat(), press("enter"))
	fake.forget()

	current = pump(t, current, eventMsg(rpc.Event{Event: "message.upsert", Data: model.Message{
		ID: "m4", ChatJID: alice, SenderJID: alice, SenderName: "Alice Example",
		Timestamp: noon + 180_000, Kind: "text", Body: "knocking",
	}}))
	if !strings.Contains(current.View(), "knocking") {
		t.Fatalf("the new message is not in the conversation:\n%s", current.View())
	}
	receipt := fake.only(t, "chat.read")
	if ids := receipt.texts("message_ids"); strings.Join(ids, ",") != "m4" {
		t.Fatalf("the receipt named %v, wanted the message that arrived", ids)
	}

	// One for a conversation nobody is looking at belongs to the list, not
	// here, and sends no receipt.
	fake.forget()
	current = pump(t, current, eventMsg(rpc.Event{Event: "message.upsert", Data: model.Message{
		ID: "x1", ChatJID: "972500000099@s.whatsapp.net", Kind: "text", Body: "somewhere else",
	}}))
	if strings.Contains(current.View(), "somewhere else") {
		t.Fatalf("another conversation's message was drawn into this one:\n%s", current.View())
	}
	if sent := fake.calls("chat.read"); len(sent) != 0 {
		t.Fatalf("a receipt was sent for another conversation: %v", sent)
	}
	if asked := fake.calls("chats.list"); len(asked) == 0 {
		t.Fatal("the conversation list was not asked for again")
	}
}

// An edit, a reaction or a deletion re-reads that one message. Reading the
// conversation again would throw away every older page the reader had loaded
// and drop them back at the newest message.
func TestAnEditReReadsTheOneMessageAndNotThePage(t *testing.T) {
	edited := aliceMessages().Messages[0]
	edited.Body, edited.Edited = "see you at six", true
	fake := newFakeDaemon().answer("messages.list", aliceMessages()).answer("message.get", edited)

	current := driveFake(t, fake, aliceChat(), press("enter"))
	fake.forget()
	current = pump(t, current, eventMsg(rpc.Event{Event: "message.edited",
		Data: map[string]any{"chat_jid": alice, "message_id": "m1", "body": "see you at six"}}))

	asked := fake.only(t, "message.get")
	if asked.text("message_id") != "m1" {
		t.Fatalf("the wrong message was re-read: %v", asked.params)
	}
	if pages := fake.calls("messages.list"); len(pages) != 0 {
		t.Fatalf("the whole conversation was read again: %v", pages)
	}
	view := current.View()
	if !strings.Contains(view, "see you at six") || !strings.Contains(view, "edited") {
		t.Fatalf("the edit is not on screen:\n%s", view)
	}
	if strings.Contains(view, "see you there") {
		t.Fatalf("the old wording is still on screen:\n%s", view)
	}
}

// A receipt moves the marks beside a message that was sent from here. They
// were stored but never reached the open conversation, so a sent message kept
// its single mark until the conversation was opened again.
func TestAReceiptMovesTheMarksOnAMessageWeSent(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())
	current := driveFake(t, fake, aliceChat(), press("enter"))
	if !strings.Contains(current.View(), "✓✓") || strings.Contains(current.View(), "✓✓ read") {
		t.Fatalf("a delivered message should carry two marks and not say read:\n%s", current.View())
	}
	current = pump(t, current, eventMsg(rpc.Event{Event: "message.receipt", Data: map[string]any{
		"chat_jid": alice, "message_ids": []string{"m2"}, "status": "read", "timestamp": noon + 200_000,
	}}))
	if !strings.Contains(current.View(), "✓✓ read") {
		t.Fatalf("the receipt did not reach the conversation:\n%s", current.View())
	}
}

// Nothing is drawn in the terminal. A picture, a recording or a file is named
// instead, with what is known about it, and opening it is what hands the file
// to another program.
func TestMediaIsNamedRatherThanDrawn(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{
		{ID: "p1", ChatJID: alice, SenderJID: alice, Timestamp: noon, Kind: "image",
			MediaName: "balcony.jpg", MediaSize: 1_258_291, Body: "the view from here"},
		{ID: "p2", ChatJID: alice, SenderJID: alice, Timestamp: noon, Kind: "document",
			MediaName: "lease.pdf", MediaSize: 245_760},
		{ID: "p3", ChatJID: alice, SenderJID: alice, Timestamp: noon, Kind: "audio", MediaDuration: 72},
		{ID: "p4", ChatJID: alice, SenderJID: alice, Timestamp: noon, Kind: "contact",
			ContactName: "Bob Example", ContactPhone: "+972500000002"},
	}})
	view := driveFake(t, fake, aliceChat(), press("enter")).View()
	for _, wanted := range []string{
		"[photo · balcony.jpg · 1.2 MB]", "the view from here",
		"[document · lease.pdf · 240 KB]", "[voice note · 1:12]",
		"[contact · Bob Example · +972500000002]",
	} {
		if !strings.Contains(view, wanted) {
			t.Fatalf("the conversation does not name %q:\n%s", wanted, view)
		}
	}
}

// Every one of these fields was written by somebody else, and a terminal obeys
// some characters instead of printing them. Not one of them may carry an
// escape sequence through to the terminal.
func TestEverythingTheConversationDrawsIsCleanedFirst(t *testing.T) {
	escape := "\x1b[2J\x1b]0;owned\x07"
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{{
		ID: "m1", ChatJID: alice, SenderJID: alice,
		SenderName:   "Alice" + escape,
		Timestamp:    noon,
		Kind:         "document",
		Body:         "look" + escape + "\nat this",
		MediaName:    "lease" + escape + ".pdf",
		ReplyTo:      "m0",
		ReplyPreview: "earlier" + escape,
		ReplySender:  "Bob" + escape,
		Reactions:    []model.Reaction{{Emoji: "👍" + escape}},
	}}})
	current := driveFake(t, fake, chatsMsg([]model.Chat{{JID: alice, Title: "Alice" + escape}}), press("enter"))
	view := current.View()
	for _, order := range []string{"\x1b[2J", "\x1b]0;", "\x07"} {
		if strings.Contains(view, order) {
			t.Fatalf("the conversation passed %q through to the terminal:\n%q", order, view)
		}
	}
	// A message is typed with the line breaks its sender chose, and those are
	// part of what was said.
	if !strings.Contains(view, "look") || !strings.Contains(view, "at this") {
		t.Fatalf("cleaning the message removed the message:\n%s", view)
	}
	if lines := strings.Count(safeText("look\nat this"), "\n"); lines != 1 {
		t.Fatalf("a message of two lines became %d", lines+1)
	}
}

// A channel has no read receipts to send. Nobody is waiting to know that a
// follower opened a post, and the address is not one a receipt can be sent to.
func TestAChannelIsOpenedWithoutSendingReceipts(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{
		{ID: "n1", ChatJID: channel, Timestamp: noon, Kind: "text", Body: "a post"},
	}})
	current := driveFake(t, fake,
		chatsMsg([]model.Chat{{JID: channel, Title: "A Channel"}}), press("enter"))
	if sent := fake.calls("chat.read"); len(sent) != 0 {
		t.Fatalf("opening a channel sent %d receipts: %v", len(sent), sent)
	}
	if !strings.Contains(current.View(), "a post") {
		t.Fatalf("the channel's post is not on screen:\n%s", current.View())
	}
}

// Moving the cursor down the list must ask the account for nothing. Opening a
// conversation tells the people in it that their messages were read, and
// running the cursor through two hundred rows would tell two hundred people.
func TestMovingTheCursorThroughTheListAsksForNothing(t *testing.T) {
	fake := newFakeDaemon()
	driveFake(t, fake, chatsMsg([]model.Chat{
		{JID: alice, Title: "Alice Example"},
		{JID: "972500000002@s.whatsapp.net", Title: "Bob Example"},
	}), press("j"), press("j"), press("k"))
	if asked := fake.calls(""); len(asked) != 0 {
		t.Fatalf("moving the cursor asked the daemon for %v", asked)
	}
}
