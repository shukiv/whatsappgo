package main

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

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
// and a row of the wrong width.
func fitLine(text string, cells int) string {
	if cells < 1 {
		return ""
	}
	return ansi.Truncate(text, cells, "…")
}

// safeText is safeLine for something written as more than one line. A message
// is typed with the line breaks its sender chose and they are part of what was
// said, so the breaks stay and every line is cleaned on its own.
func safeText(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for index, line := range lines {
		lines[index] = safeLine(line)
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}
