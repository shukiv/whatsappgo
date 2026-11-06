package main

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/bidi"
)

// The window lays right-to-left text out itself, and asks the terminal not
// to. VTE, the terminal inside GNOME's and most other GTK terminals, turns
// Hebrew and Arabic round by itself in its implicit mode, a row at a time and
// as though every row read left to right: a Hebrew sentence with an English
// word or a number in it comes out in the wrong order, and a line the window
// has already turned round is turned back. In its explicit mode it writes
// each character where it is told. Terminals that do no reordering of their
// own ignore both requests.
const (
	terminalLeavesOrder = "\x1b[8l"
	terminalReorders    = "\x1b[8h"
)

// layingOutOurselves runs the window with the terminal in its explicit mode,
// and gives the terminal its own layout back afterwards, however the window
// ended.
func layingOutOurselves(out io.Writer, window func() error) error {
	fmt.Fprint(out, terminalLeavesOrder)
	defer fmt.Fprint(out, terminalReorders)
	return window()
}

// visual puts a line of text in the order the terminal shows it. With the
// terminal told to leave the order alone, it writes what it is given from the
// left, one character after another, so a line of Hebrew or Arabic arrives
// back to front unless it is turned round here first. The line reads in the
// direction of its first letter, as the desktop client lays it out.
func visual(line string) string {
	return visualIn(line, rightToLeft(line))
}

// visualIn is visual for a line whose direction is already known - a line of
// a paragraph that was wrapped, whose direction is the paragraph's.
//
// This is the Unicode bidirectional algorithm, cut down to what a line from
// the account can hold: safeLine has already taken out the embeddings,
// overrides and isolates, so every line is one level of words and the runs
// of the other direction inside it. Numbers and Latin words inside Hebrew
// stay the right way round, and brackets face the way they read.
func visualIn(line string, rtl bool) string {
	if !strings.ContainsFunc(line, isRightToLeft) {
		return line
	}
	clusters := clustersOf(line)
	order, levels := visualOrder(clusters, rtl)
	var out strings.Builder
	out.Grow(len(line))
	for _, i := range order {
		out.WriteString(shownAt(clusters[i].text, levels[i]))
	}
	return out.String()
}

// shownAt is a cluster as it is drawn at its level: a bracket in a
// right-to-left run faces the other way (L4).
func shownAt(text string, level int) string {
	if level%2 == 1 {
		if mirror, ok := mirrored[text]; ok {
			return mirror
		}
	}
	return text
}

// visualOrder is the order a line's clusters are drawn in from the left, and
// the level each one sits at. A box being typed in needs it as well as the
// text, to put the cursor on the character it is on.
func visualOrder(clusters []cluster, rtl bool) (order, levels []int) {
	n := len(clusters)
	base, level := bidi.L, 0
	if rtl {
		base, level = bidi.R, 1
	}
	types := make([]bidi.Class, n)
	for i, c := range clusters {
		types[i] = c.class
	}

	// W2, W3: a number after Arabic letters is an Arabic number, and Arabic
	// letters are right to left like any other.
	last := base
	for i, t := range types {
		switch t {
		case bidi.L, bidi.R:
			last = t
		case bidi.AL:
			last = bidi.AL
			types[i] = bidi.R
		case bidi.EN:
			if last == bidi.AL {
				types[i] = bidi.AN
			}
		}
	}
	// W4: one separator between two numbers of a kind is part of them, as
	// in 21:15 and 1,000.
	for i := 1; i+1 < n; i++ {
		before, after := types[i-1], types[i+1]
		switch {
		case before == bidi.EN && after == bidi.EN && (types[i] == bidi.ES || types[i] == bidi.CS):
			types[i] = bidi.EN
		case before == bidi.AN && after == bidi.AN && types[i] == bidi.CS:
			types[i] = bidi.AN
		}
	}
	// W5: a currency sign or a percent next to a number belongs to it.
	for i := 0; i < n; {
		if types[i] != bidi.ET {
			i++
			continue
		}
		j := i
		for j < n && types[j] == bidi.ET {
			j++
		}
		if (i > 0 && types[i-1] == bidi.EN) || (j < n && types[j] == bidi.EN) {
			for k := i; k < j; k++ {
				types[k] = bidi.EN
			}
		}
		i = j
	}
	// W6, W7: any other separator is neutral, and a number after a Latin
	// word is part of the Latin.
	last = base
	for i, t := range types {
		switch t {
		case bidi.ES, bidi.ET, bidi.CS:
			types[i] = bidi.ON
		case bidi.L, bidi.R:
			last = t
		case bidi.EN:
			if last == bidi.L {
				types[i] = bidi.L
			}
		}
	}
	// N1, N2: spaces and punctuation between two runs of one direction take
	// that direction, and between two directions the line's own.
	strong := func(t bidi.Class) (bidi.Class, bool) {
		switch t {
		case bidi.L:
			return bidi.L, true
		case bidi.R, bidi.EN, bidi.AN:
			return bidi.R, true
		}
		return 0, false
	}
	for i := 0; i < n; {
		if _, ok := strong(types[i]); ok {
			i++
			continue
		}
		j := i
		for j < n {
			if _, ok := strong(types[j]); ok {
				break
			}
			j++
		}
		before, after := base, base
		if i > 0 {
			before, _ = strong(types[i-1])
		}
		if j < n {
			after, _ = strong(types[j])
		}
		direction := base
		if before == after {
			direction = before
		}
		for k := i; k < j; k++ {
			types[k] = direction
		}
		i = j
	}
	// I1, I2: the level of each character.
	levels = make([]int, n)
	for i, t := range types {
		switch {
		case level == 0 && t == bidi.R:
			levels[i] = 1
		case level == 0 && (t == bidi.EN || t == bidi.AN), level == 1 && t != bidi.R:
			levels[i] = 2
		default:
			levels[i] = level
		}
	}
	// L1: spaces at the end of the line are the line's own.
	for i := n - 1; i >= 0 && strings.TrimSpace(clusters[i].text) == ""; i-- {
		levels[i] = level
	}
	// L2: from the highest level down to the lowest odd one, every run at
	// that level or above is turned round.
	order = make([]int, n)
	highest := 0
	for i := range order {
		order[i] = i
		highest = max(highest, levels[i])
	}
	for at := highest; at >= 1; at-- {
		for i := 0; i < n; {
			if levels[order[i]] < at {
				i++
				continue
			}
			j := i
			for j < n && levels[order[j]] >= at {
				j++
			}
			for a, b := i, j-1; a < b; a, b = a+1, b-1 {
				order[a], order[b] = order[b], order[a]
			}
			i = j
		}
	}
	return order, levels
}

var mirrored = map[string]string{
	"(": ")", ")": "(", "[": "]", "]": "[", "{": "}", "}": "{",
	"<": ">", ">": "<", "«": "»", "»": "«", "‹": "›", "›": "‹",
}

func isRightToLeft(r rune) bool {
	properties, _ := bidi.LookupRune(r)
	return properties.Class() == bidi.R || properties.Class() == bidi.AL
}

// cluster is what is turned round as one: a letter with the vowel points and
// marks written on it, or an emoji with the joiners and modifiers that make it
// one picture. Turning those round one rune at a time would put the marks
// before their letters.
type cluster struct {
	text  string
	class bidi.Class
}

func clustersOf(line string) []cluster {
	var clusters []cluster
	joined := false
	for _, r := range line {
		attached := unicode.In(r, unicode.Mn, unicode.Me) || r == 0x200d ||
			(r >= 0xfe00 && r <= 0xfe0f) || (r >= 0x1f3fb && r <= 0x1f3ff)
		if len(clusters) > 0 && (attached || joined) {
			clusters[len(clusters)-1].text += string(r)
			joined = r == 0x200d
			continue
		}
		properties, _ := bidi.LookupRune(r)
		clusters = append(clusters, cluster{text: string(r), class: properties.Class()})
		joined = false
	}
	return clusters
}
