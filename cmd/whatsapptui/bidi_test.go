package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/shukiv/whatsappgo/internal/model"
)

// A terminal writes what it is given from the left. The window turns Hebrew
// round before it is written, so it reads right to left on screen, while the
// numbers, the Latin words and the emoji inside it keep their own order and
// sit where they belong.
func TestHebrewIsWrittenInTheOrderItReads(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for written, shown := range map[string]string{
		"שלום":                "םולש",
		"שלום world 123":      "world 123 םולש",
		"הלן (מ-21:15)":       "(21:15-מ) ןלה",
		"Hello שלום there":    "Hello םולש there",
		"50% הנחה":            "החנה 50%",
		"משפחה 🤘":             "🤘 החפשמ",
		"https://example.com": "https://example.com",
		// Vowel points stay on their letters.
		"שָׁלוֹם": "ם" + "וֹ" + "ל" + "שָׁ",
	} {
		if got := visual(written); got != shown {
			t.Fatalf("%q is shown as %q, wanted %q", written, got, shown)
		}
		if ansi.StringWidth(visual(written)) != ansi.StringWidth(written) {
			t.Fatalf("turning %q round changed its width", written)
		}
	}
}

// A long Hebrew message is wrapped as it was written - its first words on the
// first row - and every row is turned round on its own and starts at the
// bubble's right edge.
func TestALongHebrewMessageWrapsFromItsFirstWords(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current := drive(t)
	body := strings.Repeat("אחת שתיים שלוש ", 8) + "סוף"
	lines := current.messageLines(model.Message{ID: "w1", SenderJID: alice, Timestamp: noon,
		Kind: "text", Body: body}, false, 40, false)
	var rows []string
	for _, line := range lines {
		if plain := strings.TrimSpace(ansi.Strip(line)); strings.ContainsFunc(plain, isRightToLeft) {
			rows = append(rows, plain)
		}
	}
	if len(rows) < 2 {
		t.Fatalf("a long message was not wrapped:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.HasSuffix(rows[0], visual("אחת")) || !strings.HasPrefix(rows[len(rows)-1], visual("סוף")) {
		t.Fatalf("the message does not start at the right of its first row and end at the left of its last:\n%s",
			strings.Join(rows, "\n"))
	}
}

// A menu keeps its numbers at the left, whichever way an item reads.
func TestAMenuNumberStaysLeftOfAHebrewItem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current := drive(t)
	current.ask("Forward to", "", 0, choice{label: "משפחה"})
	for _, row := range strings.Split(ansi.Strip(current.chooserLines(40, 10)), "\n") {
		if strings.Contains(row, "החפשמ") {
			if !strings.HasPrefix(row, " 1 החפשמ") {
				t.Fatalf("the item's number is not at its left: %q", row)
			}
			return
		}
	}
	t.Fatalf("the Hebrew item is not in the menu, turned round:\n%s", current.chooserLines(40, 10))
}

// The window turns Hebrew round itself, so it asks the terminal not to turn
// it round again for as long as it is open - VTE does unless asked - and
// gives the terminal its own layout back when it closes, even on a failure.
func TestTheTerminalLeavesTheOrderToTheWindowWhileItIsOpen(t *testing.T) {
	var out strings.Builder
	failed := errors.New("the window failed")
	err := layingOutOurselves(&out, func() error {
		if out.String() != "\x1b[8l" {
			t.Fatalf("the window opened before the terminal was told to leave the order alone: %q", out.String())
		}
		return failed
	})
	if !errors.Is(err, failed) || out.String() != "\x1b[8l\x1b[8h" {
		t.Fatalf("the terminal did not get its own layout back (err %v): %q", err, out.String())
	}
}

// An emoji joined from parts is drawn by VTE part by part, so the window
// draws it the same way and counts it as wide as it comes out on screen. The
// widths are the ones VTE 0.80 gives. A message being changed keeps its emoji
// whole, because it goes back to WhatsApp as it is left.
func TestAnEmojiIsCountedAsWideAsTheTerminalDrawsIt(t *testing.T) {
	for written, want := range map[string]struct {
		shown string
		cells int
	}{
		"תודה 🙏🏻": {"תודה 🙏", 7},
		"❤️":      {"❤", 1},
		"👨‍👩‍👧":   {"👨👩👧", 6},
		"🤘🏼 hey":  {"🤘 hey", 6},
	} {
		if got := safeLine(written); got != want.shown || ansi.StringWidth(got) != want.cells {
			t.Fatalf("safeLine(%q) = %q, %d cells; wanted %q, %d", written, got, ansi.StringWidth(got), want.shown, want.cells)
		}
		if got := editableText(written); got != written {
			t.Fatalf("a message being changed lost part of its emoji: %q became %q", written, got)
		}
	}
	if got := fitLine("❤️ ok", 10); got != "❤ ok" {
		t.Fatalf("a line fitted to its room kept a joined emoji: %q", got)
	}

	// A conversation whose name ends in one is exactly as wide as its list.
	current := drive(t)
	chat := model.Chat{JID: alice, Title: "משפחה 🤘🏼", LastMessagePreview: "תודה 🙏🏻"}
	for _, line := range current.chatRow(chat, 0, 40, time.Now()) {
		if ansi.StringWidth(line) != 40 || strings.ContainsRune(line, 0x1f3fc) || strings.ContainsRune(line, 0x1f3fb) {
			t.Fatalf("a row with a skin tone in it is not the list's width: %q", ansi.Strip(line))
		}
	}
}
