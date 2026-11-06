package main

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/shukiv/whatsappgo/internal/model"
)

// How a page of messages arrived, which decides what is done with it.
type pageMode int

const (
	// The first page of a conversation somebody just opened.
	pageOpen pageMode = iota
	// A page from before the oldest one on screen.
	pageOlder
	// The conversation read again from the top, after the daemon collected
	// history for it, keeping the reader's place in what they had loaded.
	pageReset
)

// One page of messages is 50, which is what the desktop client asks for, and
// the daemon serves at most 200 in one answer.
const (
	pageSize = 50
	pageMost = 200
)

// Messages the conversation reacts to.
type (
	pageMsg struct {
		generation int
		chat       string
		page       model.MessagePage
		mode       pageMode
	}
	oneMessageMsg struct {
		generation int
		chat       string
		message    model.Message
	}
)

// openChat puts a conversation on screen, in the order the desktop client
// does it: forget the last one, ask for its newest page, and tell the people
// in it that their messages have been seen once that page is in.
//
// A conversation is opened on purpose, with a key, and not by moving the
// cursor onto its row. Opening one sends read receipts, which reach the other
// person's phone, and running the cursor down a list of two hundred
// conversations must not tell two hundred people that their messages were
// read.
func (a *app) openChat(chat model.Chat) tea.Cmd {
	// Whatever was said about typing belonged to the conversation being left,
	// and is taken back there before this one is opened.
	stopped := a.reportTyping(false)
	a.generation++
	a.open = chat
	a.messages = nil
	a.hasMore, a.nextBefore, a.nextBeforeID = false, 0, ""
	a.unreadFrom, a.unreadCount = "", 0
	a.cursor, a.top = 0, 0
	a.loadingOlder, a.waitingHistory = false, false
	// A half-written answer belonged to the other conversation too.
	a.replyTo, a.editing = "", ""
	a.input.SetValue("")
	a.focus = paneConversation
	return tea.Batch(stopped, a.loadPage(chat.JID, a.generation, 0, "", pageSize, pageOpen))
}

// loadPage asks for one page of a conversation. The generation it was asked in
// comes back with it: a reader who has moved on to another conversation, or
// reopened this one, must not have a page from the old one dropped into the
// new one.
func (a *app) loadPage(chat string, generation int, before int64, beforeID string, limit int, mode pageMode) tea.Cmd {
	params := map[string]any{"chat_jid": chat, "before": before, "limit": limit}
	if beforeID != "" {
		params["before_id"] = beforeID
	}
	return func() tea.Msg {
		var page model.MessagePage
		if err := a.daemon.call(a.ctx, "messages.list", params, &page); err != nil {
			return failureMsg{err}
		}
		return pageMsg{generation: generation, chat: chat, page: page, mode: mode}
	}
}

// loadOlder is what reaching the top of the conversation asks for. Once the
// daemon has no older page stored, the account is asked for one instead.
func (a *app) loadOlder() tea.Cmd {
	if a.loadingOlder || a.open.JID == "" {
		return nil
	}
	if !a.hasMore {
		return a.requestHistory()
	}
	a.loadingOlder = true
	return a.loadPage(a.open.JID, a.generation, a.nextBefore, a.nextBeforeID, pageSize, pageOlder)
}

// requestHistory asks the account for messages from before the oldest one
// here. It is asked once for each oldest message: scrolling asked for it,
// nobody typed anything, and the daemon turning down a request that is already
// on its way is coordination rather than a failure to report.
func (a *app) requestHistory() tea.Cmd {
	if a.waitingHistory || a.open.JID == "" || len(a.messages) == 0 {
		return nil
	}
	boundary := a.open.JID + ":" + a.messages[0].ID
	if a.askedHistory[boundary] {
		return nil
	}
	a.askedHistory[boundary] = true
	a.waitingHistory = true
	return a.quiet("history.request", map[string]any{"chat_jid": a.open.JID, "limit": pageSize})
}

// refreshOpen reads the open conversation again, asking for as much as the
// reader had loaded rather than for one page, so somebody who had scrolled
// back is not dropped at the newest message.
func (a *app) refreshOpen() tea.Cmd {
	if a.open.JID == "" {
		return nil
	}
	limit := len(a.messages)
	if limit < pageSize {
		limit = pageSize
	}
	if limit > pageMost {
		limit = pageMost
	}
	return a.loadPage(a.open.JID, a.generation, 0, "", limit, pageReset)
}

// refreshOne re-reads a single message and puts it back where it was, which is
// what a reaction, an edit or a deletion needs. Reading the whole conversation
// again for one of them would throw away every older page the reader had
// loaded.
func (a *app) refreshOne(id string) tea.Cmd {
	if id == "" || a.open.JID == "" {
		return nil
	}
	if a.indexOf(id) < 0 {
		// Not a message on screen: it belongs to a page nobody has loaded,
		// and there is nothing here to put back.
		return a.loadChats()
	}
	chat, generation := a.open.JID, a.generation
	return func() tea.Msg {
		var message model.Message
		if err := a.daemon.call(a.ctx, "message.get",
			map[string]any{"chat_jid": chat, "message_id": id}, &message); err != nil {
			return nil
		}
		return oneMessageMsg{generation: generation, chat: chat, message: message}
	}
}

// acknowledgeOpen tells everyone who wrote in this conversation that what they
// wrote has been seen, one request for each of them, carrying the newest
// timestamp of theirs. A conversation with nothing from anybody else still
// gets the plain form, which is what clears its unread count.
func (a *app) acknowledgeOpen() tea.Cmd {
	if !a.takesReceipts() {
		return nil
	}
	senders := make([]string, 0, 4)
	ids := map[string][]string{}
	newest := map[string]int64{}
	for _, message := range a.messages {
		if message.FromMe {
			continue
		}
		if _, seen := ids[message.SenderJID]; !seen {
			senders = append(senders, message.SenderJID)
		}
		ids[message.SenderJID] = append(ids[message.SenderJID], message.ID)
		if message.Timestamp > newest[message.SenderJID] {
			newest[message.SenderJID] = message.Timestamp
		}
	}
	if len(senders) == 0 {
		return a.quiet("chat.read", map[string]any{"chat_jid": a.open.JID})
	}
	asks := make([]tea.Cmd, 0, len(senders))
	for _, sender := range senders {
		asks = append(asks, a.quiet("chat.read", map[string]any{
			"chat_jid": a.open.JID, "sender_jid": sender,
			"message_ids": ids[sender], "timestamp": newest[sender],
		}))
	}
	return tea.Batch(asks...)
}

// acknowledgeIncoming is the receipt for one message that arrived while the
// reader was looking at the conversation. Only the page that opens a chat used
// to send receipts, and a message arriving after that stayed unread until the
// conversation was opened again.
func (a *app) acknowledgeIncoming(message model.Message) tea.Cmd {
	if message.FromMe || message.ID == "" || !a.takesReceipts() {
		return nil
	}
	return a.quiet("chat.read", map[string]any{
		"chat_jid": a.open.JID, "sender_jid": message.SenderJID,
		"message_ids": []string{message.ID}, "timestamp": message.Timestamp,
	})
}

// takesReceipts is false for a channel. Nobody is waiting to know that a
// follower opened a post, and the address is not one a receipt can be sent to.
func (a *app) takesReceipts() bool {
	return a.open.JID != "" && !strings.HasSuffix(a.open.JID, "@newsletter")
}

func (a *app) indexOf(id string) int {
	for index := range a.messages {
		if a.messages[index].ID == id {
			return index
		}
	}
	return -1
}

// upsert puts a message where it belongs: in place of the one with its id, or
// at the end, which is where a new message belongs in a conversation ordered
// oldest first.
func (a *app) upsert(message model.Message) {
	if at := a.indexOf(message.ID); at >= 0 {
		a.messages[at] = message
		return
	}
	a.messages = append(a.messages, message)
}

// prepend puts an older page in front of what is on screen and says how many
// of it were new. Ids already here are not added a second time: a page can
// overlap the one above it when messages share a timestamp, and a conversation
// showing the same message twice is a fault the reader sees.
func (a *app) prepend(older []model.Message) int {
	if len(older) == 0 {
		return 0
	}
	here := make(map[string]bool, len(a.messages))
	for _, message := range a.messages {
		here[message.ID] = true
	}
	grown := make([]model.Message, 0, len(older)+len(a.messages))
	for _, message := range older {
		if !here[message.ID] {
			grown = append(grown, message)
		}
	}
	a.messages = append(grown, a.messages...)
	return len(grown)
}

// selectedMessage is the message the keyboard is on, which is what the keys
// that act on one message act on.
func (a *app) selectedMessage() (model.Message, bool) {
	if a.cursor < 0 || a.cursor >= len(a.messages) {
		return model.Message{}, false
	}
	return a.messages[a.cursor], true
}

// applyReceipt moves the marks beside the messages the receipt names. A
// receipt that never reached the open conversation left a sent message with
// its single mark until the conversation was opened again.
func (a *app) applyReceipt(ids []string, state string, when int64) {
	for _, id := range ids {
		at := a.indexOf(id)
		if at < 0 {
			continue
		}
		message := &a.messages[at]
		message.Status = state
		switch state {
		case "played":
			message.PlayedAt = when
		case "read":
			message.ReadAt = when
		case "delivered":
			message.DeliveredAt = when
		}
	}
}

func (a *app) applyStar(id string, starred bool) {
	if at := a.indexOf(id); at >= 0 {
		a.messages[at].Starred = starred
	}
}

// conversationTitle names what is open, and says what is known about it.
func (a *app) conversationTitle(width int) string {
	if a.open.JID == "" {
		return styleMuted.Render("Nothing open")
	}
	name := safeLine(a.open.Title)
	if name == "" {
		name = safeLine(shortJID(a.open.JID))
	}
	return styleTitle.Render(fitLine(name, width))
}

// conversationLines is the whole loaded conversation, oldest first, one
// terminal row per entry, with the rows the message the keyboard is on
// occupies. The window onto it is taken in View, because how much of it is on
// screen is a question about the terminal rather than about the conversation,
// and the window has to hold that message wherever it is.
//
// Which message each row belongs to is kept in lineOwners, so a click on a row
// reaches the message drawn there.
func (a *app) conversationLines(width int) (lines []string, from, to int) {
	a.lineOwners = nil
	if a.open.JID == "" {
		return []string{styleMuted.Render("Open a conversation with enter")}, 0, 0
	}
	if len(a.messages) == 0 {
		if a.waitingHistory || a.loadingOlder {
			return []string{styleMuted.Render("Asking for messages…")}, 0, 0
		}
		return []string{styleMuted.Render("No messages here yet")}, 0, 0
	}
	group := strings.HasSuffix(a.open.JID, "@g.us")
	lines = make([]string, 0, len(a.messages)*2+1)
	owners := make([]int, 0, cap(lines))
	owned := func(index int) {
		for len(owners) < len(lines) {
			owners = append(owners, index)
		}
	}
	switch {
	case a.waitingHistory:
		lines = append(lines, styleMuted.Render("asking the account for older messages…"))
	case a.hasMore:
		lines = append(lines, styleMuted.Render("older messages above"))
	}
	lastDay := ""
	for index, message := range a.messages {
		// A row of the window's own colour between two bubbles keeps them
		// apart.
		if index > 0 {
			lines = append(lines, "")
		}
		if message.Timestamp > 0 {
			if day := time.UnixMilli(message.Timestamp).Format(time.DateOnly); day != lastDay {
				lines = append(lines, dayLine(message.Timestamp, width))
				lastDay = day
			}
		}
		if message.ID != "" && message.ID == a.unreadFrom && a.unreadCount > 0 {
			lines = append(lines, styleUnread.Render(fmt.Sprintf("── %s ──",
				plural(a.unreadCount, "unread message", "unread messages"))))
		}
		here := index == a.cursor
		owned(-1)
		if here {
			from = len(lines)
		}
		lines = append(lines, a.messageLines(message, group, width, here)...)
		owned(index)
		if here {
			to = len(lines) - 1
		}
	}
	owned(-1)
	a.lineOwners = owners
	return lines, from, to
}

// messageLines draws one message as a bubble, the way the window does: what
// this account sent on the right, what anybody else sent on the left, and the
// clock and how far it got in the bubble's bottom corner.
//
// Which message the keys act on has to be visible without colour, since a
// terminal may have none, so its bubble is drawn with a heavy line.
func (a *app) messageLines(message model.Message, group bool, width int, selected bool) []string {
	// Room for the mark that shows which message the keys are on, and for a
	// space either side of the words.
	const frame = 4
	// A bubble takes at most three quarters of the pane, as in the window.
	most := max(8, width*3/4-frame)
	paint := paintTheirs
	if message.FromMe {
		paint = paintOwn
	}
	fill := lipgloss.NewStyle().Background(paint.fill)
	words, muted := fill.Foreground(paint.words), fill.Foreground(paint.muted)

	// Each row of the bubble, put in the order the terminal shows it, and
	// whether it was written right to left: those start at the bubble's
	// right edge, which is where somebody reading them looks for the start.
	type row struct {
		text  string
		style lipgloss.Style
		right bool
	}
	var rows []row
	line := func(text string, style lipgloss.Style) {
		rows = append(rows, row{fitLine(text, most), style, rightToLeft(text)})
	}
	if group && !message.FromMe {
		line(a.who(message, group), fill.Foreground(paint.sender).Bold(true))
	}
	if quoted := replyLine(message); quoted != "" {
		line(quoted, muted)
	}
	body := messageBody(message)
	if message.Kind == "audio" && !message.Revoked {
		// A voice note says it can be played, and that it is playing.
		body = pick(a.playing.id == message.ID && a.playing.command != nil, "■ ", "▶ ") + body
	}
	for _, paragraph := range strings.Split(body, "\n") {
		rtl := rightToLeft(paragraph)
		for _, wrapped := range strings.Split(ansi.Wrap(paragraph, most, " -"), "\n") {
			rows = append(rows, row{visualIn(wrapped, rtl), words, rtl})
		}
	}
	if reactions := reactionLine(message); reactions != "" {
		line(reactions, words)
	}
	footer := stamp(message.Timestamp)
	if marks := messageMarks(message); marks != "" {
		footer += " " + marks
	}
	rows = append(rows, row{fitLine(footer, most), muted, true})

	inner := 0
	for _, r := range rows {
		inner = max(inner, ansi.StringWidth(r.text))
	}
	inner = min(inner, most)
	side := fill.Render(" ")
	lines := make([]string, len(rows))
	for at, r := range rows {
		align := lipgloss.Left
		if r.right {
			align = lipgloss.Right
		}
		// A bar beside the message the keys are on, the whole height of it.
		mark := "  "
		if selected {
			mark = styleTitle.Render("▌") + " "
		}
		lines[at] = mark + side + r.style.Width(inner).Align(align).Render(r.text) + side
	}
	if message.FromMe {
		for at := range lines {
			lines[at] = lipgloss.PlaceHorizontal(width, lipgloss.Right, lines[at])
		}
	}
	return lines
}

// rightToLeft says whether a line starts in a script written right to left,
// judged by its first letter, which is how the window decides it too.
func rightToLeft(line string) bool {
	for _, char := range ansi.Strip(line) {
		switch {
		case unicode.In(char, unicode.Hebrew, unicode.Arabic, unicode.Syriac, unicode.Thaana):
			return true
		case unicode.IsLetter(char):
			return false
		}
	}
	return false
}

// dayLine names the day the messages under it were sent on, where it changes.
func dayLine(milliseconds int64, width int) string {
	day := time.UnixMilli(milliseconds).Format("Monday, 2 January 2006")
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, styleMuted.Render("── "+day+" ──"))
}

// who wrote it. A conversation with one other person says their name once, at
// the top; inside a group every message says who wrote it, because anybody
// might have.
func (a *app) who(message model.Message, group bool) string {
	if message.FromMe {
		return "You"
	}
	for _, candidate := range []string{message.SenderName, pick(group, "", a.open.Title), shortJID(message.SenderJID)} {
		if name := safeLine(candidate); name != "" {
			return name
		}
	}
	return "Somebody"
}

// messageMarks are what the desktop client draws beside a message: how far it
// got, whether it was changed after it was sent, whether it was passed on, and
// whether it is kept.
func messageMarks(message model.Message) string {
	marks := make([]string, 0, 4)
	if message.FromMe && !message.Revoked {
		switch {
		case message.PlayedAt > 0 || message.Status == "played":
			marks = append(marks, "✓✓ played")
		case message.ReadAt > 0 || message.Status == "read":
			marks = append(marks, "✓✓ read")
		case message.DeliveredAt > 0 || message.Status == "delivered":
			marks = append(marks, "✓✓")
		default:
			marks = append(marks, "✓")
		}
	}
	if message.Edited {
		marks = append(marks, "edited")
	}
	if message.ForwardingScore > 0 {
		marks = append(marks, "forwarded")
	}
	if message.Starred {
		marks = append(marks, "★")
	}
	return strings.Join(marks, " · ")
}

// replyLine is the message this one answers, as the sender quoted it.
func replyLine(message model.Message) string {
	if message.ReplyTo == "" && message.ReplyPreview == "" {
		return ""
	}
	who := safeLine(message.ReplySender)
	if message.ReplyFromMe {
		who = "You"
	}
	if who == "" {
		who = "earlier"
	}
	preview := safeLine(message.ReplyPreview)
	if message.ReplyViewOnce {
		preview = "view once"
	}
	if preview == "" {
		preview = "a message"
	}
	return "┌ " + who + ": " + preview
}

func reactionLine(message model.Message) string {
	if len(message.Reactions) == 0 {
		return ""
	}
	counted := map[string]int{}
	order := make([]string, 0, len(message.Reactions))
	for _, reaction := range message.Reactions {
		emoji := safeLine(reaction.Emoji)
		if emoji == "" {
			continue
		}
		if counted[emoji] == 0 {
			order = append(order, emoji)
		}
		counted[emoji]++
	}
	parts := make([]string, 0, len(order))
	for _, emoji := range order {
		if counted[emoji] > 1 {
			parts = append(parts, fmt.Sprintf("%s%d", emoji, counted[emoji]))
			continue
		}
		parts = append(parts, emoji)
	}
	return strings.Join(parts, " ")
}

// messageBody is what was said. Anything that is not words - a picture, a
// recording, a shared place - is named instead of drawn: nothing is rendered
// in the terminal, and opening it hands the file to the program that handles
// its kind.
func messageBody(message model.Message) string {
	if message.Revoked {
		return "this message was deleted"
	}
	said := safeText(message.Body)
	placeholder := mediaPlaceholder(message)
	switch {
	case placeholder != "" && said != "":
		return placeholder + "\n" + said
	case placeholder != "":
		return placeholder
	case said != "":
		return said
	}
	return "(nothing)"
}

// mediaPlaceholder names what came instead of words, with what is known about
// it: its kind, what it is called, how big it is, how long it runs.
func mediaPlaceholder(message model.Message) string {
	switch message.Kind {
	case "", "text":
		return ""
	case "image":
		return bracket("photo", message.MediaName, humanSize(message.MediaSize))
	case "video":
		kind := "video"
		if message.GIFPlayback {
			kind = "GIF"
		}
		return bracket(kind, message.MediaName, humanSize(message.MediaSize), runtimeOf(message.MediaDuration))
	case "audio":
		return bracket("voice note", message.MediaName, humanSize(message.MediaSize), runtimeOf(message.MediaDuration))
	case "sticker":
		return bracket("sticker", "", humanSize(message.MediaSize))
	case "document":
		return bracket("document", message.MediaName, humanSize(message.MediaSize))
	case "view_once":
		return bracket("view once", message.MediaName, humanSize(message.MediaSize))
	case "contact":
		who := safeLine(message.ContactName)
		if message.ContactCount > 1 {
			return bracket(plural(message.ContactCount, "contact", "contacts"), who, "")
		}
		return bracket("contact", who, safeLine(message.ContactPhone))
	case "location":
		return bracket("location", fmt.Sprintf("%.5f, %.5f", message.Latitude, message.Longitude), "")
	case "poll":
		return bracket("poll", "", "")
	case "event":
		return bracket("event", "", "")
	case "system":
		return ""
	}
	return bracket(safeLine(message.Kind), message.MediaName, humanSize(message.MediaSize))
}

// bracket writes one of those placeholders: the kind first, then whatever else
// is known, and nothing where nothing is known.
func bracket(kind string, name string, rest ...string) string {
	parts := make([]string, 0, len(rest)+2)
	parts = append(parts, kind)
	if cleaned := safeLine(name); cleaned != "" {
		parts = append(parts, cleaned)
	}
	for _, part := range rest {
		if cleaned := safeLine(part); cleaned != "" {
			parts = append(parts, cleaned)
		}
	}
	return "[" + strings.Join(parts, " · ") + "]"
}

func humanSize(bytes int64) string {
	switch {
	case bytes <= 0:
		return ""
	case bytes < 1024:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.0f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}

func runtimeOf(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func stamp(milliseconds int64) string {
	if milliseconds <= 0 {
		return "--:--"
	}
	return time.UnixMilli(milliseconds).Format("15:04")
}

// shortJID is the part of an address worth showing when there is no name for
// it: the number, without the server it lives on.
func shortJID(jid string) string {
	name := safeLine(jid)
	if at := strings.IndexByte(name, '@'); at > 0 {
		name = name[:at]
	}
	if colon := strings.IndexByte(name, ':'); colon > 0 {
		name = name[:colon]
	}
	return name
}

func plural(count int, one, many string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, one)
	}
	return fmt.Sprintf("%d %s", count, many)
}

func pick(when bool, yes, no string) string {
	if when {
		return yes
	}
	return no
}
