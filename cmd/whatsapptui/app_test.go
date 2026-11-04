package main

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/rpc"
)

// The interface is driven entirely by messages, so a test can drive it the way
// the terminal does - without a terminal, a socket or an account - and read
// what the reader would see.
func drive(t *testing.T, messages ...tea.Msg) *app {
	t.Helper()
	current := newApp(context.Background(), nil, "default")
	current.width, current.height = 90, 24
	for _, msg := range messages {
		updated, _ := current.Update(msg)
		next, ok := updated.(*app)
		if !ok {
			t.Fatalf("update returned %T, not the app", updated)
		}
		current = next
	}
	return current
}

func twoChats() chatsMsg {
	return chatsMsg([]model.Chat{
		{JID: "972500000001@s.whatsapp.net", Title: "Alice Example", LastMessagePreview: "see you there"},
		{JID: "972500000002@s.whatsapp.net", Title: "Bob Example", UnreadCount: 3},
	})
}

func TestTheChatListShowsWhatArrived(t *testing.T) {
	view := drive(t, twoChats()).View()
	for _, wanted := range []string{"Alice Example", "Bob Example", "(3)"} {
		if !strings.Contains(view, wanted) {
			t.Fatalf("the window does not show %q:\n%s", wanted, view)
		}
	}
}

// A conversation is opened by moving onto it, and the keyboard must stop at
// both ends of the list rather than running off it.
func TestMovingThroughTheListStopsAtItsEnds(t *testing.T) {
	up := drive(t, twoChats(), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if up.selected != 0 {
		t.Fatalf("moving up from the first row landed on %d", up.selected)
	}
	down := drive(t, twoChats(),
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if down.selected != 1 {
		t.Fatalf("moving down past the last row landed on %d", down.selected)
	}
	chat, ok := down.selectedChat()
	if !ok || chat.Title != "Bob Example" {
		t.Fatalf("the conversation on screen is %q (%v)", chat.Title, ok)
	}
	if !strings.Contains(down.View(), "Bob Example") {
		t.Fatal("the conversation pane does not name the chat the keyboard is on")
	}
}

// Fewer conversations than before must not leave the keyboard pointing past
// the end of the list, which would read a row that is not there.
func TestAShorterListPullsTheSelectionBack(t *testing.T) {
	shortened := drive(t, twoChats(),
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		chatsMsg([]model.Chat{{JID: "972500000001@s.whatsapp.net", Title: "Alice Example"}}))
	if shortened.selected != 0 {
		t.Fatalf("the selection stayed at %d with one conversation left", shortened.selected)
	}
	if _, ok := shortened.selectedChat(); !ok {
		t.Fatal("the selection points at no conversation")
	}
}

// The daemon announces a new message to every client at once. The terminal has
// to ask for the list again and go on listening; a client that stops listening
// after one announcement goes quiet for the rest of the session.
func TestAnAnnouncementIsActedOnAndListeningGoesOn(t *testing.T) {
	current := newApp(context.Background(), nil, "default")
	for _, name := range []string{"message.upsert", "chat.updated", "something.else"} {
		_, cmd := current.Update(eventMsg(rpc.Event{Event: name}))
		if cmd == nil {
			t.Fatalf("%q left the terminal with nothing to do, so it stopped listening", name)
		}
	}
}

// A daemon that refuses says so where the reader is looking, rather than
// leaving an empty window that reads as an account with no conversations.
func TestAFailureIsShown(t *testing.T) {
	refused := drive(t, failureMsg{err: errRefused{}})
	if !strings.Contains(refused.View(), "daemon refused") {
		t.Fatalf("the window does not carry the failure:\n%s", refused.View())
	}
}

type errRefused struct{}

func (errRefused) Error() string { return "daemon refused" }

// Nobody reading this window wrote what is in it. A contact picks their own
// push name and a message carries whatever was typed at the other end, and a
// terminal obeys some of those characters instead of printing them: an escape
// sequence clears the window, retitles it, or asks the terminal a question
// whose answer comes back as if it had been typed. None of it may reach the
// terminal, and what is left must still read as the name.
func TestWhatTheAccountSendsCannotDriveTheTerminal(t *testing.T) {
	nasty := "Alice\x1b[2J\x1b]0;owned\x07\x1b[31m"
	view := drive(t, chatsMsg([]model.Chat{{
		JID:                "972500000001@s.whatsapp.net",
		Title:              nasty,
		LastMessagePreview: "see you\x1b[1;1Hthere\u202eelif.txt",
	}}), failureMsg{err: errRefused{}}).View()
	for _, order := range []string{"\x1b[2J", "\x1b]0;", "\x07", "\x1b[31m", "\x1b[1;1H", "\u202e"} {
		if strings.Contains(view, order) {
			t.Fatalf("the window passed %q through to the terminal:\n%q", order, view)
		}
	}
	if !strings.Contains(view, "Alice") {
		t.Fatalf("cleaning the name removed the name:\n%q", view)
	}
}

// A line that does not fit is cut by the room the terminal gives each
// character, not by bytes. A byte cut shows up two ways: between the bytes of
// one character, which leaves a broken character, and in a row that stops far
// short of the width it was given, because it counted bytes as though each
// were a column.
func TestARowTooLongIsCutByWidthAndNotMidCharacter(t *testing.T) {
	current := drive(t, chatsMsg([]model.Chat{
		// Three bytes and two columns per character, so a byte cut lands
		// inside a character and fills barely two thirds of the row.
		{JID: "972500000001@s.whatsapp.net", Title: strings.Repeat("日本語", 20), UnreadCount: 7},
		{JID: "972500000002@s.whatsapp.net", Title: strings.Repeat("שוקי וקנין ", 12)},
		{JID: "972500000003@s.whatsapp.net", Title: "Bob Example"},
	}))
	list := current.chatList(30)
	if !utf8.ValidString(list) {
		t.Fatalf("a row was cut in the middle of a character: %q", list)
	}
	rows := strings.Split(list, "\n")
	if len(rows) != 3 {
		t.Fatalf("rows = %d, wanted 3: %q", len(rows), list)
	}
	for _, row := range rows {
		if width := ansi.StringWidth(row); width > 28 {
			t.Fatalf("a row is %d columns wide in a 30-column list: %q", width, row)
		}
	}
	// The two overlong names fill the row they were given. A row that stops
	// well short of it was cut by bytes rather than by columns.
	for _, row := range rows[:2] {
		if width := ansi.StringWidth(row); width < 26 {
			t.Fatalf("an overlong name was cut back to %d of 28 columns: %q", width, row)
		}
	}
	// A name that fits is not cut at all.
	if !strings.Contains(rows[2], "Bob Example") || strings.Contains(rows[2], "…") {
		t.Fatalf("a name that fits was cut: %q", rows[2])
	}
	// What is waiting is the part that must survive the cut.
	if !strings.Contains(list, "(7)") {
		t.Fatalf("the unread count fell off the end of the row: %q", list)
	}
}
