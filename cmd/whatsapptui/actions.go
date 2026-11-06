package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/shukiv/whatsappgo/internal/model"
)

// The six reactions the desktop client offers under a message, in its order.
var quickReactions = []string{"👍", "❤️", "😂", "😮", "😢", "🙏"}

// How long a pinned message stays pinned, as the desktop client's dialog
// offers it. Seven days is what that dialog starts on.
const (
	pinDay   = 24 * 60 * 60
	pinWeek  = 7 * pinDay
	pinMonth = 30 * pinDay
)

// chooser is the one-of-several prompt the window lays over the conversation:
// which reaction, how long to pin, which conversation to forward to, and
// whether to go through with something that cannot be undone. A read-only list
// - the earlier versions of a message, the kept messages, the keys themselves -
// is the same thing with nothing to choose.
type chooser struct {
	title  string
	hint   string
	items  []choice
	cursor int
	// keys are what a list does beyond choosing, as going up a folder does
	// while a file is chosen, and footer says what they are.
	keys   map[string]func() tea.Cmd
	footer string
	// leftToRight rows read from the left whatever is in them - an icon, a
	// name in any script, a size - as a list of files does.
	leftToRight bool
}

type choice struct {
	label string
	// run is what choosing it does, and nothing is a list that only reads.
	run func() tea.Cmd
}

// Lists that arrive from the daemon rather than being known here.
type (
	revisionsMsg []model.MessageRevision
	starredMsg   []model.Message
)

// wrapped lays a line out over as many rows as it takes.
func wrapped(text string, width int, style lipgloss.Style) []string {
	if width < 8 {
		width = 8
	}
	rtl := rightToLeft(text)
	lines := strings.Split(ansi.Wrap(text, width, " -"), "\n")
	for at, line := range lines {
		lines[at] = style.Render(visualIn(line, rtl))
	}
	return lines
}

func (a *app) chooserKey(msg tea.KeyMsg) tea.Cmd {
	if run, ok := a.chooser.keys[msg.String()]; ok {
		return run()
	}
	switch key := msg.String(); key {
	case "esc", "q":
		a.chooser = nil
	case "up", "k":
		if a.chooser.cursor > 0 {
			a.chooser.cursor--
		}
	case "down", "j":
		if a.chooser.cursor < len(a.chooser.items)-1 {
			a.chooser.cursor++
		}
	case "enter":
		return a.chooseHere()
	default:
		// A short list can be answered by its own number.
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			if at := int(key[0] - '1'); at < len(a.chooser.items) {
				a.chooser.cursor = at
				return a.chooseHere()
			}
		}
	}
	return nil
}

func (a *app) chooseHere() tea.Cmd {
	taken := a.chooser
	a.chooser = nil
	if taken.cursor < 0 || taken.cursor >= len(taken.items) {
		return nil
	}
	if run := taken.items[taken.cursor].run; run != nil {
		return run()
	}
	return nil
}

func (a *app) ask(title, hint string, cursor int, items ...choice) tea.Cmd {
	a.chooser = &chooser{title: title, hint: hint, items: items, cursor: cursor}
	return nil
}

// reactPrompt offers the six reactions the desktop client offers. Choosing the
// one already left takes it back, which is what tapping it does there.
func (a *app) reactPrompt() tea.Cmd {
	message, ok := a.selectedMessage()
	if !ok || message.Revoked {
		return nil
	}
	mine := a.myReaction(message)
	items := make([]choice, 0, len(quickReactions)+1)
	for _, emoji := range quickReactions {
		emoji := emoji
		label := emoji
		if emoji == mine {
			label = emoji + "  leave it"
		}
		items = append(items, choice{label: label, run: func() tea.Cmd {
			if emoji == mine {
				return a.react(message, "")
			}
			return a.react(message, emoji)
		}})
	}
	if mine != "" {
		items = append(items, choice{label: "take mine back", run: func() tea.Cmd { return a.react(message, "") }})
	}
	return a.ask("React to "+a.quoted(message.ID), "", 0, items...)
}

func (a *app) react(message model.Message, emoji string) tea.Cmd {
	return a.quiet("message.react", map[string]any{
		"chat_jid": a.open.JID, "message_id": message.ID,
		"sender_jid": message.SenderJID, "emoji": emoji,
	})
}

// myReaction is the one left from this account, if there is one. Reactions
// arriving as announcements carry only the address, and an address carries the
// device it was sent from, which is not part of who sent it.
func (a *app) myReaction(message model.Message) string {
	for _, reaction := range message.Reactions {
		if sameUser(reaction.SenderJID, a.status.UserJID) {
			return reaction.Emoji
		}
	}
	return ""
}

func sameUser(one, other string) bool {
	return one != "" && other != "" && shortJID(one) == shortJID(other)
}

// toggleStar keeps a message or stops keeping it. The daemon reports the new
// state as an announcement, and that is what moves the mark, in both clients.
func (a *app) toggleStar() tea.Cmd {
	message, ok := a.selectedMessage()
	if !ok {
		return nil
	}
	return a.quiet("message.star", map[string]any{
		"chat_jid": a.open.JID, "message_id": message.ID, "sender_jid": message.SenderJID,
		"from_me": message.FromMe, "starred": !message.Starred,
	})
}

// pinPrompt asks how long, as the desktop client's dialog does, and starts on
// the same answer it starts on.
func (a *app) pinPrompt() tea.Cmd {
	message, ok := a.selectedMessage()
	if !ok || message.Revoked {
		return nil
	}
	pinFor := func(seconds int64) func() tea.Cmd {
		return func() tea.Cmd {
			return a.quiet("message.pin", map[string]any{
				"chat_jid": a.open.JID, "message_id": message.ID,
				"sender_jid": message.SenderJID, "duration_seconds": seconds,
			})
		}
	}
	return a.ask("Pin this message for how long?", "It can be unpinned at any time.", 1,
		choice{label: "24 hours", run: pinFor(pinDay)},
		choice{label: "7 days", run: pinFor(pinWeek)},
		choice{label: "30 days", run: pinFor(pinMonth)},
		choice{label: "unpin it", run: func() tea.Cmd {
			return a.quiet("message.unpin", map[string]any{
				"chat_jid": a.open.JID, "message_id": message.ID, "sender_jid": message.SenderJID,
			})
		}})
}

// forwardPrompt passes a message on to another conversation. Picking the
// conversation out of the list is the deliberate act: it sends a message to
// whoever is chosen.
func (a *app) forwardPrompt() tea.Cmd {
	message, ok := a.selectedMessage()
	if !ok || message.Revoked || len(a.chats) == 0 {
		return nil
	}
	items := make([]choice, 0, len(a.chats))
	for _, chat := range a.chats {
		target := chat.JID
		name := safeLine(chat.Title)
		if name == "" {
			name = safeLine(shortJID(chat.JID))
		}
		items = append(items, choice{label: name, run: func() tea.Cmd {
			return a.quiet("message.forward", map[string]any{
				"chat_jid": a.open.JID, "message_id": message.ID, "to_chat_jid": target,
			})
		}})
	}
	return a.ask("Pass this on to which conversation?", a.quoted(message.ID), 0, items...)
}

// deletePrompt asks before deleting, as the desktop client's dialog does. The
// answer that changes nothing is the one the cursor starts on, because here
// the answer is a single keystroke and this one cannot be undone.
func (a *app) deletePrompt() tea.Cmd {
	message, ok := a.selectedMessage()
	if !ok || message.Revoked {
		return nil
	}
	return a.ask("Delete this message for everyone?", a.quoted(message.ID)+" — this cannot be undone.", 0,
		choice{label: "No, keep it"},
		choice{label: "Delete it for everyone", run: func() tea.Cmd {
			return a.quiet("message.delete", map[string]any{
				"chat_jid": a.open.JID, "message_id": message.ID, "sender_jid": message.SenderJID,
			})
		}})
}

// revisions reads what a message said before it was changed.
func (a *app) revisions() tea.Cmd {
	message, ok := a.selectedMessage()
	if !ok {
		return nil
	}
	chat, id := a.open.JID, message.ID
	return func() tea.Msg {
		var found []model.MessageRevision
		if err := a.daemon.call(a.ctx, "message.revisions",
			map[string]any{"chat_jid": chat, "message_id": id}, &found); err != nil {
			return failureMsg{err}
		}
		return revisionsMsg(found)
	}
}

func (a *app) showRevisions(found []model.MessageRevision) tea.Cmd {
	if len(found) == 0 {
		return a.ask("Earlier versions", "This message has not been changed.", 0,
			choice{label: "close"})
	}
	items := make([]choice, 0, len(found))
	for _, revision := range found {
		reason := safeLine(revision.Reason)
		if reason == "" {
			reason = "changed"
		}
		items = append(items, choice{label: fmt.Sprintf("%s  %s: %s",
			stamp(revision.RecordedAt), reason, safeLine(revision.Body))})
	}
	return a.ask("Earlier versions", "oldest first", 0, items...)
}

// starred lists the messages kept in this conversation.
func (a *app) starred() tea.Cmd {
	if a.open.JID == "" {
		return nil
	}
	chat := a.open.JID
	return func() tea.Msg {
		var found []model.Message
		if err := a.daemon.call(a.ctx, "messages.starred",
			map[string]any{"chat_jid": chat, "limit": pageMost}, &found); err != nil {
			return failureMsg{err}
		}
		return starredMsg(found)
	}
}

func (a *app) showStarred(found []model.Message) tea.Cmd {
	if len(found) == 0 {
		return a.ask("Kept messages", "Nothing is kept in this conversation.", 0,
			choice{label: "close"})
	}
	items := make([]choice, 0, len(found))
	for _, message := range found {
		who := "You"
		if !message.FromMe {
			if who = safeLine(message.SenderName); who == "" {
				who = safeLine(shortJID(message.SenderJID))
			}
		}
		items = append(items, choice{label: fmt.Sprintf("%s  %s: %s",
			stamp(message.Timestamp), who, safeLine(messageBody(message)))})
	}
	return a.ask("Kept messages", plural(len(found), "message", "messages"), 0, items...)
}

// help is every key, since a terminal has nowhere to put them all at once.
func (a *app) help() tea.Cmd {
	keys := []string{
		"enter   open the conversation, or send what is typed",
		"tab     move between the list and the conversation",
		"j k     move through the list, or through the messages",
		"g G     the oldest loaded message, the newest",
		"i       write a message",
		"r       answer the message under the cursor",
		"e       change one of ours",
		"x       react to one",
		"a :     attach, emoji (ctrl+o, ctrl+g while writing)",
		"s       keep one, or stop keeping it",
		"p       pin one, or unpin it",
		"f       pass one on to another conversation",
		"d       delete one for everyone",
		"v       what it said before it was changed",
		"S       the messages kept here",
		"ctrl+r  read the conversation again",
		"esc     back out of the box, a prompt, or the conversation",
		"q       quit, except while writing",
		"ctrl+c  quit, always",
	}
	items := make([]choice, 0, len(keys))
	for _, key := range keys {
		items = append(items, choice{label: key})
	}
	return a.ask("Keys", "", 0, items...)
}

// chooserLines draws the prompt where the conversation was. The title and what
// it says about itself are wrapped rather than cut: what a prompt warns about
// is the part the reader most needs whole, and a prompt about something that
// cannot be undone says so in more characters than a narrow pane holds.
func (a *app) chooserLines(width, height int) string {
	rows := wrapped(a.chooser.title, width, styleTitle)
	if a.chooser.hint != "" {
		rows = append(rows, wrapped(a.chooser.hint, width, styleMuted)...)
	}
	rows = append(rows, "")
	// A long list - every conversation, every kept message - is windowed on
	// the row the cursor is on.
	room := max(1, height-len(rows)-1)
	start := 0
	if a.chooser.cursor >= room {
		start = a.chooser.cursor - room + 1
	}
	for range rows {
		a.hits = append(a.hits, nothing)
	}
	for at := start; at < len(a.chooser.items) && at < start+room; at++ {
		a.hits = append(a.hits, hit{message: -1, item: at})
		// The number stays at the left whichever way the item reads.
		number := " "
		if at < 9 {
			number = fmt.Sprintf(" %d ", at+1)
		}
		label, room := a.chooser.items[at].label, width-ansi.StringWidth(number)
		row := number + fitLine(label, room)
		if a.chooser.leftToRight {
			row = number + visualIn(ansi.Truncate(unjoined(label), room, "…"), false)
		}
		if at == a.chooser.cursor {
			row = styleSelected.Render(row)
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}
