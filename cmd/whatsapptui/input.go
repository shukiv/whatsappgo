package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// inputRows draws a box being typed in: what is typed, wrapped onto as many
// rows as the box has, each row in the order the terminal shows it, with the
// cursor on the character it stands before. The text input keeps what is
// typed and where the cursor is, and is left to do the typing; what it draws
// itself is in the order the words were typed, which with the terminal told
// to leave the order to the window reads backwards in Hebrew.
//
// Only the rows that hold words come back, at most rows of them, each exactly
// width cells wide on the base style. A box with more words than rows shows
// the rows around the cursor.
func inputRows(value string, cursor, width, rows int, placeholder string, focused bool, base lipgloss.Style) []string {
	width, rows = max(2, width), max(1, rows)
	if value == "" {
		mark := ""
		if focused {
			mark = base.Reverse(true).Render(" ")
		}
		room := width - ansi.StringWidth(mark)
		hint := ansi.Truncate(unjoined(placeholder), room, "…")
		return []string{mark + base.Foreground(colourMuted).Render(hint) + spread(base, room-ansi.StringWidth(hint))}
	}

	clusters := clustersOf(value)
	widths := make([]int, len(clusters))
	at := len(clusters)
	runes := 0
	for i, c := range clusters {
		widths[i] = ansi.StringWidth(unjoined(c.text))
		count := len([]rune(c.text))
		if at == len(clusters) && cursor < runes+count {
			at = i
		}
		runes += count
	}
	if !focused {
		at = -1
	}
	// One cell is kept free on every row for the cursor after the last word.
	lines := wrapClusters(clusters, widths, width-1)
	on := len(lines) - 1
	for row, span := range lines {
		if at >= span[0] && at < span[1] {
			on = row
			break
		}
	}
	first := max(0, on-rows+1)
	rtl := rightToLeft(value)

	shown := make([]string, 0, rows)
	for row := first; row < len(lines) && row < first+rows; row++ {
		start, end := lines[row][0], lines[row][1]
		order, levels := visualOrder(clusters[start:end], rtl)
		var before, under, after strings.Builder
		used := 0
		place := &before
		for _, i := range order {
			text := unjoined(shownAt(clusters[start+i].text, levels[i]))
			used += widths[start+i]
			if start+i == at && widths[start+i] > 0 {
				under.WriteString(text)
				place = &after
				continue
			}
			place.WriteString(text)
		}
		drawn := base.Render(before.String())
		if under.Len() > 0 {
			drawn += base.Reverse(true).Render(under.String())
		}
		drawn += base.Render(after.String())
		// A cursor after the last word sits past its end: on the right of a
		// row that reads from the left, on the left of one that reads from
		// the right.
		if at == len(clusters) && row == len(lines)-1 {
			mark := base.Reverse(true).Render(" ")
			used++
			if rtl {
				drawn = mark + drawn
			} else {
				drawn += mark
			}
		}
		if rtl {
			drawn = spread(base, width-used) + drawn
		} else {
			drawn += spread(base, width-used)
		}
		shown = append(shown, drawn)
	}
	return shown
}

// wrapClusters breaks a line into rows no wider than room: after the last
// space that fits, or inside a word longer than a row. Each row is the span
// of clusters it holds.
func wrapClusters(clusters []cluster, widths []int, room int) [][2]int {
	room = max(1, room)
	var rows [][2]int
	start, used, gap := 0, 0, -1
	for i := range clusters {
		for used+widths[i] > room && i > start {
			end := i
			if gap > start {
				end = gap
			}
			rows = append(rows, [2]int{start, end})
			start, used, gap = end, 0, -1
			for k := start; k < i; k++ {
				used += widths[k]
			}
		}
		used += widths[i]
		if clusters[i].text == " " {
			gap = i + 1
		}
	}
	return append(rows, [2]int{start, len(clusters)})
}
