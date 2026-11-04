package main

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	daemon  *daemon
	profile string

	width, height int
	focus         pane
	status        status
	chats         []model.Chat
	selected      int
	failure       string

	events <-chan rpc.Event
}

func newApp(ctx context.Context, d *daemon, profile string) *app {
	return &app{ctx: ctx, daemon: d, profile: profile, width: 80, height: 24}
}

func (a *app) Init() tea.Cmd {
	return tea.Batch(a.loadStatus(), a.loadChats(), a.nextEvent())
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
		a.failure = msg.err.Error()
	case eventMsg:
		// A conversation changed or a message arrived: ask for the list
		// again rather than guessing what moved, and keep listening.
		switch msg.Event {
		case "chat.updated", "message.upsert":
			return a, tea.Batch(a.loadChats(), a.nextEvent())
		}
		return a, a.nextEvent()
	case tea.KeyMsg:
		return a, a.key(msg)
	}
	return a, nil
}

func (a *app) key(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c", "q":
		return tea.Quit
	case "tab":
		if a.focus == paneChats {
			a.focus = paneConversation
		} else {
			a.focus = paneChats
		}
	case "down", "j":
		if a.focus == paneChats && a.selected < len(a.chats)-1 {
			a.selected++
		}
	case "up", "k":
		if a.focus == paneChats && a.selected > 0 {
			a.selected--
		}
	case "r":
		return tea.Batch(a.loadStatus(), a.loadChats())
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
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		stylePane.Width(listWidth).Height(a.height-2).Render(a.chatList(listWidth)),
		lipgloss.NewStyle().Width(a.width-listWidth-1).Height(a.height-2).Render(a.conversation()),
	)
	return strings.Join([]string{a.header(), body, a.footer()}, "\n")
}

func (a *app) header() string {
	where := a.status.State
	if where == "" {
		where = "connecting"
	}
	who := a.status.UserName
	if who == "" {
		who = a.profile
	}
	return styleTitle.Render("WhatsAppGo") + styleMuted.Render(fmt.Sprintf("  %s  ·  %s", who, where))
}

func (a *app) chatList(width int) string {
	if len(a.chats) == 0 {
		return styleMuted.Render("No conversations yet")
	}
	lines := make([]string, 0, len(a.chats))
	for i, chat := range a.chats {
		title := chat.Title
		if title == "" {
			title = chat.JID
		}
		if len(title) > width-2 {
			title = title[:max(0, width-3)] + "…"
		}
		if chat.UnreadCount > 0 {
			title += fmt.Sprintf(" (%d)", chat.UnreadCount)
		}
		if i == a.selected && a.focus == paneChats {
			lines = append(lines, styleSelected.Render(title))
			continue
		}
		lines = append(lines, title)
	}
	return strings.Join(lines, "\n")
}

func (a *app) conversation() string {
	chat, ok := a.selectedChat()
	if !ok {
		return styleMuted.Render("Pick a conversation")
	}
	return styleTitle.Render(chat.Title) + "\n" + styleMuted.Render(chat.LastMessagePreview)
}

func (a *app) footer() string {
	if a.failure != "" {
		return styleDanger.Render(a.failure)
	}
	return styleMuted.Render("tab switch · j/k move · r refresh · q quit")
}
