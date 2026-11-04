package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shukiv/whatsappgo/internal/model"
)

func typed(text string) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)}
}

// Opening a conversation leaves the keyboard in the message box, which is
// where the desktop client leaves it: clicking a conversation in the sidebar is
// usually the start of answering it.
func TestOpeningAConversationLeavesTheKeyboardInTheMessageBox(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())
	current := driveFake(t, fake, aliceChat(), press("enter"), typed("hello"))
	if !current.composing {
		t.Fatal("the keyboard is not in the message box")
	}
	if current.input.Value() != "hello" {
		t.Fatalf("the box holds %q, so the letters went to the keys instead", current.input.Value())
	}
	if !strings.Contains(current.View(), "hello") {
		t.Fatalf("what is being typed is not on screen:\n%s", current.View())
	}
}

// Enter sends what was typed, and the message is on screen without waiting for
// the daemon to announce it.
func TestEnterSendsWhatWasTyped(t *testing.T) {
	sent := model.Message{ID: "m9", ChatJID: alice, FromMe: true, Timestamp: noon + 300_000,
		Kind: "text", Body: "on my way", Status: "pending"}
	fake := newFakeDaemon().answer("messages.list", aliceMessages()).answer("message.send", sent)

	current := driveFake(t, fake, aliceChat(), press("enter"), typed("on my way"), press("enter"))

	asked := fake.only(t, "message.send")
	if asked.text("chat_jid") != alice || asked.text("text") != "on my way" || asked.text("reply_to") != "" {
		t.Fatalf("the message went as %v", asked.params)
	}
	if current.input.Value() != "" {
		t.Fatalf("the box still holds %q after sending", current.input.Value())
	}
	if !strings.Contains(current.View(), "on my way") {
		t.Fatalf("the sent message is not in the conversation:\n%s", current.View())
	}
	// Still ready for the next one, as the desktop client is.
	if !current.composing {
		t.Fatal("sending took the keyboard out of the message box")
	}
}

// An answer carries the message it answers, and says which one that is before
// it goes.
func TestAnAnswerCarriesTheMessageItAnswers(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages()).
		answer("message.send", model.Message{ID: "m9", ChatJID: alice, FromMe: true, Kind: "text", Body: "outside now"})

	// Back to the conversation, onto the first message, and answer that one.
	current := driveFake(t, fake, aliceChat(), press("enter"), press("esc"),
		press("g"), press("r"))
	if current.replyTo != "m1" {
		t.Fatalf("the answer is to %q, wanted the message the keyboard was on", current.replyTo)
	}
	if !strings.Contains(current.View(), "answering: Alice Example: see you there") {
		t.Fatalf("the box does not say what is being answered:\n%s", current.View())
	}
	current = pump(t, current, typed("outside now"), press("enter"))
	asked := fake.only(t, "message.send")
	if asked.text("reply_to") != "m1" || asked.text("text") != "outside now" {
		t.Fatalf("the answer went as %v", asked.params)
	}
	if current.replyTo != "" {
		t.Fatal("the box is still answering a message after sending")
	}
}

// Only a message sent from here can be changed, and changing one is a
// different request from sending a new one.
func TestOnlyOurOwnMessageCanBeChanged(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())

	// m1 is Alice's.
	current := driveFake(t, fake, aliceChat(), press("enter"), press("esc"), press("g"), press("e"))
	if current.editing != "" || current.composing {
		t.Fatalf("somebody else's message was opened for changing: %q", current.editing)
	}

	// m2 is ours.
	current = pump(t, current, press("j"), press("e"))
	if current.editing != "m2" {
		t.Fatalf("the change is to %q, wanted our own message", current.editing)
	}
	if current.input.Value() != "on my way" {
		t.Fatalf("the box holds %q, wanted the message as it stands", current.input.Value())
	}
	current = pump(t, current, typed(" shortly"), press("enter"))

	asked := fake.only(t, "message.edit")
	if asked.text("message_id") != "m2" || asked.text("text") != "on my way shortly" {
		t.Fatalf("the change went as %v", asked.params)
	}
	// A change is not a new message.
	if sends := fake.calls("message.send"); len(sends) != 0 {
		t.Fatalf("changing a message also sent one: %v", sends)
	}
	if current.editing != "" {
		t.Fatal("the box is still changing a message after sending")
	}
}

// The other person is told that somebody is typing, but only after the typing
// settles - the desktop client waits the same 700ms - and it is taken back when
// the message goes.
func TestTypingIsReportedAfterThePauseAndTakenBackOnSend(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages()).
		answer("message.send", model.Message{ID: "m9", ChatJID: alice, FromMe: true, Kind: "text", Body: "hi"})
	current := driveFake(t, fake, aliceChat(), press("enter"), typed("hi"))
	if said := fake.calls("chat.typing"); len(said) != 0 {
		t.Fatalf("a single keystroke was reported as typing: %v", said)
	}

	// The pause the last keystroke started.
	current = pump(t, current, typingMsg(current.typingToken))
	said := fake.only(t, "chat.typing")
	if said.text("chat_jid") != alice || said.params["typing"] != true {
		t.Fatalf("typing was reported as %v", said.params)
	}

	// A pause that an older keystroke started says nothing a second time.
	fake.forget()
	current = pump(t, current, typingMsg(current.typingToken-1), typingMsg(current.typingToken))
	if again := fake.calls("chat.typing"); len(again) != 0 {
		t.Fatalf("typing was reported twice over: %v", again)
	}

	current = pump(t, current, press("enter"))
	stopped := fake.only(t, "chat.typing")
	if stopped.params["typing"] != false {
		t.Fatalf("sending did not take back what was said about typing: %v", stopped.params)
	}
}

// A conversation left behind is told that nobody is typing in it any more,
// before the next one is opened. The desktop client does the same on the way
// out of a chat.
func TestLeavingAConversationTakesBackWhatWasSaidAboutTyping(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())
	current := driveFake(t, fake,
		chatsMsg([]model.Chat{{JID: alice, Title: "Alice Example"},
			{JID: "972500000002@s.whatsapp.net", Title: "Bob Example"}}),
		press("enter"), typed("hi"))
	current = pump(t, current, typingMsg(current.typingToken))
	fake.forget()

	current = pump(t, current, press("tab"), press("j"), press("enter"))
	stopped := fake.only(t, "chat.typing")
	if stopped.text("chat_jid") != alice || stopped.params["typing"] != false {
		t.Fatalf("the conversation being left was told %v", stopped.params)
	}
	// And nothing half-written follows the reader into the next conversation.
	if current.input.Value() != "" {
		t.Fatalf("the box still holds %q in the next conversation", current.input.Value())
	}
}

// Opening a conversation takes back what was said about typing in the one
// being left, wherever the reader came from. Today every route out of the
// message box already does that, so this holds the rule at the place that owns
// it rather than at the one key that happens to reach it.
func TestOpeningAConversationTakesBackTypingInTheLastOne(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())
	current := driveFake(t, fake, aliceChat(), press("enter"))
	fake.forget()
	// Typing had been reported in the conversation that is open.
	current.typing = true

	pump(t, current, do(current.openChat(model.Chat{JID: "972500000002@s.whatsapp.net", Title: "Bob Example"})))
	stopped := fake.only(t, "chat.typing")
	if stopped.text("chat_jid") != alice || stopped.params["typing"] != false {
		t.Fatalf("the conversation being left was told %v", stopped.params)
	}
}

// A channel has nobody to tell that somebody is typing, and the address is not
// one that takes it.
func TestNobodyIsToldAboutTypingInAChannel(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{
		{ID: "n1", ChatJID: channel, Timestamp: noon, Kind: "text", Body: "a post"},
	}})
	current := driveFake(t, fake, chatsMsg([]model.Chat{{JID: channel, Title: "A Channel"}}),
		press("enter"), typed("hello"))
	pump(t, current, typingMsg(current.typingToken))
	if said := fake.calls("chat.typing"); len(said) != 0 {
		t.Fatalf("a channel was told about typing: %v", said)
	}
}

// An empty box is not a message, and neither is a boxful of spaces.
func TestAnEmptyMessageIsNotSent(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())
	driveFake(t, fake, aliceChat(), press("enter"), press("enter"), typed("   "), press("enter"))
	if sends := fake.calls("message.send"); len(sends) != 0 {
		t.Fatalf("an empty message was sent: %v", sends)
	}
}

// While the box has the keyboard, q is a letter. Quitting on it would end the
// session in the middle of a word.
func TestQIsALetterWhileWriting(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())
	current := driveFake(t, fake, aliceChat(), press("enter"))

	_, next := current.Update(typed("q"))
	if next != nil {
		if _, quitting := next().(tea.QuitMsg); quitting {
			t.Fatal("typing a q quit the program")
		}
	}
	if current.input.Value() != "q" {
		t.Fatalf("the box holds %q", current.input.Value())
	}

	// Out of the box, it quits again.
	current = pump(t, current, press("esc"), press("h"))
	_, next = current.Update(press("q"))
	if next == nil {
		t.Fatal("q in the conversation list did nothing")
	}
	if _, quitting := next().(tea.QuitMsg); !quitting {
		t.Fatal("q in the conversation list did not quit")
	}
}
