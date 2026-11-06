package main

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// barRows is the message bar as drawn: its three rows, and the column the
// conversation pane starts at.
func barRows(t *testing.T, current *app) ([]string, int) {
	t.Helper()
	listWidth, paneWidth, _ := current.layout()
	rows := current.composerBar(paneWidth)
	if len(rows) != composerHeight {
		t.Fatalf("the bar is %d rows, not %d", len(rows), composerHeight)
	}
	for _, row := range rows {
		if ansi.StringWidth(row) != paneWidth {
			t.Fatalf("a row of the bar is %d cells in a %d-cell pane: %q", ansi.StringWidth(row), paneWidth, ansi.Strip(row))
		}
	}
	return rows, listWidth + 1
}

// The message box is a bar three rows tall, as the desktop client's is: the
// paperclip and the face on its left, the words beside them, and the arrow
// that sends them once there are any. The window stays the terminal's height.
func TestTheMessageBarIsThreeRowsTall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current := openAlice(t, newFakeDaemon(), aliceMessages())
	if lines := strings.Split(current.View(), "\n"); len(lines) != current.height {
		t.Fatalf("the window is %d rows and was drawn %d", current.height, len(lines))
	}
	rows, _ := barRows(t, current)
	middle := ansi.Strip(rows[1])
	if !strings.HasPrefix(middle, " 📎 🙂 ") || !strings.Contains(middle, "Type a message") || strings.Contains(middle, "➤") {
		t.Fatalf("the bar's middle row is %q", middle)
	}
	current = pump(t, current, press("i"), typed("hi"))
	rows, _ = barRows(t, current)
	if middle := ansi.Strip(rows[1]); !strings.Contains(middle, "hi") || !strings.Contains(middle, "➤") {
		t.Fatalf("with words in it the bar's middle row is %q", middle)
	}
}

// Hebrew typed into a box reads from the right, with the cursor where the
// next letter goes: past the end of the words, on their left. A cursor moved
// back into the word sits on the letter it stands before.
func TestHebrewInABoxReadsFromTheRight(t *testing.T) {
	coloured(t)
	plain := lipgloss.NewStyle()
	end := inputRows("שלום", 4, 10, 1, "", true, plain)[0]
	if ansi.Strip(end) != "     "+" "+"םולש" || strings.Index(end, "\x1b[7m") > strings.Index(end, "ם") {
		t.Fatalf("with the cursor at the end the row is %q", end)
	}
	inside := inputRows("שלום", 1, 10, 1, "", true, plain)[0]
	if ansi.Strip(inside) != "      םולש" || !strings.Contains(inside, "\x1b[7mל") {
		t.Fatalf("with the cursor before the second letter the row is %q", inside)
	}
	english := inputRows("hello", 5, 10, 1, "", true, plain)[0]
	if ansi.Strip(english) != "hello     " || strings.Index(english, "\x1b[7m") < strings.Index(english, "o") {
		t.Fatalf("English does not read from the left with the cursor after it: %q", english)
	}

	// The find box is a box like it.
	current := pump(t, drive(t), press("/"), typed("שלום"))
	if !strings.Contains(ansi.Strip(current.View()), "םולש") {
		t.Fatalf("Hebrew typed into the find box does not read from the right:\n%s", ansi.Strip(current.View()))
	}
}

// Words longer than a row wrap onto the next, and words longer than the box
// show the rows around the cursor.
func TestALongMessageWrapsOntoTheBarsRows(t *testing.T) {
	plain := lipgloss.NewStyle()
	value := "one two three four five six seven eight nine ten"
	rows := inputRows(value, len(value), 12, 3, "", true, plain)
	if len(rows) != 3 {
		t.Fatalf("a long message took %d rows of three", len(rows))
	}
	for _, row := range rows {
		if ansi.StringWidth(row) != 12 {
			t.Fatalf("a row is %d cells, not 12: %q", ansi.StringWidth(row), row)
		}
	}
	if last := ansi.Strip(rows[2]); !strings.Contains(last, "ten") {
		t.Fatalf("the rows shown are not the ones with the cursor: %q", rows)
	}
	if first := ansi.Strip(inputRows(value, 0, 12, 3, "", true, plain)[0]); !strings.HasPrefix(first, "one two") {
		t.Fatalf("with the cursor at the start the first row is %q", first)
	}
}

// The paperclip opens the attach menu, the face the emoji, the arrow sends,
// and the words start typing.
func TestTheBarsButtonsDoWhatTheySay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fake := newFakeDaemon()
	current := openAlice(t, fake, aliceMessages())
	row := rowOf(t, current, "Type a message")
	_, start := barRows(t, current)

	current = pump(t, current, click(start+1, row))
	if current.chooser == nil || current.chooser.title != "Attach" {
		t.Fatal("the paperclip did not open the attach menu")
	}
	current = pump(t, current, press("esc"), click(start+4, row))
	if current.picker == nil {
		t.Fatal("the face did not open the emoji")
	}
	current = pump(t, current, press("esc"), click(start+12, row))
	if !current.composing {
		t.Fatal("a click on the words did not start typing")
	}
	current = pump(t, current, typed("on my way"))
	_, paneWidth, _ := current.layout()
	current = pump(t, current, click(start+paneWidth-3, row))
	if sent := fake.only(t, "message.send"); sent.text("text") != "on my way" {
		t.Fatalf("the arrow sent %v", sent.params)
	}
}

// The attach menu is the desktop client's, item for item, in its order, and
// what the desktop client cannot do yet it says so here too.
func TestTheAttachMenuIsTheDesktopClients(t *testing.T) {
	current := pump(t, openAlice(t, newFakeDaemon(), aliceMessages()), press("a"))
	want := []string{"Document", "Photos and videos", "Camera", "Audio", "Contact", "Poll", "Event", "New sticker"}
	if got := menuLabels(current); !slices.Equal(got, want) {
		t.Fatalf("the attach menu is %v, not %v", got, want)
	}
	for _, item := range []string{"Camera", "Event"} {
		current = pump(t, current, chooseLabel(t, current, item))
		if current.failure != item+" is not supported yet" {
			t.Fatalf("%s said %q", item, current.failure)
		}
		current = pump(t, current, press("a"))
	}
}

// A poll is a question and its options, asked for one at a time, no option
// twice, and then whether each person may choose more than one.
func TestAPollIsAQuestionAndItsOptions(t *testing.T) {
	fake := newFakeDaemon().answer("poll.create", map[string]any{"id": "p1"})
	current := pump(t, openAlice(t, fake, aliceMessages()), press("a"))
	current = pump(t, current, chooseLabel(t, current, "Poll"))
	current = pump(t, current, typed("When?"), press("enter"), typed("Today"), press("enter"))
	current = pump(t, current, typed("Today"), press("enter"))
	if current.failure == "" || current.prompt == nil || current.prompt.title != "Option 2" {
		t.Fatalf("the same option twice was taken (failure %q)", current.failure)
	}
	current = pump(t, current, typed("Tomorrow"), press("enter"), press("enter"))
	current = pump(t, current, chooseLabel(t, current, "Several answers"))
	sent := fake.only(t, "poll.create")
	if sent.text("chat_jid") != alice || sent.text("question") != "When?" ||
		!slices.Equal(sent.texts("options"), []string{"Today", "Tomorrow"}) || sent.params["multiple"] != true {
		t.Fatalf("the poll went as %v", sent.params)
	}
}

// An emoji goes into the message where the cursor is, as the desktop client
// keeps it - a heart with the selector that colours it - and the emoji used
// lately come first the next time.
func TestAnEmojiGoesInWhereTheCursorIs(t *testing.T) {
	fake := newFakeDaemon()
	current := pump(t, openAlice(t, fake, aliceMessages()), press("i"), typed("ab"), tea.KeyMsg{Type: tea.KeyLeft})
	current = pump(t, current, tea.KeyMsg{Type: tea.KeyCtrlG}, typed("red heart"), press("enter"))
	if current.picker != nil || !current.composing || current.input.Value() != "a❤️b" {
		t.Fatalf("the heart did not go in at the cursor: %q", current.input.Value())
	}
	current = pump(t, current, press("enter"))
	if sent := fake.only(t, "message.send"); sent.text("text") != "a❤️b" {
		t.Fatalf("the message went as %q", sent.text("text"))
	}
	current = pump(t, current, tea.KeyMsg{Type: tea.KeyCtrlG})
	if current.picker.group != -1 || len(current.pickerEntries()) != 1 || emojiGlyph(current.pickerEntries()[0]) != "❤️" {
		t.Fatalf("the emoji used lately are not first: group %d %v", current.picker.group, current.pickerEntries())
	}
}

// Every emoji the panel offers takes the same place in its grid, and almost
// all of the desktop client's are offered.
func TestEveryEmojiTakesTheSamePlaceInTheGrid(t *testing.T) {
	offered, all := 0, 0
	for _, group := range emojiGroups {
		all += len(group.items)
		for _, item := range drawableEmoji(group.items) {
			offered++
			if cells := ansi.StringWidth(emojiCell(emojiGlyph(item))); cells != 3 {
				t.Fatalf("%q takes %d cells in the grid, not 3", item, cells)
			}
		}
		if cells := ansi.StringWidth(emojiCell(group.icon)); cells != 3 {
			t.Fatalf("the %s tab takes %d cells, not 3", group.name, cells)
		}
	}
	if all != 421 || offered < 400 {
		t.Fatalf("%d of the desktop client's %d emoji are offered", offered, all)
	}
}
