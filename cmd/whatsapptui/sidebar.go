package main

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/shukiv/whatsappgo/internal/model"
)

// minListRows is how much of the list the account's disc leaves at the
// least: below that, the disc gives its rows back.
const minListRows = 6

// The account's disc is braille dots, six columns by three rows. A cell is
// twice as tall as it is wide and holds two dots across and four down, so
// twelve dots each way are as tall on screen as they are wide.
const (
	discWidth  = 6
	discHeight = 3
)

// disc is the circle of dots with the initials in its middle, in the dots'
// own colour; the letters are in the terminal's, so they read on a light
// terminal and a dark one alike. Every part of it, the spaces too, is drawn
// on the base style, so a tinted row stays tinted behind it.
func disc(letters string, colour lipgloss.TerminalColor, base lipgloss.Style) string {
	dots := base.Foreground(colour)
	room := discWidth - 2
	letters = ansi.Truncate(letters, room, "")
	left := (room - ansi.StringWidth(letters)) / 2
	right := room - ansi.StringWidth(letters) - left
	middle := base.Render(strings.Repeat(" ", left)) + base.Bold(true).Render(letters) + base.Render(strings.Repeat(" ", right))
	return strings.Join([]string{
		dots.Render("⢀⣴⣶⣶⣦⡀"),
		dots.Render("⢸") + middle + dots.Render("⡇"),
		dots.Render("⠈⠻⠿⠿⠟⠁"),
	}, "\n")
}

// sidebar is the list with the account above it: its disc, who it is and
// how its connection is, a blank row, then the conversations.
func (a *app) sidebar(width, height int) string {
	top := a.accountRows(width, height)
	if top == 0 {
		return a.chatList(width, height)
	}
	return a.account(width) + "\n\n" + a.chatList(width, height-top)
}

// accountRows is how many rows of the list the account takes, which is none
// when the pane is too narrow for it or too short to give them up.
func (a *app) accountRows(width, height int) int {
	if width < discWidth+4 || height-discHeight-1 < minListRows {
		return 0
	}
	return discHeight + 1
}

// account is the account's disc, with its name and what its connection is
// doing beside it. The disc turns red when the connection is down.
func (a *app) account(width int) string {
	colour, said := lipgloss.TerminalColor(colourDisc), styleMuted
	if a.connectionDown() {
		colour, said = colourDanger, styleDanger
	}
	room := width - discWidth - 2
	beside := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(fitLine(a.accountName(), room)),
		said.Render(fitLine(a.connectionWords(), room)))
	return lipgloss.JoinHorizontal(lipgloss.Top, disc(initials(a.accountName()), colour, lipgloss.NewStyle()), "  ", beside)
}

// chatRowHeight is how many rows of the list one conversation takes: as many
// as its disc, with the name and the time on the first and the last message
// on the second.
const chatRowHeight = discHeight

// minRowWords is the least room a row keeps for its words. A list narrower
// than that and a disc leaves its rows without discs.
const minRowWords = 8

// chatsShown is how many conversations a list this many rows tall has room
// for. It is one at the least, and the list is cut to the rows it has.
func chatsShown(height int) int {
	return max(1, height/chatRowHeight)
}

// chatRow is one conversation as WhatsApp draws it in its list: the disc,
// then the name with the time of the last message, and under them that
// message with how many are waiting at its end. The open conversation is
// tinted the whole height of its row, and the one the keys are on a shade
// further.
func (a *app) chatRow(chat model.Chat, index, width int, now time.Time) []string {
	base := lipgloss.NewStyle()
	switch {
	case index == a.selected && a.focus == paneChats:
		base = base.Background(colourRowKeys)
	case chat.JID == a.open.JID && a.open.JID != "":
		base = base.Background(colourRowOpen)
	}
	title := safeLine(chat.Title)
	if title == "" {
		title = safeLine(shortJID(chat.JID))
	}
	room := width
	var circle []string
	if width >= discWidth+1+minRowWords {
		circle = strings.Split(disc(initials(title), discColour(chat.JID), base), "\n")
		room -= discWidth + 1
	}

	// The time gives way to the name when there is not room for both.
	when, whenStyle := rowTime(chat.LastMessageAt, now), base.Foreground(colourMuted)
	if chat.UnreadCount > 0 {
		whenStyle = base.Foreground(colourBrand)
	}
	if ansi.StringWidth(when)+1+minRowWords > room {
		when = ""
	}
	name := fitLine(title, room-gapBefore(when))
	top := base.Bold(true).Render(name) + spread(base, room-ansi.StringWidth(name)-ansi.StringWidth(when)) + whenStyle.Render(when)

	// How many are waiting must not be the part that falls off the end, so
	// the message gives up the room for it.
	badge := ""
	if chat.UnreadCount > 0 {
		pill := lipgloss.NewStyle().Background(colourBrand).Foreground(colourBadgeText).Bold(true)
		if chat.MutedUntil > now.UnixMilli() {
			pill = pill.Background(colourMuted)
		}
		badge = pill.Render(fmt.Sprintf(" %d ", chat.UnreadCount))
	}
	ticks := chatTicks(chat, base)
	said := fitLine(chatPreview(chat), room-ansi.StringWidth(ticks)-gapBefore(badge))
	under := ticks + base.Foreground(colourMuted).Render(said) +
		spread(base, room-ansi.StringWidth(ticks)-ansi.StringWidth(said)-ansi.StringWidth(badge)) + badge

	lines := []string{top, under, spread(base, room)}
	if circle != nil {
		for at := range lines {
			lines[at] = circle[at] + base.Render(" ") + lines[at]
		}
	}
	return lines
}

// gapBefore is the room something at the end of a row takes, with the space
// that keeps it off the words before it.
func gapBefore(end string) int {
	if end == "" {
		return 0
	}
	return ansi.StringWidth(end) + 1
}

// spread is that many spaces on the row's own background.
func spread(base lipgloss.Style, cells int) string {
	if cells <= 0 {
		return ""
	}
	return base.Render(strings.Repeat(" ", cells))
}

// discColour is a conversation's own colour, picked by its address so it is
// the same one each time the list is drawn.
func discColour(jid string) lipgloss.Color {
	sum := fnv.New32a()
	sum.Write([]byte(jid))
	return discColours[sum.Sum32()%uint32(len(discColours))]
}

// chatPreview is the last message as the desktop client's list says it: a
// voice note as its length, inside a group with who wrote it in front, and a
// conversation with nothing in it yet as such.
func chatPreview(chat model.Chat) string {
	text := safeLine(chat.LastMessagePreview)
	if text == "" {
		return "No messages yet"
	}
	if text == "Audio" && chat.LastMessageDuration > 0 {
		text = "🎤 " + runtimeOf(chat.LastMessageDuration)
	}
	if who := previewSender(chat); who != "" {
		text = who + ": " + text
	}
	return text
}

// previewSender is who wrote the last message of a group, named the way the
// desktop client names them: a name when there is one, a phone number when
// the address is one, and never the opaque number of a hidden address.
func previewSender(chat model.Chat) string {
	if !chat.IsGroup && !strings.HasSuffix(chat.JID, "@g.us") {
		return ""
	}
	if chat.LastMessageFromMe {
		return "You"
	}
	jid := safeLine(chat.LastMessageSenderJID)
	local := shortJID(jid)
	if name := safeLine(chat.LastMessageSenderName); name != "" && name != jid && name != local && name != "+"+local {
		return name
	}
	switch {
	case strings.HasSuffix(jid, "@s.whatsapp.net") && local != "":
		return "+" + local
	case local != "":
		return "Contact · " + local[max(0, len(local)-4):]
	}
	return "Group member"
}

// chatTicks are how far our own last message got, before it: one grey tick
// sent, two delivered, two blue read or played.
func chatTicks(chat model.Chat, base lipgloss.Style) string {
	if !chat.LastMessageFromMe || chat.LastMessageStatus == "" {
		return ""
	}
	switch chat.LastMessageStatus {
	case "read", "played":
		return base.Foreground(colourRead).Render("✓✓") + base.Render(" ")
	case "delivered":
		return base.Foreground(colourMuted).Render("✓✓") + base.Render(" ")
	}
	return base.Foreground(colourMuted).Render("✓") + base.Render(" ")
}

// rowTime is when a conversation last heard anything, as WhatsApp Web says it
// in the list: the time today, "Yesterday", the day's name within the week,
// and the date before that.
func rowTime(milliseconds int64, now time.Time) string {
	if milliseconds <= 0 {
		return ""
	}
	when := time.UnixMilli(milliseconds).In(now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case !when.Before(today):
		return when.Format("15:04")
	case !when.Before(today.AddDate(0, 0, -1)):
		return "Yesterday"
	case !when.Before(today.AddDate(0, 0, -6)):
		return when.Format("Monday")
	}
	return when.Format("02/01/2006")
}

func (a *app) accountName() string {
	if name := safeLine(a.status.UserName); name != "" {
		return name
	}
	return a.profile
}

// connectionDown is whether the account cannot reach WhatsApp from here: the
// daemon is not answering, or says it is not connected.
func (a *app) connectionDown() bool {
	switch a.status.State {
	case "", "connected", "connecting", "pairing":
		return a.statusFailed
	}
	return true
}

func (a *app) connectionWords() string {
	switch {
	case a.statusFailed:
		return "the daemon is not answering"
	case a.status.State == "connected" && (a.waitingHistory || a.loadingOlder):
		return "fetching older messages…"
	}
	switch a.status.State {
	case "connected":
		return "connected"
	case "connecting":
		return "connecting…"
	case "pairing":
		return "waiting for the phone"
	case "logged_out":
		return "not linked to a phone"
	case "":
		return ""
	}
	return "not connected"
}

// initials is up to two letters for a name: the first of each of its first
// two words, or the first two of a name of one word, in the order the
// terminal shows them.
func initials(name string) string {
	var picked []rune
	words := strings.Fields(name)
	for _, word := range words {
		for _, r := range word {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				picked = append(picked, unicode.ToUpper(r))
				break
			}
		}
		if len(picked) == 2 {
			break
		}
	}
	if len(picked) == 1 {
		picked = picked[:0]
		for _, r := range strings.Join(words, "") {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				picked = append(picked, unicode.ToUpper(r))
				if len(picked) == 2 {
					break
				}
			}
		}
	}
	return visual(string(picked))
}
