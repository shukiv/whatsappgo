package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// emojiGroup is one of the desktop client's emoji groups: its name, the
// picture its tab shows, and its emoji with the words each is found by.
type emojiGroup struct {
	name  string
	icon  string
	items []string
}

// emojiPicker is the emoji panel the face in the message bar opens, laid out
// as the desktop client's is: a row of groups, a search, and the emoji.
type emojiPicker struct {
	group  int // -1 is the ones used lately
	search textinput.Model
	cursor int
	top    int
	// Where the last drawing put things, for the mouse.
	tabsRow, gridRow, gridRows, columns, stickersRow int
}

// recentEmojiKept is how many of the emoji used lately the panel offers
// first. They are kept for as long as the window is open.
const recentEmojiKept = 24

func emojiGlyph(item string) string {
	glyph, _, _ := strings.Cut(item, "|")
	return glyph
}

func emojiWords(item string) string {
	_, words, _ := strings.Cut(item, "|")
	return words
}

// drawableEmoji keeps the emoji the terminal draws in one cell or two. One
// joined from several pictures is drawn by the terminal as all of them, and
// would not fit its place in the grid.
func drawableEmoji(items []string) []string {
	kept := make([]string, 0, len(items))
	for _, item := range items {
		if cells := ansi.StringWidth(unjoined(emojiGlyph(item))); cells >= 1 && cells <= 2 {
			kept = append(kept, item)
		}
	}
	return kept
}

// emojiCell is an emoji in its place in the grid: a space, then two cells,
// whether the emoji takes one of them or both.
func emojiCell(glyph string) string {
	glyph = unjoined(glyph)
	return " " + glyph + strings.Repeat(" ", max(0, 2-ansi.StringWidth(glyph)))
}

// openEmoji opens the panel on the ones used lately, or on the first group
// when none has been used yet.
func (a *app) openEmoji() tea.Cmd {
	if a.open.JID == "" {
		return nil
	}
	search := textinput.New()
	search.CharLimit = 64
	group := 0
	if len(a.recentEmoji) > 0 {
		group = -1
	}
	a.picker = &emojiPicker{group: group, search: search}
	return a.picker.search.Focus()
}

// pickerEntries are the emoji the panel shows: those whose words hold what is
// typed in its search, or else the group's.
func (a *app) pickerEntries() []string {
	query := strings.ToLower(strings.TrimSpace(a.picker.search.Value()))
	var source []string
	switch {
	case query != "":
		for _, group := range emojiGroups {
			source = append(source, group.items...)
		}
	case a.picker.group < 0:
		source = a.recentEmoji
	default:
		source = emojiGroups[a.picker.group].items
	}
	kept := drawableEmoji(source)
	if query == "" {
		return kept
	}
	found := kept[:0:0]
	for _, item := range kept {
		if strings.Contains(strings.ToLower(emojiWords(item)), query) || emojiGlyph(item) == query {
			found = append(found, item)
		}
	}
	return found
}

// pickGroup moves to a group, going round from the last to the ones used
// lately and on to the first.
func (a *app) pickGroup(group int) {
	tabs := len(emojiGroups) + 1
	a.picker.group = ((group+1)%tabs+tabs)%tabs - 1
	a.picker.cursor, a.picker.top = 0, 0
	a.picker.search.SetValue("")
}

func (a *app) pickerKey(msg tea.KeyMsg) tea.Cmd {
	picker, entries := a.picker, a.pickerEntries()
	columns := max(1, picker.columns)
	switch msg.String() {
	case "esc":
		a.picker = nil
	case "enter":
		if picker.cursor < len(entries) {
			return a.putEmoji(entries[picker.cursor])
		}
	case "left":
		picker.cursor = max(0, picker.cursor-1)
	case "right":
		picker.cursor = max(0, min(len(entries)-1, picker.cursor+1))
	case "up":
		if picker.cursor >= columns {
			picker.cursor -= columns
		}
	case "down":
		if picker.cursor+columns < len(entries) {
			picker.cursor += columns
		}
	case "tab":
		a.pickGroup(picker.group + 1)
	case "shift+tab":
		a.pickGroup(picker.group - 1)
	case "ctrl+s":
		a.picker = nil
		return a.stickerPrompt()
	default:
		before := picker.search.Value()
		var moved tea.Cmd
		picker.search, moved = picker.search.Update(msg)
		if picker.search.Value() != before {
			picker.cursor, picker.top = 0, 0
		}
		return moved
	}
	return nil
}

// putEmoji puts an emoji into the message where the cursor is, as it was
// chosen - with the parts the window itself draws without - and goes back to
// the message.
func (a *app) putEmoji(item string) tea.Cmd {
	glyph := emojiGlyph(item)
	value := []rune(a.input.Value())
	at := min(max(a.input.Position(), 0), len(value))
	a.input.SetValue(string(value[:at]) + glyph + string(value[at:]))
	a.input.SetCursor(at + len([]rune(glyph)))
	recent := []string{item}
	for _, used := range a.recentEmoji {
		if used != item && len(recent) < recentEmojiKept {
			recent = append(recent, used)
		}
	}
	a.recentEmoji = recent
	a.picker = nil
	return tea.Batch(a.enterComposer(), a.noteTyping())
}

func (a *app) pickerLines(width, height int) string {
	picker, entries := a.picker, a.pickerEntries()
	tint := lipgloss.NewStyle().Background(colourRowKeys)
	searching := picker.search.Value() != ""

	var tabs strings.Builder
	for group := -1; group < len(emojiGroups); group++ {
		icon := "↺"
		if group >= 0 {
			icon = emojiGroups[group].icon
		}
		cell := emojiCell(icon)
		if group == picker.group && !searching {
			cell = tint.Render(cell)
		}
		tabs.WriteString(cell)
	}
	rows := []string{tabs.String()}
	search := inputRows(picker.search.Value(), picker.search.Position(), max(4, width-2), 1, "Search emoji", true, lipgloss.NewStyle())
	rows = append(rows, "⌕ "+search[0])
	heading := "Used lately"
	switch {
	case searching:
		heading = fmt.Sprintf("%s for %q", plural(len(entries), "emoji", "emoji"), picker.search.Value())
	case picker.group >= 0:
		heading = emojiGroups[picker.group].name
	case len(entries) == 0:
		heading = "Nothing used yet - tab goes through the groups"
	}
	rows = append(rows, styleMuted.Render(fitLine(heading, width)))

	columns := max(1, (width-1)/3)
	shown := max(1, height-len(rows)-2)
	picker.tabsRow, picker.gridRow, picker.columns, picker.gridRows = 0, len(rows), columns, shown
	on := picker.cursor / columns
	if on < picker.top {
		picker.top = on
	}
	if on >= picker.top+shown {
		picker.top = on - shown + 1
	}
	for row := 0; row < shown; row++ {
		var line strings.Builder
		for column := 0; column < columns; column++ {
			at := (picker.top+row)*columns + column
			if at >= len(entries) {
				break
			}
			cell := emojiCell(emojiGlyph(entries[at]))
			if at == picker.cursor {
				cell = tint.Render(cell)
			}
			line.WriteString(cell)
		}
		rows = append(rows, line.String())
	}
	rows = append(rows, "")
	picker.stickersRow = len(rows)
	rows = append(rows, styleSelected.Render(" Emoji ")+"  "+styleMuted.Render("Stickers"))
	return strings.Join(rows, "\n")
}

// pickerMouse is a click on the panel: a group, an emoji, or the stickers.
func (a *app) pickerMouse(msg tea.MouseMsg, row, column int) tea.Cmd {
	picker, entries := a.picker, a.pickerEntries()
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		picker.cursor = max(0, picker.cursor-picker.columns)
		return nil
	case tea.MouseButtonWheelDown:
		picker.cursor = max(0, min(len(entries)-1, picker.cursor+picker.columns))
		return nil
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return nil
		}
	default:
		return nil
	}
	switch {
	case row == picker.tabsRow:
		if group := column/3 - 1; group < len(emojiGroups) {
			a.pickGroup(group)
		}
	case row >= picker.gridRow && row < picker.gridRow+picker.gridRows && column/3 < picker.columns:
		if at := (picker.top+row-picker.gridRow)*picker.columns + column/3; at < len(entries) {
			return a.putEmoji(entries[at])
		}
	case row == picker.stickersRow && column >= len(" Emoji  "):
		a.picker = nil
		return a.stickerPrompt()
	}
	return nil
}
