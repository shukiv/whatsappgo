package main

import (
	"strings"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
)

const me = "972500000000@s.whatsapp.net"

// openAlice is the conversation open, the keyboard handed back to it, and the
// cursor on the first message - Alice's "see you there".
func openAlice(t *testing.T, fake *fakeDaemon, page model.MessagePage) *app {
	t.Helper()
	if len(fake.answers["messages.list"]) == 0 {
		fake.answer("messages.list", page)
	}
	current := driveFake(t, fake, statusMsg(status{LoggedIn: true, State: "connected", UserJID: me}),
		aliceChat(), press("enter"), press("esc"), press("g"))
	fake.forget()
	return current
}

// The reactions offered are the six the desktop client offers, in its order,
// and choosing the one already left takes it back - which is what tapping it
// does there.
func TestReactingOffersTheSameSixAndTakesMineBack(t *testing.T) {
	fake := newFakeDaemon()
	current := openAlice(t, fake, aliceMessages())

	current = pump(t, current, press("x"))
	if current.chooser == nil || len(current.chooser.items) != 6 {
		t.Fatalf("the prompt offers %v", current.chooser)
	}
	for at, emoji := range quickReactions {
		if !strings.HasPrefix(current.chooser.items[at].label, emoji) {
			t.Fatalf("reaction %d is %q, wanted %q", at+1, current.chooser.items[at].label, emoji)
		}
	}
	current = pump(t, current, press("1"))
	left := fake.only(t, "message.react")
	if left.text("message_id") != "m1" || left.text("emoji") != "👍" || left.text("sender_jid") != alice {
		t.Fatalf("the reaction went as %v", left.params)
	}

	// With that one already left from here, choosing it again takes it back.
	withMine := aliceMessages()
	withMine.Messages[0].Reactions = []model.Reaction{{SenderJID: me + ":12", Emoji: "👍"}}
	fake.forget()
	current = pump(t, current, pageMsg{generation: current.generation, chat: alice, page: withMine, mode: pageReset},
		press("g"), press("x"))
	if mine := current.myReaction(current.messages[0]); mine != "👍" {
		t.Fatalf("the reaction left from here reads as %q", mine)
	}
	current = pump(t, current, press("1"))
	back := fake.only(t, "message.react")
	if back.text("emoji") != "" {
		t.Fatalf("choosing the reaction already left sent %q", back.text("emoji"))
	}
}

// Keeping a message and stopping keeping it are the same key, and the daemon
// is told which way round it is going.
func TestKeepingAMessageTurnsItBothWays(t *testing.T) {
	fake := newFakeDaemon()
	current := openAlice(t, fake, aliceMessages())

	current = pump(t, current, press("s"))
	kept := fake.only(t, "message.star")
	if kept.params["starred"] != true || kept.text("message_id") != "m1" ||
		kept.text("sender_jid") != alice || kept.params["from_me"] != false {
		t.Fatalf("keeping it went as %v", kept.params)
	}

	// The daemon announces the new state, which is what moves the mark.
	fake.forget()
	current = pump(t, current, eventMsg(event("message.starred", map[string]any{
		"chat_jid": alice, "message_id": "m1", "starred": true})))
	if !strings.Contains(current.View(), "★") {
		t.Fatalf("the mark is not on screen:\n%s", current.View())
	}
	current = pump(t, current, press("s"))
	released := fake.only(t, "message.star")
	if released.params["starred"] != false {
		t.Fatalf("stopping keeping it went as %v", released.params)
	}
}

// Pinning asks how long, offers what the desktop client's dialog offers, and
// starts on the same answer that dialog starts on.
func TestPinningAsksHowLongAndStartsOnSevenDays(t *testing.T) {
	fake := newFakeDaemon()
	current := openAlice(t, fake, aliceMessages())

	current = pump(t, current, press("p"))
	if current.chooser == nil || current.chooser.cursor != 1 {
		t.Fatalf("the prompt starts on %v", current.chooser)
	}
	current = pump(t, current, press("enter"))
	pinned := fake.only(t, "message.pin")
	if pinned.number("duration_seconds") != 7*24*60*60 || pinned.text("message_id") != "m1" {
		t.Fatalf("the pin went as %v", pinned.params)
	}

	fake.forget()
	current = pump(t, current, press("p"), press("4"))
	unpinned := fake.only(t, "message.unpin")
	if unpinned.text("message_id") != "m1" || unpinned.text("chat_jid") != alice {
		t.Fatalf("the unpin went as %v", unpinned.params)
	}
}

// Passing a message on names the conversation it goes to, chosen from the list
// of them.
func TestPassingAMessageOnPicksTheConversation(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())
	current := driveFake(t, fake, chatsMsg([]model.Chat{
		{JID: alice, Title: "Alice Example"},
		{JID: "972500000002@s.whatsapp.net", Title: "Bob Example"},
	}), press("enter"), press("esc"), press("g"))
	fake.forget()

	current = pump(t, current, press("f"))
	if current.chooser == nil || len(current.chooser.items) != 2 {
		t.Fatalf("the prompt offers %v", current.chooser)
	}
	current = pump(t, current, press("2"))
	passed := fake.only(t, "message.forward")
	if passed.text("chat_jid") != alice || passed.text("message_id") != "m1" ||
		passed.text("to_chat_jid") != "972500000002@s.whatsapp.net" {
		t.Fatalf("the message was passed on as %v", passed.params)
	}
}

// Deleting asks first, as the desktop client's dialog does, and the answer the
// cursor starts on is the one that changes nothing: here the answer is a single
// keystroke and this one cannot be undone.
func TestDeletingAMessageAsksFirstAndKeepsItByDefault(t *testing.T) {
	fake := newFakeDaemon()
	current := openAlice(t, fake, aliceMessages())

	current = pump(t, current, press("d"))
	if current.chooser == nil || current.chooser.cursor != 0 {
		t.Fatalf("the prompt starts on %v", current.chooser)
	}
	if !strings.Contains(current.View(), "cannot be undone") {
		t.Fatalf("the prompt does not say what it does:\n%s", current.View())
	}
	current = pump(t, current, press("enter"))
	if deleted := fake.calls("message.delete"); len(deleted) != 0 {
		t.Fatalf("the default answer deleted the message: %v", deleted)
	}

	current = pump(t, current, press("d"), press("j"), press("enter"))
	deleted := fake.only(t, "message.delete")
	if deleted.text("message_id") != "m1" || deleted.text("sender_jid") != alice {
		t.Fatalf("the deletion went as %v", deleted.params)
	}
}

// What a message said before it was changed is read from the daemon, not
// guessed at from what is on screen.
func TestEarlierVersionsAreRead(t *testing.T) {
	fake := newFakeDaemon().answer("message.revisions", []model.MessageRevision{
		{MessageID: "m1", Revision: 1, Body: "see you there", RecordedAt: noon, Reason: "edited"},
	})
	current := openAlice(t, fake, aliceMessages())

	current = pump(t, current, press("v"))
	asked := fake.only(t, "message.revisions")
	if asked.text("chat_jid") != alice || asked.text("message_id") != "m1" {
		t.Fatalf("the earlier versions were asked for as %v", asked.params)
	}
	if !strings.Contains(current.View(), "see you there") || !strings.Contains(current.View(), "edited") {
		t.Fatalf("the earlier version is not on screen:\n%s", current.View())
	}
}

// The messages kept in this conversation are read from the daemon.
func TestKeptMessagesAreListed(t *testing.T) {
	fake := newFakeDaemon().answer("messages.starred", []model.Message{
		{ID: "m1", ChatJID: alice, SenderName: "Alice Example", Timestamp: noon, Kind: "text", Body: "the address"},
	})
	current := openAlice(t, fake, aliceMessages())

	current = pump(t, current, press("S"))
	asked := fake.only(t, "messages.starred")
	if asked.text("chat_jid") != alice || asked.number("limit") != 200 {
		t.Fatalf("the kept messages were asked for as %v", asked.params)
	}
	if !strings.Contains(current.View(), "the address") {
		t.Fatalf("the kept message is not on screen:\n%s", current.View())
	}
}

// A prompt holds the keyboard while it is up, so moving through it does not
// also move through the conversation behind it.
func TestAPromptHoldsTheKeyboard(t *testing.T) {
	fake := newFakeDaemon()
	current := openAlice(t, fake, aliceMessages())
	was := current.cursor

	current = pump(t, current, press("p"), press("j"), press("j"))
	if current.cursor != was {
		t.Fatalf("the conversation moved to %d behind the prompt", current.cursor)
	}
	if current.chooser.cursor != 3 {
		t.Fatalf("the prompt is on %d", current.chooser.cursor)
	}
	current = pump(t, current, press("esc"))
	if current.chooser != nil {
		t.Fatal("the prompt is still up")
	}
	if asked := fake.calls(""); len(asked) != 0 {
		t.Fatalf("backing out of the prompt asked for %v", asked)
	}
}

// Every key is written down somewhere, because a terminal has nowhere to put
// them all at once.
func TestTheKeysAreListed(t *testing.T) {
	current := driveFake(t, newFakeDaemon(), aliceChat(), press("?"))
	view := current.View()
	for _, wanted := range []string{"answer the message", "delete one for everyone", "quit, always"} {
		if !strings.Contains(view, wanted) {
			t.Fatalf("the keys do not mention %q:\n%s", wanted, view)
		}
	}
}
