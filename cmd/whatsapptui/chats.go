package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shukiv/whatsappgo/internal/model"
)

// How long a conversation stays quiet, as the desktop client's menu offers it.
// Zero is that menu's "Always": the daemon reads it as until it is undone.
const (
	muteEightHours = 8 * 60 * 60
	muteOneWeek    = 7 * 24 * 60 * 60
	// Written out rather than left to carry the line above it, which is what
	// a constant with no value of its own does.
	muteUntilUndone = 0
)

// How long a message lives before it goes, as the desktop client's dialog
// offers it.
var disappearingChoices = []struct {
	label   string
	seconds int64
}{
	{"off", 0},
	{"24 hours", 86400},
	{"7 days", 604800},
	{"90 days", 7776000},
}

// What a conversation has shared in it, in the daemon's own words. "all" is
// not one of them: the daemon refuses it.
var sharedCategories = []string{"media", "documents", "links", "stickers"}

// asking is the other prompt: the one that wants something typed rather than
// one of several. A search, a file to write, a date to go back to.
type asking struct {
	title string
	hint  string
	input textinput.Model
	run   func(string) tea.Cmd
}

// Answers from the daemon that raise a prompt or change the window.
type (
	countsMsg struct{ unread, archived int }
	infoMsg   model.ChatInfo
	sharedMsg struct {
		category string
		page     model.SharedMessagePage
	}
	labelsMsg struct {
		chat   string
		labels []model.Label
		on     map[string]bool
	}
	foundChatsMsg struct {
		query string
		chats []model.Chat
	}
	onDateMsg struct {
		chat      string
		messageID string
		day       string
	}
	exportedMsg struct{ path string }
)

func (a *app) type_(title, hint, start string, run func(string) tea.Cmd) tea.Cmd {
	input := textinput.New()
	input.Prompt = "› "
	input.CharLimit = 512
	input.SetValue(start)
	input.CursorEnd()
	a.prompt = &asking{title: title, hint: hint, input: input, run: run}
	return a.prompt.input.Focus()
}

func (a *app) promptKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.prompt = nil
		return nil
	case "enter":
		taken := a.prompt
		a.prompt = nil
		return taken.run(strings.TrimSpace(taken.input.Value()))
	}
	var moved tea.Cmd
	a.prompt.input, moved = a.prompt.input.Update(msg)
	return moved
}

func (a *app) promptLines(width int) string {
	rows := wrapped(a.prompt.title, width, styleTitle)
	if a.prompt.hint != "" {
		rows = append(rows, wrapped(a.prompt.hint, width, styleMuted)...)
	}
	rows = append(rows, "")
	typed := inputRows(a.prompt.input.Value(), a.prompt.input.Position(), max(4, width-2), 3, "", true, lipgloss.NewStyle())
	for at, row := range typed {
		prompt := "  "
		if at == 0 {
			prompt = "› "
		}
		rows = append(rows, prompt+row)
	}
	return strings.Join(rows, "\n")
}

// subjectChat is the conversation the menu acts on: the one that is open, or
// the one the cursor is on in the list.
func (a *app) subjectChat() (model.Chat, bool) {
	if a.focus == paneConversation && a.open.JID != "" {
		return a.open, true
	}
	return a.selectedChat()
}

// chatMenu is everything that can be done to a conversation, which the desktop
// client keeps in the menu on its row and in the one above the conversation.
func (a *app) chatMenu() tea.Cmd {
	chat, ok := a.subjectChat()
	if !ok {
		return nil
	}
	flag := func(method string, value bool) func() tea.Cmd {
		return func() tea.Cmd {
			return a.quietThenRefresh(method, map[string]any{"chat_jid": chat.JID, "value": value})
		}
	}
	items := []choice{
		{label: pick(chat.Pinned, "unpin this conversation", "pin this conversation"),
			run: flag("chat.pin", !chat.Pinned)},
		{label: pick(chat.MutedUntil > model.NowMillis(), "turn notifications back on", "mute it"),
			run: func() tea.Cmd { return a.mutePrompt(chat) }},
		{label: pick(chat.Archived, "take it out of the archive", "archive it"),
			run: flag("chat.archive", !chat.Archived)},
		{label: pick(chat.Favorite, "take it out of the favourites", "make it a favourite"),
			run: flag("chat.favorite", !chat.Favorite)},
		{label: pick(chat.UnreadCount > 0, "mark it read", "mark it unread"),
			run: flag("chat.set_read", chat.UnreadCount > 0)},
		{label: "what is in this conversation", run: func() tea.Cmd { return a.chatInfo(chat) }},
		{label: "what has been shared in it", run: func() tea.Cmd { return a.sharedPrompt(chat) }},
		{label: "open its picture", run: func() tea.Cmd { return a.openAvatar(chat) }},
		{label: "the lists it is on", run: func() tea.Cmd { return a.labelsPrompt(chat) }},
		{label: "how long messages live", run: func() tea.Cmd { return a.disappearingPrompt(chat) }},
		{label: "write it to a file", run: func() tea.Cmd { return a.exportPrompt(chat, false) }},
		{label: "go back to a date", run: func() tea.Cmd { return a.onDatePrompt(chat) }},
		{label: "empty it of messages", run: func() tea.Cmd { return a.clearPrompt(chat) }},
		{label: "delete the conversation", run: func() tea.Cmd { return a.deleteChatPrompt(chat) }},
	}
	name := safeLine(chat.Title)
	if name == "" {
		name = safeLine(shortJID(chat.JID))
	}
	return a.ask(name, safeLine(shortJID(chat.JID)), 0, items...)
}

// quietThenRefresh is for the changes the daemon does not announce back to
// every client, where the list has to be read again to show the new state.
func (a *app) quietThenRefresh(method string, params map[string]any) tea.Cmd {
	return tea.Batch(a.quiet(method, params), a.loadChats(), a.counts())
}

func (a *app) mutePrompt(chat model.Chat) tea.Cmd {
	quiet := func(value bool, seconds int64) func() tea.Cmd {
		return func() tea.Cmd {
			return a.quietThenRefresh("chat.mute", map[string]any{
				"chat_jid": chat.JID, "value": value, "duration_seconds": seconds})
		}
	}
	return a.ask("Mute this conversation for how long?", "", 0,
		choice{label: "notifications on", run: quiet(false, 0)},
		choice{label: "8 hours", run: quiet(true, muteEightHours)},
		choice{label: "1 week", run: quiet(true, muteOneWeek)},
		choice{label: "always", run: quiet(true, muteUntilUndone)})
}

func (a *app) disappearingPrompt(chat model.Chat) tea.Cmd {
	items := make([]choice, 0, len(disappearingChoices))
	at := 0
	for index, span := range disappearingChoices {
		seconds := span.seconds
		if seconds == chat.DisappearingSeconds {
			at = index
		}
		items = append(items, choice{label: span.label, run: func() tea.Cmd {
			return a.quietThenRefresh("chat.disappearing", map[string]any{
				"chat_jid": chat.JID, "duration_seconds": seconds})
		}})
	}
	// The prompt starts on what the conversation is set to now, as the
	// desktop client's dialog does.
	return a.ask("How long should new messages live?", "This changes it for everybody in the conversation.", at, items...)
}

func (a *app) clearPrompt(chat model.Chat) tea.Cmd {
	return a.ask("Empty this conversation of messages?",
		"The messages go from this computer and this cannot be undone.", 0,
		choice{label: "No, leave it"},
		choice{label: "Empty it", run: func() tea.Cmd {
			return a.quietThenRefresh("chat.clear", map[string]any{"chat_jid": chat.JID})
		}})
}

func (a *app) deleteChatPrompt(chat model.Chat) tea.Cmd {
	return a.ask("Delete this conversation?",
		"It goes with its messages, and this cannot be undone.", 0,
		choice{label: "No, keep it"},
		choice{label: "Delete it", run: func() tea.Cmd {
			closing := a.open.JID == chat.JID
			return tea.Batch(a.quiet("chat.delete", map[string]any{"chat_jid": chat.JID}),
				func() tea.Msg {
					// A conversation that is gone cannot stay on screen.
					if closing {
						return closeOpenMsg{}
					}
					return nil
				}, a.loadChats(), a.counts())
		}})
}

// closeOpenMsg says the open conversation is no longer there to be open.
type closeOpenMsg struct{}

func (a *app) chatInfo(chat model.Chat) tea.Cmd {
	return func() tea.Msg {
		var info model.ChatInfo
		if err := a.daemon.call(a.ctx, "chat.info", map[string]any{"chat_jid": chat.JID}, &info); err != nil {
			return failureMsg{err}
		}
		return infoMsg(info)
	}
}

func (a *app) showInfo(info model.ChatInfo) tea.Cmd {
	rows := make([]choice, 0, 8)
	add := func(format string, parts ...any) {
		rows = append(rows, choice{label: safeLine(fmt.Sprintf(format, parts...))})
	}
	if info.Phone != "" {
		add("number: %s", info.Phone)
	}
	if info.LastSeen > 0 {
		add("last seen: %s", time.UnixMilli(info.LastSeen).Format("2 Jan 15:04"))
	}
	if info.Chat.IsGroup {
		add("a group")
	}
	if info.PinnedMessage != nil {
		add("pinned: %s", messageBody(*info.PinnedMessage))
	}
	add("shared: %d, pictures and video: %d, files: %d, links: %d",
		info.SharedCount, info.MediaCount, info.DocumentCount, info.LinkCount)
	if info.Chat.DisappearingSeconds > 0 {
		add("messages live for %s", span(info.Chat.DisappearingSeconds))
	}
	if info.Chat.MutedUntil > model.NowMillis() {
		add("quiet until %s", time.UnixMilli(info.Chat.MutedUntil).Format("2 Jan 15:04"))
	}
	for _, state := range []struct {
		on   bool
		said string
	}{{info.Chat.Pinned, "pinned to the top"}, {info.Chat.Favorite, "a favourite"},
		{info.Chat.Archived, "archived"}} {
		if state.on {
			add("%s", state.said)
		}
	}
	name := safeLine(info.Chat.Title)
	if name == "" {
		name = safeLine(shortJID(info.Chat.JID))
	}
	return a.ask(name, safeLine(shortJID(info.Chat.JID)), 0, rows...)
}

func (a *app) sharedPrompt(chat model.Chat) tea.Cmd {
	items := make([]choice, 0, len(sharedCategories))
	for _, category := range sharedCategories {
		category := category
		items = append(items, choice{label: category, run: func() tea.Cmd {
			return func() tea.Msg {
				var page model.SharedMessagePage
				if err := a.daemon.call(a.ctx, "chat.shared", map[string]any{
					"chat_jid": chat.JID, "category": category, "offset": 0, "limit": pageSize}, &page); err != nil {
					return failureMsg{err}
				}
				return sharedMsg{category: category, page: page}
			}
		}})
	}
	return a.ask("What has been shared here?", "", 0, items...)
}

func (a *app) showShared(shared sharedMsg) tea.Cmd {
	if len(shared.page.Messages) == 0 {
		return a.ask("Shared: "+shared.category, "Nothing of that kind here.", 0, choice{label: "close"})
	}
	items := make([]choice, 0, len(shared.page.Messages))
	for _, message := range shared.page.Messages {
		items = append(items, choice{label: fmt.Sprintf("%s  %s",
			stamp(message.Timestamp), safeLine(messageBody(message)))})
	}
	hint := plural(len(shared.page.Messages), "item", "items")
	if shared.page.HasMore {
		hint += ", and more"
	}
	return a.ask("Shared: "+shared.category, hint, 0, items...)
}

// labelsPrompt reads the lists there are and which of them this conversation
// is on, then offers each as something to turn on or off.
func (a *app) labelsPrompt(chat model.Chat) tea.Cmd {
	return func() tea.Msg {
		var all struct {
			Labels []model.Label `json:"labels"`
		}
		if err := a.daemon.call(a.ctx, "labels.list", nil, &all); err != nil {
			return failureMsg{err}
		}
		var on struct {
			IDs []string `json:"label_ids"`
		}
		if err := a.daemon.call(a.ctx, "chat.labels", map[string]any{"chat_jid": chat.JID}, &on); err != nil {
			return failureMsg{err}
		}
		has := make(map[string]bool, len(on.IDs))
		for _, id := range on.IDs {
			has[id] = true
		}
		return labelsMsg{chat: chat.JID, labels: all.Labels, on: has}
	}
}

func (a *app) showLabels(found labelsMsg) tea.Cmd {
	if len(found.labels) == 0 {
		return a.ask("Lists", "There are no lists on this account yet.", 0, choice{label: "close"})
	}
	items := make([]choice, 0, len(found.labels))
	for _, label := range found.labels {
		id, name, on := label.ID, safeLine(label.Name), found.on[label.ID]
		mark := "  "
		if on {
			mark = "✓ "
		}
		items = append(items, choice{label: mark + name, run: func() tea.Cmd {
			return a.quietThenRefresh("chat.label", map[string]any{
				"chat_jid": found.chat, "label_id": id, "value": !on})
		}})
	}
	return a.ask("Lists", "enter turns one on or off", 0, items...)
}

// exportPrompt asks where to write the conversation. The daemon will not
// replace a file unless it is told to, so a file that is already there comes
// back as a question rather than as a loss.
func (a *app) exportPrompt(chat model.Chat, replace bool) tea.Cmd {
	name := safeLine(chat.Title)
	if name == "" {
		name = shortJID(chat.JID)
	}
	suggestion := filepath.Join(exportDirectory(), fmt.Sprintf("%s.txt", safeName(name)))
	hint := "The file is written on this computer."
	if replace {
		hint = "That file is already there. Writing it again replaces it."
	}
	return a.type_("Write this conversation to which file?", hint, suggestion, func(path string) tea.Cmd {
		if path == "" {
			return nil
		}
		return func() tea.Msg {
			var written struct {
				Path string `json:"path"`
			}
			err := a.daemon.call(a.ctx, "chat.export", map[string]any{
				"chat_jid": chat.JID, "path": path, "replace": replace}, &written)
			if err != nil {
				// A file that is already there is the one failure worth
				// asking about rather than reporting.
				if !replace && strings.Contains(strings.ToLower(err.Error()), "exists") {
					return replaceFileMsg{chat: chat}
				}
				return failureMsg{err}
			}
			if written.Path == "" {
				written.Path = path
			}
			return exportedMsg{path: written.Path}
		}
	})
}

type replaceFileMsg struct{ chat model.Chat }

func exportDirectory() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

// safeName keeps a file name to what every filesystem takes.
func safeName(name string) string {
	var out strings.Builder
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9',
			char == '-', char == '_':
			out.WriteRune(char)
		case char == ' ':
			out.WriteByte('-')
		}
	}
	if out.Len() == 0 {
		return "conversation"
	}
	return out.String()
}

// onDatePrompt asks for a day and goes back to the first message on it, which
// means reading older pages until that message is here.
func (a *app) onDatePrompt(chat model.Chat) tea.Cmd {
	return a.type_("Go back to which day?", "As 2026-11-04.", "", func(day string) tea.Cmd {
		start, err := time.ParseInLocation("2006-01-02", day, time.Local)
		if err != nil {
			a.failure = "A day is written as 2026-11-04."
			return nil
		}
		from, to := start.UnixMilli(), start.AddDate(0, 0, 1).UnixMilli()-1
		return func() tea.Msg {
			var found struct {
				MessageID string `json:"message_id"`
			}
			if err := a.daemon.call(a.ctx, "messages.on_date", map[string]any{
				"chat_jid": chat.JID, "start": from, "end": to}, &found); err != nil {
				return failureMsg{err}
			}
			return onDateMsg{chat: chat.JID, messageID: found.MessageID, day: day}
		}
	})
}

// seek puts the cursor on a message, reading older pages until it is here.
// Bounded, because a day nobody said anything near would otherwise read the
// whole conversation one page at a time.
const seekPages = 20

func (a *app) seek(id string) tea.Cmd {
	if at := a.indexOf(id); at >= 0 {
		a.cursor, a.seeking, a.seekingPages = at, "", 0
		return nil
	}
	if !a.hasMore || a.seekingPages >= seekPages {
		a.seeking, a.seekingPages = "", 0
		a.failure = "That day is further back than this conversation goes here."
		return nil
	}
	a.seeking = id
	a.seekingPages++
	return a.loadOlder()
}

// counts are the two numbers the desktop client keeps above its list: how many
// messages are waiting anywhere, and how many conversations are in the
// archive.
func (a *app) counts() tea.Cmd {
	return func() tea.Msg {
		var unread, archived struct {
			Count int `json:"count"`
		}
		if err := a.daemon.call(a.ctx, "chats.unread_count", nil, &unread); err != nil {
			return nil
		}
		if err := a.daemon.call(a.ctx, "chats.archived_count", nil, &archived); err != nil {
			return nil
		}
		return countsMsg{unread: unread.Count, archived: archived.Count}
	}
}

// markAllReadPrompt asks first: it reaches every conversation on the account,
// and everybody who wrote in one of them is told their messages were read.
func (a *app) markAllReadPrompt() tea.Cmd {
	return a.ask("Mark every conversation read?",
		"Everybody who wrote is told their messages were read.", 0,
		choice{label: "No"},
		choice{label: "Mark them all read", run: func() tea.Cmd {
			return tea.Batch(a.quiet("chats.mark_all_read", nil), a.loadChats(), a.counts())
		}})
}

// searchPrompt looks for a conversation by name or number.
func (a *app) searchPrompt() tea.Cmd {
	return a.type_("Find a conversation", "by name or by number", "", func(query string) tea.Cmd {
		if query == "" {
			return tea.Batch(a.loadChats(), a.counts())
		}
		return func() tea.Msg {
			var found []model.Chat
			if err := a.daemon.call(a.ctx, "chats.search",
				map[string]any{"query": query, "limit": pageMost}, &found); err != nil {
				return failureMsg{err}
			}
			return foundChatsMsg{query: query, chats: found}
		}
	})
}

// archived shows the conversations in the archive, which are kept out of the
// list until they are asked for.
func (a *app) archived() tea.Cmd {
	return func() tea.Msg {
		var found []model.Chat
		if err := a.daemon.call(a.ctx, "chats.list",
			map[string]any{"limit": pageMost, "archived": true}, &found); err != nil {
			return failureMsg{err}
		}
		return foundChatsMsg{query: "the archive", chats: found}
	}
}

// span writes a number of seconds the way the window writes one.
func span(seconds int64) string {
	for _, choice := range disappearingChoices {
		if choice.seconds == seconds {
			return choice.label
		}
	}
	if seconds%86400 == 0 {
		return plural(int(seconds/86400), "day", "days")
	}
	return plural(int(seconds/3600), "hour", "hours")
}
