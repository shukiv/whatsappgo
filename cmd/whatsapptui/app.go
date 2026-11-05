package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/rpc"
)

// pane names the half of the window the keyboard is in. The desktop client
// has a list beside a conversation; so does this.
type pane int

const (
	paneChats pane = iota
	paneConversation
)

type status struct {
	LoggedIn bool   `json:"logged_in"`
	State    string `json:"state"`
	UserName string `json:"user_name"`
	// Which account this is, which is how a reaction left from here is told
	// apart from everybody else's.
	UserJID string `json:"user_jid"`
}

// Messages the interface reacts to. Every one of them is something that
// happened elsewhere - the daemon answered, the daemon announced, the terminal
// resized - because the interface itself never blocks.
type (
	statusMsg  status
	chatsMsg   []model.Chat
	eventMsg   rpc.Event
	failureMsg struct{ err error }
)

type app struct {
	ctx     context.Context
	daemon  socket
	profile string

	width, height int
	focus         pane
	status        status
	chats         []model.Chat
	selected      int
	chatTop       int
	failure       string

	// The open conversation, and the page of it that has been read. The
	// generation counts how many times a conversation has been opened, so an
	// answer about one the reader has left can be recognised and dropped.
	open         model.Chat
	generation   int
	messages     []model.Message
	hasMore      bool
	nextBefore   int64
	nextBeforeID string
	unreadFrom   string
	unreadCount  int

	// What the list is showing, when it is not simply the conversations:
	// a search, or the archive.
	listing string

	// Which message the keys act on, and the first row of the conversation
	// that is on screen. The window follows the keyboard: whichever message
	// it is on stays visible.
	cursor int
	top    int

	// The message box, and what the message in it is for. Typing is reported
	// to the other person, which is why the window remembers what it last
	// said about it.
	input       textinput.Model
	composing   bool
	replyTo     string
	editing     string
	typing      bool
	typingToken int

	// The page named in what is being written, and what the daemon said
	// about it, so the message carries the preview the window would send.
	previewFor string
	preview    model.LinkPreview

	// The prompts laid over the conversation, when there are any: one of
	// several, or something to type.
	chooser *chooser
	prompt  *asking

	// The two numbers the desktop client keeps above its list.
	unreadEverywhere int
	archivedChats    int

	// A message being gone back to, and how many pages have been read
	// looking for it.
	seeking      string
	seekingPages int

	loadingOlder   bool
	waitingHistory bool
	askedHistory   map[string]bool

	events <-chan rpc.Event
}

func newApp(ctx context.Context, d socket, profile string) *app {
	if d == nil {
		d = noDaemon{}
	}
	return &app{ctx: ctx, daemon: d, profile: profile, width: 80, height: 24,
		askedHistory: map[string]bool{}, input: newComposer()}
}

func (a *app) Init() tea.Cmd {
	return tea.Batch(a.loadStatus(), a.loadChats(), a.counts(), a.nextEvent())
}

func (a *app) loadStatus() tea.Cmd {
	return func() tea.Msg {
		var s status
		if err := a.daemon.call(a.ctx, "status.get", nil, &s); err != nil {
			return failureMsg{err}
		}
		return statusMsg(s)
	}
}

func (a *app) loadChats() tea.Cmd {
	return func() tea.Msg {
		var chats []model.Chat
		if err := a.daemon.call(a.ctx, "chats.list", map[string]any{"limit": 200}, &chats); err != nil {
			return failureMsg{err}
		}
		return chatsMsg(chats)
	}
}

// quiet makes a call whose answer the window has no use for. A read receipt
// that the daemon turned down is not something to put in front of the reader.
func (a *app) quiet(method string, params map[string]any) tea.Cmd {
	return func() tea.Msg {
		_ = a.daemon.call(a.ctx, method, params, nil)
		return nil
	}
}

// nextEvent waits for one announcement and asks for the next, which is how a
// channel is read inside this kind of program without blocking the interface.
func (a *app) nextEvent() tea.Cmd {
	if a.events == nil {
		a.events = a.daemon.follow(a.ctx)
	}
	return func() tea.Msg {
		evt, ok := <-a.events
		if !ok {
			return nil
		}
		return eventMsg(evt)
	}
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	case statusMsg:
		a.status = status(msg)
	case chatsMsg:
		a.chats = []model.Chat(msg)
		if a.selected >= len(a.chats) {
			a.selected = max(0, len(a.chats)-1)
		}
	case failureMsg:
		a.failure = safeLine(msg.err.Error())
	case pageMsg:
		return a, a.page(msg)
	case oneMessageMsg:
		if msg.generation == a.generation && msg.chat == a.open.JID && msg.message.ID != "" {
			a.upsert(msg.message)
		}
	case openedMsg:
		if msg.generation == a.generation && msg.chat == a.open.JID && msg.message.ID != "" {
			a.upsert(msg.message)
			return a, a.handOverMedia(msg.message, msg.play)
		}
	case handedOverMsg:
		a.failure = "Opened " + safeLine(msg.what) + " in another program."
	case contactsMsg:
		return a, a.showContacts(msg)
	case stickersMsg:
		return a, a.showStickers(msg)
	case previewMsg:
		// Only while that page is still named in what is being written.
		if a.previewFor == msg.address && strings.Contains(a.input.Value(), msg.address) {
			a.preview = msg.preview
		}
	case countsMsg:
		a.unreadEverywhere, a.archivedChats = msg.unread, msg.archived
	case infoMsg:
		return a, a.showInfo(model.ChatInfo(msg))
	case sharedMsg:
		return a, a.showShared(msg)
	case labelsMsg:
		return a, a.showLabels(msg)
	case foundChatsMsg:
		a.chats = msg.chats
		a.selected = 0
		a.listing = msg.query
	case onDateMsg:
		if msg.chat != a.open.JID || msg.messageID == "" {
			a.failure = "Nothing was said on that day in this conversation."
			return a, nil
		}
		a.focus = paneConversation
		a.composing = false
		a.input.Blur()
		return a, a.seek(msg.messageID)
	case exportedMsg:
		a.failure = "Written to " + msg.path
	case replaceFileMsg:
		return a, a.exportPrompt(msg.chat, true)
	case closeOpenMsg:
		a.open, a.messages = model.Chat{}, nil
		a.cursor, a.top = 0, 0
		a.focus = paneChats
		a.composing = false
		a.input.Blur()
	case revisionsMsg:
		return a, a.showRevisions([]model.MessageRevision(msg))
	case starredMsg:
		return a, a.showStarred([]model.Message(msg))
	case typingMsg:
		// Only the last keystroke's wait reports anything. The same pause is
		// when a page named in the message is looked up, for the same reason:
		// not on every letter struck.
		if int(msg) == a.typingToken {
			return a, tea.Batch(a.reportTyping(a.composing && strings.TrimSpace(a.input.Value()) != ""),
				a.linkPreview(a.input.Value()))
		}
	case eventMsg:
		// Whatever the announcement asks for, listening goes on: a client
		// that stops after one goes quiet for the rest of the session.
		return a, tea.Batch(a.announcement(rpc.Event(msg)), a.nextEvent())
	case tea.KeyMsg:
		return a, a.key(msg)
	}
	return a, nil
}

// page takes a page of messages that arrived, if it is still about the
// conversation that asked for it.
func (a *app) page(msg pageMsg) tea.Cmd {
	if msg.generation != a.generation || msg.chat != a.open.JID {
		return nil
	}
	switch msg.mode {
	case pageOpen, pageReset:
		a.messages = msg.page.Messages
		a.unreadFrom, a.unreadCount = msg.page.FirstUnreadID, msg.page.UnreadCount
		// A conversation opens at its newest message, as it does in the
		// desktop client.
		a.cursor, a.top = max(0, len(a.messages)-1), 0
	case pageOlder:
		a.loadingOlder, a.waitingHistory = false, false
		// Older messages go in above, so the message the keyboard was on is
		// now further down the conversation. It must not be left pointing at
		// a different one.
		a.cursor += a.prepend(msg.page.Messages)
	}
	a.hasMore = msg.page.HasMore
	a.nextBefore, a.nextBeforeID = msg.page.NextBefore, msg.page.NextBeforeID
	if msg.mode == pageOpen {
		return a.acknowledgeOpen()
	}
	if a.seeking != "" {
		return a.seek(a.seeking)
	}
	if !a.hasMore {
		// The daemon has served everything it holds. Whether the account has
		// more is a question only the account can answer.
		return a.requestHistory()
	}
	return nil
}

// announcement is what the daemon says happened, which is the same stream the
// desktop client follows, so both windows answer to the same moment.
func (a *app) announcement(evt rpc.Event) tea.Cmd {
	switch evt.Event {
	case "message.upsert":
		var message model.Message
		if !eventData(evt, &message) || message.ChatJID != a.open.JID || a.open.JID == "" {
			return a.loadChats()
		}
		wasNewest := a.cursor == len(a.messages)-1
		a.upsert(message)
		// Somebody reading the newest message follows the conversation as it
		// arrives; somebody who had scrolled back stays where they were.
		if wasNewest {
			a.cursor = len(a.messages) - 1
		}
		return tea.Batch(a.acknowledgeIncoming(message), a.loadChats())
	case "message.receipt":
		var receipt struct {
			ChatJID   string   `json:"chat_jid"`
			IDs       []string `json:"message_ids"`
			Status    string   `json:"status"`
			Timestamp int64    `json:"timestamp"`
		}
		if eventData(evt, &receipt) && receipt.ChatJID == a.open.JID && a.open.JID != "" {
			a.applyReceipt(receipt.IDs, receipt.Status, receipt.Timestamp)
		}
		return a.loadChats()
	case "message.revoked", "message.reaction", "message.edited", "message.pinned":
		var about struct {
			ChatJID   string `json:"chat_jid"`
			MessageID string `json:"message_id"`
		}
		if eventData(evt, &about) && about.ChatJID == a.open.JID && a.open.JID != "" {
			return a.refreshOne(about.MessageID)
		}
	case "message.starred":
		var about struct {
			ChatJID   string `json:"chat_jid"`
			MessageID string `json:"message_id"`
			Starred   bool   `json:"starred"`
		}
		// A star set on the phone reaches the window only as this, so the open
		// conversation has to answer to it and not only to its own keys.
		if eventData(evt, &about) && about.ChatJID == a.open.JID && a.open.JID != "" {
			a.applyStar(about.MessageID, about.Starred)
		}
	case "chat.updated", "chat.deleted", "chat.cleared", "directory.synced", "labels.updated":
		return a.loadChats()
	case "history.synced":
		var synced struct {
			ChatJIDs []string `json:"chat_jids"`
		}
		asks := []tea.Cmd{a.loadChats()}
		if eventData(evt, &synced) && a.open.JID != "" {
			for _, jid := range synced.ChatJIDs {
				if jid == a.open.JID {
					asks = append(asks, a.refreshOpen())
					break
				}
			}
		}
		if a.waitingHistory && a.open.JID != "" && !a.loadingOlder {
			// The page this was asked for is now the daemon's to serve.
			a.waitingHistory, a.loadingOlder = false, true
			asks = append(asks, a.loadPage(a.open.JID, a.generation, a.nextBefore, a.nextBeforeID, pageSize, pageOlder))
		}
		return tea.Batch(asks...)
	case "connection.changed", "pairing.success", "profile.changed":
		return a.loadStatus()
	case "daemon.error":
		var about struct {
			Message string `json:"message"`
		}
		if eventData(evt, &about) && about.Message != "" {
			a.failure = safeLine(about.Message)
		}
	}
	return nil
}

func (a *app) key(msg tea.KeyMsg) tea.Cmd {
	// Control-C always quits. While the message box has the keyboard, every
	// other key is part of a message - including q.
	if msg.Type == tea.KeyCtrlC {
		return tea.Quit
	}
	// A prompt has the keyboard while it is up, so that j and k move through
	// it rather than through what is behind it.
	if a.prompt != nil {
		return a.promptKey(msg)
	}
	if a.chooser != nil {
		return a.chooserKey(msg)
	}
	if a.composing {
		return a.composerKey(msg)
	}
	switch msg.String() {
	case "q":
		return tea.Quit
	case "?":
		return a.help()
	case "m":
		return a.chatMenu()
	case "/":
		return a.searchPrompt()
	case "tab":
		if a.focus == paneChats {
			a.focus = paneConversation
		} else {
			a.focus = paneChats
		}
		return nil
	}
	if a.focus == paneChats {
		return a.chatsKey(msg)
	}
	return a.conversationKey(msg)
}

func (a *app) chatsKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "down", "j":
		if a.selected < len(a.chats)-1 {
			a.selected++
		}
	case "up", "k":
		if a.selected > 0 {
			a.selected--
		}
	case "enter", "l", "right":
		if chat, ok := a.selectedChat(); ok {
			// Opening a conversation leaves the keyboard in the message box,
			// which is where the desktop client leaves it: clicking a
			// conversation in the sidebar is usually the start of answering
			// it.
			return tea.Batch(a.openChat(chat), a.enterComposer())
		}
	case "r":
		return tea.Batch(a.loadStatus(), a.loadChats(), a.counts())
	case "A":
		return a.archived()
	case "M":
		return a.markAllReadPrompt()
	}
	return nil
}

func (a *app) conversationKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "h", "left", "esc":
		a.focus = paneChats
	case "up", "k":
		// Moving off the top of what is loaded is what asks for more.
		if a.cursor > 0 {
			a.cursor--
			return nil
		}
		return a.loadOlder()
	case "down", "j":
		if a.cursor < len(a.messages)-1 {
			a.cursor++
		}
	case "g", "home":
		a.cursor = 0
		return a.loadOlder()
	case "G", "end":
		a.cursor = max(0, len(a.messages)-1)
	case "i", "enter":
		return a.enterComposer()
	case "r":
		return a.replyToSelected()
	case "e":
		return a.editSelected()
	case "o":
		return a.openSelected()
	case "a":
		return a.attachMenu()
	case "x":
		return a.reactPrompt()
	case "s":
		return a.toggleStar()
	case "p":
		return a.pinPrompt()
	case "f":
		return a.forwardPrompt()
	case "d":
		return a.deletePrompt()
	case "v":
		return a.revisions()
	case "S":
		return a.starred()
	case "ctrl+r":
		return a.refreshOpen()
	}
	return nil
}

// selectedChat is the conversation the keyboard is on, if there is one.
func (a *app) selectedChat() (model.Chat, bool) {
	if a.selected < 0 || a.selected >= len(a.chats) {
		return model.Chat{}, false
	}
	return a.chats[a.selected], true
}

func (a *app) View() string {
	listWidth := a.width / 3
	if listWidth < 24 {
		listWidth = 24
	}
	if listWidth > a.width-20 {
		listWidth = max(12, a.width-20)
	}
	paneWidth := max(12, a.width-listWidth-1)
	paneHeight := max(1, a.height-2)
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		stylePane.Width(listWidth).Height(paneHeight).Render(anchored(fitted(a.chatList(listWidth, paneHeight), listWidth))),
		lipgloss.NewStyle().Width(paneWidth).Height(paneHeight).Render(anchored(fitted(a.conversation(paneWidth, paneHeight), paneWidth))),
	)
	// A line wider than the terminal is wrapped by the terminal, which pushes
	// every row below it down and the title off the top.
	return strings.Join([]string{
		anchored(ansi.Truncate(a.header(), a.width, "…")),
		body,
		anchored(ansi.Truncate(a.footer(), a.width, "…")),
	}, "\n")
}

func (a *app) header() string {
	where := safeLine(a.status.State)
	if where == "" {
		where = "connecting"
	}
	who := safeLine(a.status.UserName)
	if who == "" {
		who = a.profile
	}
	said := fmt.Sprintf("  %s  ·  %s", who, where)
	if a.unreadEverywhere > 0 {
		said += fmt.Sprintf("  ·  %s waiting", plural(a.unreadEverywhere, "message", "messages"))
	}
	if a.archivedChats > 0 {
		said += fmt.Sprintf("  ·  %d archived", a.archivedChats)
	}
	if a.listing != "" {
		said += "  ·  " + safeLine(a.listing)
	}
	return styleTitle.Render("WhatsAppGo") + styleMuted.Render(said)
}

// chatList draws as many conversations as the window has rows for. A height
// sets only how tall the pane is at least, so a list longer than the window
// would push the title off the top of the screen. It moves only as far as it
// takes to show the conversation the keyboard is on.
func (a *app) chatList(width, height int) string {
	if len(a.chats) == 0 {
		return styleMuted.Render("No conversations yet")
	}
	top := min(a.chatTop, max(0, len(a.chats)-height))
	switch {
	case a.selected < top:
		top = max(0, a.selected)
	case a.selected >= top+height:
		top = a.selected - height + 1
	}
	a.chatTop = top
	end := min(len(a.chats), top+height)
	lines := make([]string, 0, end-top)
	for i := top; i < end; i++ {
		chat := a.chats[i]
		title := safeLine(chat.Title)
		if title == "" {
			title = safeLine(shortJID(chat.JID))
		}
		unread := ""
		if chat.UnreadCount > 0 {
			unread = fmt.Sprintf(" (%d)", chat.UnreadCount)
		}
		// How many are waiting is the one part of a row that must not be the
		// part that falls off the end, so the name gives up the room for it.
		title = fitLine(title, width-2-ansi.StringWidth(unread)) + unread
		switch {
		case i == a.selected && a.focus == paneChats:
			lines = append(lines, styleSelected.Render(title))
		case chat.JID == a.open.JID && a.open.JID != "":
			lines = append(lines, styleTitle.Render(title))
		default:
			lines = append(lines, title)
		}
	}
	return strings.Join(lines, "\n")
}

// conversation draws the open conversation, with the title above it and the
// message box below. The newest message sits at the bottom, as it does in the
// desktop client, and the window holds whichever message the keyboard is on.
func (a *app) conversation(width, height int) string {
	if a.prompt != nil {
		return a.promptLines(width)
	}
	if a.chooser != nil {
		return a.chooserLines(width, height)
	}
	lines, from, to := a.conversationLines(width)
	box := a.composerLines(width)
	room := max(1, height-1-len(box))
	// Start at the bottom, then move only as far as it takes to show the
	// message the keyboard is on.
	a.top = max(0, len(lines)-room)
	if to >= 0 && to < a.top {
		a.top = from
	}
	end := min(len(lines), a.top+room)
	rows := append([]string{a.conversationTitle(width)}, lines[a.top:end]...)
	for len(rows) < height-len(box) {
		rows = append(rows, "")
	}
	return strings.Join(append(rows, box...), "\n")
}

func (a *app) footer() string {
	if a.failure != "" {
		// What went wrong is often quoted back from the daemon, and a daemon
		// answer can carry an account's own text inside it.
		return styleDanger.Render(safeLine(a.failure))
	}
	if a.prompt != nil {
		return styleMuted.Render("enter accept · esc back")
	}
	if a.chooser != nil {
		return styleMuted.Render("enter choose · j/k move · 1-9 pick · esc back")
	}
	if a.composing {
		if a.editing != "" || a.replyTo != "" {
			return styleMuted.Render("enter send · esc drop the answer · tab list · ctrl+c quit")
		}
		return styleMuted.Render("enter send · esc stop typing · tab list · ctrl+c quit")
	}
	if a.focus == paneConversation {
		return styleMuted.Render("i write · o open · a attach · r answer · x react · ? keys · q quit")
	}
	return styleMuted.Render("enter open · m menu · / find · A archive · ? keys · q quit")
}
