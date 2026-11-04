package main

import (
	"context"
	"strings"
	"testing"

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
