package main

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// lrm is the left-to-right mark. It takes no room on screen. A terminal that
// lays out right-to-left text itself, and does not take the request to leave
// that to the window (terminalLeavesOrder), treats a whole row as one
// paragraph, so a
// Hebrew name at the end of the list column runs on into the spaces, the
// border and the clock beside it and drags them into its own order. A mark at
// each end of every cell holds each column to its own side of the row.
const lrm = "‎"

// fitted cuts every line of a block to the room its pane has. A pane given a
// line wider than itself wraps it onto a row of its own, the frame comes out
// taller than the terminal, and the terminal drops the top of the window.
func fitted(block string, width int) string {
	lines := strings.Split(block, "\n")
	for at, line := range lines {
		lines[at] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

// anchored puts the mark at both ends of every line of a block.
func anchored(block string) string {
	lines := strings.Split(block, "\n")
	for at, line := range lines {
		lines[at] = lrm + line + lrm
	}
	return strings.Join(lines, "\n")
}

// safeLine makes a string that came from the account safe to write to a
// terminal, and fit to live on one row.
//
// Nothing arriving over the socket was written by the person reading it. A
// contact picks their own push name, a group picks its own subject, and a
// message carries whatever was typed at the other end, so all of it reaches
// here as characters somebody else chose. A terminal reads some of those
// characters as orders rather than text: an escape sequence moves the cursor,
// repaints the window, retitles it, or asks the terminal a question whose
// answer arrives back on standard input as though it had been typed. A name is
// a name, so every control character goes, and the ones that merely separate
// words become a single space to keep the row on one line.
//
// The right-to-left overrides and isolates go too. They are not orders, but
// they reorder what follows them, which is how a name is made to read as
// something it is not - and the window already lays out Hebrew and Arabic by
// the characters themselves.
func safeLine(text string) string {
	return unjoined(cleanLine(text))
}

// cleanLine is safeLine for words that go back to WhatsApp rather than onto
// the screen, such as a message being changed: it keeps the emoji whole.
func cleanLine(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	gap := false
	for _, char := range text {
		switch {
		case unicode.IsSpace(char):
			gap = true
			continue
		case char < 0x20, char == 0x7f, char >= 0x80 && char <= 0x9f:
			continue
		case char >= 0x202a && char <= 0x202e, char >= 0x2066 && char <= 0x2069:
			continue
		case char == 0xfffd:
			// What a byte that is not UTF-8 at all reads as.
			continue
		}
		if gap && out.Len() > 0 {
			out.WriteByte(' ')
		}
		gap = false
		out.WriteRune(char)
	}
	return out.String()
}

// fitLine cuts a line to the room there is for it, counted in the cells the
// terminal gives each character rather than in bytes: a Hebrew letter or an
// emoji is several bytes, and cutting between them leaves a broken character
// and a row of the wrong width. What is left is put in the order the terminal
// shows it, so the cut falls at the end of what was written, in whichever
// direction it was written.
func fitLine(text string, cells int) string {
	if cells < 1 {
		return ""
	}
	return visual(ansi.Truncate(unjoined(text), cells, "…"))
}

// unjoined takes out the parts of an emoji that a terminal draws on their
// own. The window counts a joined emoji as the one picture it stands for, and
// VTE draws its parts: a skin tone as a picture two cells wide of its own, a
// family as the faces it is joined from, and a heart asked to be coloured as
// the one-cell heart it is without asking. A row counted narrower than it is
// drawn wraps, and every row under it moves down a row, so the window draws
// these as the terminal does: the skin tone, the joiner and the selector go.
func unjoined(text string) string {
	return strings.Map(func(char rune) rune {
		switch {
		case char == 0x200d, char == 0xfe0e, char == 0xfe0f, char >= 0x1f3fb && char <= 0x1f3ff:
			return -1
		}
		return char
	}, text)
}

// safeText is safeLine for something written as more than one line. A message
// is typed with the line breaks its sender chose and they are part of what was
// said, so the breaks stay and every line is cleaned on its own.
func safeText(text string) string {
	return eachLine(text, safeLine)
}

// editableText is safeText for a message being changed, which goes back to
// WhatsApp as it is left: its emoji stay whole.
func editableText(text string) string {
	return eachLine(text, cleanLine)
}

func eachLine(text string, clean func(string) string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for index, line := range lines {
		lines[index] = clean(line)
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}
