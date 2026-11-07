package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

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

// An account has far more conversations than a window has rows. The list shows
// as many as fit, the title and the key help stay on screen, and the
// conversation the keyboard is on is always one of the rows shown.
func TestALongListFitsTheWindow(t *testing.T) {
	chats := make([]model.Chat, 200)
	for i := range chats {
		chats[i] = model.Chat{JID: fmt.Sprintf("97250%07d@s.whatsapp.net", i), Title: fmt.Sprintf("Person %03d", i)}
	}
	current := drive(t, chatsMsg(chats))
	fits := func(view string) {
		t.Helper()
		lines := strings.Split(view, "\n")
		if len(lines) != current.height {
			t.Fatalf("the window is %d rows tall and was drawn %d rows:\n%s", current.height, len(lines), view)
		}
		if !strings.Contains(lines[0], "WhatsAppGo") || !strings.Contains(lines[len(lines)-1], "quit") {
			t.Fatalf("the title or the key help fell off the window:\n%s", view)
		}
	}
	fits(current.View())

	for range 60 {
		current = pump(t, current, press("j"))
	}
	view := current.View()
	fits(view)
	if !strings.Contains(view, "Person 060") || strings.Contains(view, "Person 000") {
		t.Fatalf("the list did not follow the keyboard down:\n%s", view)
	}
	for range 60 {
		current = pump(t, current, press("k"))
	}
	view = current.View()
	fits(view)
	if !strings.Contains(view, "Person 000") {
		t.Fatalf("the list did not follow the keyboard back up:\n%s", view)
	}
}

// A terminal that lays out right-to-left text itself treats a whole row as one
// paragraph. A Hebrew name at the end of a list cell would run on into the
// border and the clock beside it and drag them into its own order, so every
// cell ends with a left-to-right mark before the border.
func TestAHebrewNameStaysInItsOwnColumn(t *testing.T) {
	current := drive(t, chatsMsg([]model.Chat{{JID: "972500000009@s.whatsapp.net", Title: "שוקי וקנין"}}))
	for _, row := range strings.Split(current.View(), "\n") {
		name := strings.Index(row, visual("שוקי וקנין"))
		if name < 0 {
			continue
		}
		mark := strings.Index(row[name:], "‎")
		border := strings.Index(row[name:], "│")
		if mark < 0 || border < 0 || mark > border {
			t.Fatalf("nothing holds the name to its column before the border: %q", row)
		}
		return
	}
	t.Fatalf("the name is not in the list:\n%s", current.View())
}

// A word longer than the room it has, which is what a link usually is, is cut
// to fit rather than left to run past the edge of its pane.
func TestALongLinkStaysInsideThePane(t *testing.T) {
	for _, link := range []string{
		"https://ny-executive-podcast-info.simplecast.com/episodes/4582-yaniv-ron-10-12-26-enterprise-cloud-devops-it-architecture-emily",
		"https://docs.google.com/forms/d/1VFxVu0exAP2-8Qeui0tPcQtxAgIu0X7Sb9PtsT9XllY/" + strings.Repeat("x", 120),
	} {
		fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{
			{ID: "l1", ChatJID: alice, SenderJID: alice, Timestamp: noon, Kind: "text", Body: link}}})
		current := openAlice(t, fake, model.MessagePage{})
		for width := 60; width <= 170; width++ {
			current.width = width
			for _, row := range strings.Split(current.View(), "\n") {
				if got := ansi.StringWidth(row); got > width {
					t.Fatalf("a row is %d columns in a %d-column window: %q", got, width, row)
				}
			}
			if strings.Count(current.View(), "\n")+1 != current.height {
				t.Fatalf("at %d columns the window was drawn %d rows tall, not %d:\n%s",
					width, strings.Count(current.View(), "\n")+1, current.height, current.View())
			}
		}
	}
}

// Messages are bubbles of colour on their own side: everybody else's against
// the left, the account's own against the right, with the clock and the marks
// inside at the bottom right, and a mark beside the one the keys are on.
func TestMessagesAreBubblesOnTheirOwnSide(t *testing.T) {
	coloured(t)
	current := drive(t)
	const width = 60
	theirs := current.messageLines(model.Message{ID: "t1", SenderJID: alice, Timestamp: noon,
		Kind: "text", Body: "hello"}, false, width, false)
	ours := current.messageLines(model.Message{ID: "o1", FromMe: true, Timestamp: noon,
		Kind: "text", Body: "hi", DeliveredAt: noon}, false, width, true)

	if first := ansi.Strip(theirs[0]); !strings.HasPrefix(first, "   hello") {
		t.Fatalf("their words do not start at the left, inside the bubble: %q", first)
	}
	for _, line := range ours {
		if plain := ansi.Strip(line); ansi.StringWidth(plain) != width || !strings.HasSuffix(plain, " ") {
			t.Fatalf("our bubble is not against the right edge: %q", plain)
		}
	}
	for at, line := range ours {
		if row := strings.TrimLeft(ansi.Strip(line), " "); !strings.HasPrefix(row, "▌ ") || (at == 0 && !strings.HasPrefix(row, "▌  hi")) {
			t.Fatalf("the message the keys are on is not marked the whole of its height: %q", row)
		}
	}
	if last := ansi.Strip(ours[len(ours)-1]); !strings.HasSuffix(last, stamp(noon)+" ✓✓ ") {
		t.Fatalf("the clock and the ticks are not inside our bubble at its bottom right: %q", last)
	}
	// Each bubble is filled, every row of it: ours in WhatsApp's light green,
	// theirs in grey.
	for _, line := range ours {
		if !strings.Contains(line, "48;2;217;253;211") {
			t.Fatalf("a row of our bubble is not filled: %q", line)
		}
	}
	for _, line := range theirs {
		if !strings.Contains(line, "48;2;") || strings.Contains(line, "48;2;217;253;211") {
			t.Fatalf("a row of their bubble is not filled grey: %q", line)
		}
	}
}

// coloured makes the window write its colours, as it does in a terminal that
// shows them all, for as long as the test runs. The terminal is a light one
// unless the test says otherwise, whatever the one running the tests is.
func coloured(t *testing.T) {
	t.Helper()
	before, wasDark := lipgloss.ColorProfile(), lipgloss.HasDarkBackground()
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(false)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(before)
		lipgloss.SetHasDarkBackground(wasDark)
	})
}

// In a dark terminal our bubbles are WhatsApp's dark green with light words,
// as the desktop client's dark theme draws them, and theirs a dark grey.
func TestOurBubblesAreTheDarkGreenInADarkTerminal(t *testing.T) {
	coloured(t)
	lipgloss.SetHasDarkBackground(true)
	current := drive(t)
	ours := current.messageLines(model.Message{ID: "o1", FromMe: true, Timestamp: noon,
		Kind: "text", Body: "hi", DeliveredAt: noon}, false, 60, false)
	theirs := current.messageLines(model.Message{ID: "t1", SenderJID: alice, Timestamp: noon,
		Kind: "text", Body: "hello"}, false, 60, false)
	for _, line := range ours {
		if !strings.Contains(line, "48;2;0;92;75") || strings.Contains(line, "48;2;217;253;211") {
			t.Fatalf("a row of our bubble is not the dark green: %q", line)
		}
	}
	if !strings.Contains(ours[0], "38;2;233;237;239") {
		t.Fatalf("our words are not light on the dark green: %q", ours[0])
	}
	for _, line := range theirs {
		if !strings.Contains(line, "48;2;32;44;51") {
			t.Fatalf("a row of their bubble is not the dark grey: %q", line)
		}
	}
}

// Hebrew is written in the order it reads - the terminal writes from the
// left, so the window turns it round - and it starts at the bubble's right
// edge, where somebody reading it looks for the start.
func TestHebrewStartsAtTheRightOfItsBubble(t *testing.T) {
	current := drive(t)
	lines := current.messageLines(model.Message{ID: "h1", SenderJID: alice, Timestamp: noon,
		Kind: "text", Body: "שלום", Edited: true, Starred: true}, false, 60, false)
	for _, line := range lines {
		if plain := ansi.Strip(line); strings.Contains(plain, "םולש") {
			if !strings.HasSuffix(plain, "םולש ") {
				t.Fatalf("the Hebrew line does not end against the bubble's right side: %q", plain)
			}
			return
		}
	}
	t.Fatalf("the words are not in the bubble, turned round to read right to left:\n%s", strings.Join(lines, "\n"))
}

// Inside a group each bubble says who wrote it. With one other person the side
// already says so.
func TestOnlyAGroupBubbleSaysWhoWroteIt(t *testing.T) {
	current := drive(t)
	message := model.Message{ID: "g1", SenderJID: alice, SenderName: "Bob Example", Timestamp: noon,
		Kind: "text", Body: "hello"}
	if group := ansi.Strip(strings.Join(current.messageLines(message, true, 60, false), "\n")); !strings.Contains(group, "Bob Example") {
		t.Fatalf("a group bubble does not say who wrote it:\n%s", group)
	}
	if alone := ansi.Strip(strings.Join(current.messageLines(message, false, 60, false), "\n")); strings.Contains(alone, "Bob Example") {
		t.Fatalf("a bubble from the one other person names them:\n%s", alone)
	}
}

// A conversation that runs over more than one day says which day each part of
// it was on, and the window still holds the whole of the bubble the keys are on.
func TestAConversationSaysWhichDayAndHoldsTheWholeBubble(t *testing.T) {
	const day = 24 * 60 * 60 * 1000
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{
		{ID: "d1", ChatJID: alice, SenderJID: alice, Timestamp: noon, Kind: "text", Body: "yesterday"},
		{ID: "d2", ChatJID: alice, FromMe: true, Timestamp: noon + day, Kind: "text", Body: "today"}}})
	current := openAlice(t, fake, model.MessagePage{})
	view := current.View()
	for _, when := range []int64{noon, noon + day} {
		if name := time.UnixMilli(when).Format("Monday, 2 January 2006"); !strings.Contains(view, name) {
			t.Fatalf("the window does not say %q:\n%s", name, view)
		}
	}
	lines, from, to := current.conversationLines(current.width / 2)
	if !strings.Contains(ansi.Strip(lines[from]), "▌") || !strings.Contains(ansi.Strip(lines[to]), stamp(noon+day)) {
		t.Fatalf("the rows of the message the keys are on are %d to %d, not its whole bubble:\n%s",
			from, to, strings.Join(lines, "\n"))
	}
}

func twoChats() chatsMsg {
	return chatsMsg([]model.Chat{
		{JID: "972500000001@s.whatsapp.net", Title: "Alice Example", LastMessagePreview: "see you there"},
		{JID: "972500000002@s.whatsapp.net", Title: "Bob Example", UnreadCount: 3},
	})
}

func TestTheChatListShowsWhatArrived(t *testing.T) {
	view := drive(t, twoChats()).View()
	for _, wanted := range []string{"Alice Example", "see you there", "Bob Example", " 3 "} {
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
	list := current.chatList(30, 10)
	if !utf8.ValidString(list) {
		t.Fatalf("a row was cut in the middle of a character: %q", list)
	}
	rows := strings.Split(list, "\n")
	if len(rows) != 3*chatRowHeight {
		t.Fatalf("rows = %d, wanted %d: %q", len(rows), 3*chatRowHeight, list)
	}
	for _, row := range rows {
		if width := ansi.StringWidth(row); width != 30 {
			t.Fatalf("a row is %d columns wide in a 30-column list: %q", width, row)
		}
	}
	// The two overlong names fill the room beside the disc. A name that
	// stops well short of it was cut by bytes rather than by columns.
	for _, row := range []string{rows[0], rows[chatRowHeight]} {
		name := strings.TrimRight(ansi.Strip(row), " ")
		if width := ansi.StringWidth(name); width < 29 || !strings.Contains(name, "…") {
			t.Fatalf("an overlong name was cut back to %d of 30 columns: %q", width, name)
		}
	}
	// A name that fits is not cut at all.
	if bob := ansi.Strip(rows[2*chatRowHeight]); !strings.Contains(bob, "Bob Example") || strings.Contains(bob, "…") {
		t.Fatalf("a name that fits was cut: %q", bob)
	}
	// What is waiting is the part that must survive the cut.
	if !strings.HasSuffix(ansi.Strip(rows[1]), " 7 ") {
		t.Fatalf("the unread count fell off the end of the row: %q", ansi.Strip(rows[1]))
	}
}
