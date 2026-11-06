package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/shukiv/whatsappgo/internal/model"
)

func connectedAs(name string) statusMsg {
	return statusMsg(status{LoggedIn: true, State: "connected", UserName: name})
}

// The account's initials sit in a disc of braille dots at the top of the
// list, six columns by three rows - as tall as it is wide on a screen whose
// cells are twice as tall as they are wide - with its name and its
// connection beside it. The list keeps its width and starts below.
func TestTheAccountSitsInADiscAboveTheList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	drawn := strings.Split(disc(initials("Dana Levi"), colourDisc, lipgloss.NewStyle()), "\n")
	want := []string{"⢀⣴⣶⣶⣦⡀", "⢸ DL ⡇", "⠈⠻⠿⠿⠟⠁"}
	if len(drawn) != len(want) {
		t.Fatalf("the disc is %d rows, not %d: %q", len(drawn), len(want), drawn)
	}
	for i, row := range drawn {
		if ansi.StringWidth(row) != discWidth || ansi.Strip(row) != want[i] {
			t.Fatalf("row %d of the disc is %q, wanted %q", i, ansi.Strip(row), want[i])
		}
	}

	current := drive(t, connectedAs("Dana Levi"), chatsMsg([]model.Chat{{JID: alice, Title: "Alice Example"}}))
	view := strings.Split(ansi.Strip(current.View()), "\n")
	if len(view) != current.height {
		t.Fatalf("the window is %d rows and was drawn %d", current.height, len(view))
	}
	listWidth, _, _ := current.layout()
	rows := map[string]int{}
	for y, line := range view {
		if w := ansi.StringWidth(line); w > current.width {
			t.Fatalf("row %d is %d columns in a %d-column window: %q", y, w, current.width, line)
		}
		if y == 0 {
			// The title says who the account is too.
			continue
		}
		list := ansi.Truncate(line, listWidth, "")
		for _, text := range []string{"DL", "Dana Levi", "connected", "Alice Example"} {
			if _, seen := rows[text]; !seen && strings.Contains(list, text) {
				rows[text] = y
			}
		}
	}
	if rows["Dana Levi"] != 1 || rows["connected"] != 2 || rows["DL"] != 2 || rows["Alice Example"] != 5 {
		t.Fatalf("the account and the list are not where they belong: %v\n%s", rows, strings.Join(view, "\n"))
	}
}

// A window too short to give up the rows, or a list too narrow for the
// disc, leaves the list as it was.
func TestASmallWindowKeepsItsRowsForTheList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current := drive(t, connectedAs("Dana Levi"), chatsMsg([]model.Chat{{JID: alice, Title: "Alice Example"}}))
	current.height = 10
	view := strings.Split(ansi.Strip(current.View()), "\n")
	if strings.Contains(strings.Join(view[1:], "\n"), "Dana Levi") || !strings.Contains(view[1], "Alice Example") {
		t.Fatalf("a 10-row window gave the list's rows to the account:\n%s", strings.Join(view, "\n"))
	}
	if current.accountRows(9, 40) != 0 {
		t.Fatal("a 9-column list made room for a 6-column disc")
	}
}

// The disc does nothing when clicked, and a click on the list below it
// still opens the conversation it lands on.
func TestAClickOnTheDiscDoesNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fake := newFakeDaemon()
	current := driveFake(t, fake, connectedAs("Dana Levi"), chatsMsg([]model.Chat{{JID: alice, Title: "Alice Example"}}))
	for y := 1; y <= 4; y++ {
		current = pump(t, current, click(2, y))
	}
	if len(fake.calls("messages.list")) != 0 || current.open.JID != "" {
		t.Fatal("a click on the account's disc opened a conversation")
	}
	current = pump(t, current, click(2, rowOf(t, current, "Alice Example")))
	if current.open.JID != alice {
		t.Fatal("a click on the first conversation below the disc did not open it")
	}
}

// The disc turns red, and says why, when the account cannot reach WhatsApp
// from here; it stays as it is while the connection is up or being made.
func TestTheDiscSaysWhenTheConnectionIsDown(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, c := range []struct {
		state  string
		failed bool
		down   bool
		words  string
	}{
		{"connected", false, false, "connected"},
		{"connecting", false, false, "connecting…"},
		{"pairing", false, false, "waiting for the phone"},
		{"", false, false, ""},
		{"logged_out", false, true, "not linked to a phone"},
		{"disconnected", false, true, "not connected"},
		{"", true, true, "the daemon is not answering"},
	} {
		current := newApp(t.Context(), nil, "default")
		current.status.State, current.statusFailed = c.state, c.failed
		if current.connectionDown() != c.down || current.connectionWords() != c.words {
			t.Fatalf("state %q (daemon failed %v): down %v %q, wanted %v %q", c.state, c.failed,
				current.connectionDown(), current.connectionWords(), c.down, c.words)
		}
	}

	// A daemon that cannot say how the account is, is a connection that is
	// down, and the window says so where the reader is looking.
	failed, ok := newApp(t.Context(), nil, "default").loadStatus()().(statusFailedMsg)
	if !ok {
		t.Fatal("a status the daemon could not give is not told apart from other failures")
	}
	current := drive(t, failed)
	if !current.connectionDown() || current.failure == "" || !strings.Contains(current.footer(), current.failure) {
		t.Fatalf("a daemon that does not answer left the disc as it was, or said nothing: %q", current.footer())
	}
}

// Initials are the first letters of the first two words, or the first two
// letters of a single word, in the order the terminal shows them: Hebrew
// turned round, so that it reads right to left.
func TestInitials(t *testing.T) {
	for name, want := range map[string]string{
		"Dana Levi":       "DL",
		"shuki":           "SH",
		"  dana  levi x ": "DL",
		"🙂 Dana":          "DA",
		"שוקי ואקנין":     "וש",
		"":                "",
	} {
		if got := initials(name); got != want {
			t.Fatalf("initials(%q) = %q, wanted %q", name, got, want)
		}
	}
}

// A conversation in the list is drawn as WhatsApp draws it: a disc with its
// initials, its name with the time of its last message on the first row, and
// that message with how many are waiting under them.
func TestAConversationIsADiscWithItsNameTimeAndLastMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	chat := model.Chat{JID: alice, Title: "Dana Levi", LastMessagePreview: "see you there",
		LastMessageAt: time.Date(2026, 11, 5, 9, 41, 0, 0, time.Local).UnixMilli(), UnreadCount: 3}
	current := drive(t)
	lines := current.chatRow(chat, 5, 40, now)
	want := []struct{ start, end string }{
		{"⢀⣴⣶⣶⣦⡀ Dana Levi", "09:41"},
		{"⢸ DL ⡇ see you there", " 3 "},
		{"⠈⠻⠿⠿⠟⠁ ", " "},
	}
	if len(lines) != chatRowHeight {
		t.Fatalf("a conversation is %d rows, not %d: %q", len(lines), chatRowHeight, lines)
	}
	for at, line := range lines {
		plain := ansi.Strip(line)
		if ansi.StringWidth(line) != 40 || !strings.HasPrefix(plain, want[at].start) || !strings.HasSuffix(plain, want[at].end) {
			t.Fatalf("row %d of the conversation is %q (%d columns)", at, plain, ansi.StringWidth(line))
		}
	}

	// A list too narrow for a disc keeps the words and drops the disc.
	for _, line := range current.chatRow(chat, 5, discWidth+minRowWords, now) {
		if strings.ContainsAny(ansi.Strip(line), "⢀⢸⠈") || ansi.StringWidth(line) != discWidth+minRowWords {
			t.Fatalf("a narrow list drew a disc, or a row that is not its width: %q", ansi.Strip(line))
		}
	}
}

// The row the keys are on is tinted the whole of its height and width, the
// disc and the gaps too, and the rows around it are not.
func TestTheRowTheKeysAreOnIsTintedTheWholeOfItsHeight(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	coloured(t)
	tint := regexp.MustCompile(`48;2;\d+;\d+;\d+`).FindString(lipgloss.NewStyle().Background(colourRowKeys).Render(" "))
	if tint == "" {
		t.Fatal("the tint is not written as a colour")
	}
	current := drive(t)
	current.selected, current.focus = 1, paneChats
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	chat := model.Chat{JID: alice, Title: "Alice Example", LastMessagePreview: "hi", LastMessageAt: now.UnixMilli()}
	for _, line := range current.chatRow(chat, 1, 40, now) {
		for _, piece := range strings.Split(line, "\x1b[0m") {
			if piece != "" && !strings.Contains(piece, tint) {
				t.Fatalf("part of the row the keys are on is not tinted: %q in %q", piece, line)
			}
		}
	}
	for _, line := range current.chatRow(chat, 0, 40, now) {
		if strings.Contains(line, tint) {
			t.Fatalf("a row the keys are not on is tinted: %q", line)
		}
	}
}

// A click on any row of a conversation opens it, not only on its name.
func TestAClickAnywhereOnAConversationOpensIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, below := range []int{1, 2} {
		fake := newFakeDaemon()
		current := driveFake(t, fake, twoChats())
		current = pump(t, current, click(2, rowOf(t, current, "Bob Example")+below))
		if current.open.JID != "972500000002@s.whatsapp.net" {
			t.Fatalf("a click %d rows under Bob's name did not open Bob (open: %q)", below, current.open.JID)
		}
	}
}

// The last message says what it is the way the desktop client's list does.
func TestTheLastMessageReadsAsTheDesktopListSaysIt(t *testing.T) {
	group := "120363000000000001@g.us"
	for _, c := range []struct {
		chat model.Chat
		want string
	}{
		{model.Chat{JID: alice}, "No messages yet"},
		{model.Chat{JID: alice, LastMessagePreview: "Audio", LastMessageKind: "audio", LastMessageDuration: 9}, "🎤 0:09"},
		{model.Chat{JID: alice, LastMessagePreview: "hi", LastMessageSenderName: "Alice"}, "hi"},
		{model.Chat{JID: group, IsGroup: true, LastMessagePreview: "build is up", LastMessageSenderName: "Noa",
			LastMessageSenderJID: "972501111111@s.whatsapp.net"}, "Noa: build is up"},
		{model.Chat{JID: group, IsGroup: true, LastMessagePreview: "ok", LastMessageFromMe: true}, "You: ok"},
		{model.Chat{JID: group, IsGroup: true, LastMessagePreview: "hi",
			LastMessageSenderJID: "972501234567@s.whatsapp.net"}, "+972501234567: hi"},
		{model.Chat{JID: group, IsGroup: true, LastMessagePreview: "hi",
			LastMessageSenderJID: "98765432101234@lid", LastMessageSenderName: "98765432101234"}, "Contact · 1234: hi"},
	} {
		if got := chatPreview(c.chat); got != c.want {
			t.Fatalf("the list says %q for %+v, wanted %q", got, c.chat, c.want)
		}
	}
}

// The time beside a conversation is a clock time today, "Yesterday", the
// day's name within the week, and a date before that.
func TestTheRowTimeIsSaidAsWhatsAppWebSaysIt(t *testing.T) {
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	at := func(day, hour int) int64 { return time.Date(2026, 11, day, hour, 30, 0, 0, time.Local).UnixMilli() }
	for when, want := range map[int64]string{
		0:         "",
		at(5, 0):  "00:30",
		at(4, 23): "Yesterday",
		at(4, 0):  "Yesterday",
		at(2, 12): "Monday",
		time.Date(2026, 10, 30, 0, 0, 0, 0, time.Local).UnixMilli():  "Friday",
		time.Date(2026, 10, 29, 23, 0, 0, 0, time.Local).UnixMilli(): "29/10/2026",
	} {
		if got := rowTime(when, now); got != want {
			t.Fatalf("rowTime(%s) = %q, wanted %q", time.UnixMilli(when), got, want)
		}
	}
}
