package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shukiv/whatsappgo/internal/model"
)

func click(x, y int) tea.Msg {
	return tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}

func wheel(x, y int, up bool) tea.Msg {
	button := tea.MouseButtonWheelDown
	if up {
		button = tea.MouseButtonWheelUp
	}
	return tea.MouseMsg{X: x, Y: y, Button: button, Action: tea.MouseActionPress}
}

// rowOf is the row of the window that shows text, for a click to land on.
func rowOf(t *testing.T, current *app, text string) int {
	t.Helper()
	for y, row := range strings.Split(current.View(), "\n") {
		if strings.Contains(row, text) {
			return y
		}
	}
	t.Fatalf("%q is not on screen:\n%s", text, current.View())
	return -1
}

// A click on a conversation in the list opens it, the way enter does.
func TestAClickOnAConversationOpensIt(t *testing.T) {
	bob := "972500000002@s.whatsapp.net"
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{})
	current := driveFake(t, fake, chatsMsg([]model.Chat{
		{JID: alice, Title: "Alice Example"}, {JID: bob, Title: "Bob Example"}}))
	current = pump(t, current, click(2, rowOf(t, current, "Bob Example")))
	if asked := fake.only(t, "messages.list"); asked.text("chat_jid") != bob {
		t.Fatalf("the click opened %v", asked.params)
	}
	if current.open.JID != bob || !current.composing {
		t.Fatalf("Bob's conversation is not open with the keyboard in the message box")
	}
}

func twoMessages(t *testing.T) (*app, int) {
	t.Helper()
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{
		{ID: "m1", ChatJID: alice, SenderJID: alice, Timestamp: noon, Kind: "text", Body: "the first one"},
		{ID: "m2", ChatJID: alice, FromMe: true, Timestamp: noon + 60_000, Kind: "text", Body: "the second one"}}})
	current := openAlice(t, fake, model.MessagePage{})
	listWidth, _, _ := current.layout()
	return current, listWidth + 10
}

// A click on a message puts the keys on it, wherever in its bubble it lands.
func TestAClickOnAMessagePutsTheKeysOnIt(t *testing.T) {
	current, x := twoMessages(t)
	current = pump(t, current, click(x, rowOf(t, current, "the second one")))
	if current.cursor != 1 {
		t.Fatalf("the keys are on message %d, not the one clicked", current.cursor)
	}
	current = pump(t, current, click(x, rowOf(t, current, "the first one")))
	if current.cursor != 0 {
		t.Fatalf("the keys are on message %d, not the one clicked", current.cursor)
	}
}

// The wheel moves through the conversation the way the arrow keys do.
func TestTheWheelMovesThroughTheConversation(t *testing.T) {
	current, x := twoMessages(t)
	current.cursor = 0
	current = pump(t, current, wheel(x, 5, false))
	if current.cursor != 1 {
		t.Fatalf("the wheel down left the keys on message %d", current.cursor)
	}
	current = pump(t, current, wheel(x, 5, true))
	if current.cursor != 0 {
		t.Fatalf("the wheel up left the keys on message %d", current.cursor)
	}
}

// A click on the message box starts typing.
func TestAClickOnTheMessageBoxStartsTyping(t *testing.T) {
	current, x := twoMessages(t)
	current = pump(t, current, click(x, rowOf(t, current, "Type a message")))
	if !current.composing {
		t.Fatal("the click did not put the keyboard in the message box")
	}
}

// A click on an item of a list being offered chooses it.
func TestAClickOnAnItemChoosesIt(t *testing.T) {
	recorder(t)
	link := model.Message{ID: "l1", ChatJID: alice, SenderJID: alice, Timestamp: noon,
		Kind: "text", Body: "look at https://example.com", LinkURL: "https://example.com"}
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{link}})
	current := openAlice(t, fake, model.MessagePage{})
	current = pump(t, current, press("o"))
	listWidth, _, _ := current.layout()
	current = pump(t, current, click(listWidth+4, rowOf(t, current, "fetch the full preview")))
	if refreshed := fake.only(t, "link.preview.refresh"); refreshed.text("message_id") != "l1" {
		t.Fatalf("the preview was asked for as %v", refreshed.params)
	}
	if current.chooser != nil {
		t.Fatal("the list is still up after its item was chosen")
	}
}
