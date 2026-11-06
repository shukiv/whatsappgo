package main

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shukiv/whatsappgo/internal/model"
)

// How long after the last keystroke the other person is told that somebody is
// typing. The desktop client waits the same 700ms, so neither client reports a
// single letter as it is struck.
const typingSettles = 700 * time.Millisecond

// typingMsg is a settled pause, carrying the keystroke it was started for so
// that only the last one counts.
type typingMsg int

func newComposer() textinput.Model {
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "Type a message"
	input.CharLimit = 4096
	return input
}

// enterComposer puts the keyboard in the message box. Opening a conversation
// does this, which is what the desktop client does: clicking a conversation in
// the sidebar leaves the cursor in the message box, ready to answer.
func (a *app) enterComposer() tea.Cmd {
	a.focus = paneConversation
	a.composing = true
	return a.input.Focus()
}

// leaveComposer gives the keyboard back to the conversation, and takes back
// what was said about typing: somebody who stopped to read is not typing.
func (a *app) leaveComposer() tea.Cmd {
	a.composing = false
	a.input.Blur()
	return a.reportTyping(false)
}

func (a *app) composerKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		return a.send()
	case "esc":
		// The first press drops what the message was going to be an answer to
		// or a change of; the second gives the keyboard back.
		if a.replyTo != "" || a.editing != "" {
			a.replyTo, a.editing = "", ""
			a.input.SetValue("")
			return nil
		}
		return a.leaveComposer()
	case "tab":
		left := a.leaveComposer()
		a.focus = paneChats
		return left
	case "ctrl+o":
		return a.attachMenu()
	case "ctrl+g":
		return a.openEmoji()
	}
	var moved tea.Cmd
	a.input, moved = a.input.Update(msg)
	return tea.Batch(moved, a.noteTyping())
}

// noteTyping starts the wait after a keystroke. Each keystroke starts its own,
// and only the last one to be started reports anything, so a pause is a pause
// in the typing rather than in one key.
func (a *app) noteTyping() tea.Cmd {
	a.typingToken++
	token := a.typingToken
	return tea.Tick(typingSettles, func(time.Time) tea.Msg { return typingMsg(token) })
}

// reportTyping tells the conversation whether somebody here is typing, and
// says so only when the answer has changed. Nothing is reported for a channel,
// which has nobody to tell, or before a conversation is open.
func (a *app) reportTyping(typing bool) tea.Cmd {
	if a.open.JID == "" || strings.HasSuffix(a.open.JID, "@newsletter") {
		a.typing = false
		return nil
	}
	if a.typing == typing {
		return nil
	}
	a.typing = typing
	return a.quiet("chat.typing", map[string]any{"chat_jid": a.open.JID, "typing": typing})
}

// send is enter in the message box: a new message, an answer to one, or a
// change to one already sent.
func (a *app) send() tea.Cmd {
	text := strings.TrimSpace(a.input.Value())
	if text == "" || a.open.JID == "" {
		// An empty box is not a message. The desktop client sends nothing
		// here either.
		return nil
	}
	chat, generation := a.open.JID, a.generation
	if edited := a.editing; edited != "" {
		a.input.SetValue("")
		a.editing, a.replyTo = "", ""
		return tea.Batch(a.reportTyping(false), func() tea.Msg {
			if err := a.daemon.call(a.ctx, "message.edit", map[string]any{
				"chat_jid": chat, "message_id": edited, "text": text}, nil); err != nil {
				return failureMsg{err}
			}
			// The daemon announces the change, and that announcement is what
			// puts the new wording on screen, in both clients.
			return nil
		})
	}
	params := map[string]any{"chat_jid": chat, "text": text, "reply_to": a.replyTo}
	// The preview goes only when the page it was resolved for is still named
	// in the message, which is the check the window makes too.
	if a.preview.URL != "" && strings.Contains(text, a.preview.URL) {
		params["link_preview"] = a.preview
	}
	a.input.SetValue("")
	a.replyTo = ""
	a.previewFor, a.preview = "", model.LinkPreview{}
	return tea.Batch(a.reportTyping(false), func() tea.Msg {
		var sent model.Message
		if err := a.daemon.call(a.ctx, "message.send", params, &sent); err != nil {
			return failureMsg{err}
		}
		return oneMessageMsg{generation: generation, chat: chat, message: sent}
	})
}

// replyToSelected starts an answer to the message the keyboard is on.
func (a *app) replyToSelected() tea.Cmd {
	message, ok := a.selectedMessage()
	if !ok || message.Revoked {
		return nil
	}
	a.replyTo, a.editing = message.ID, ""
	return a.enterComposer()
}

// editSelected starts a change to a message sent from here. Only one of ours
// can be changed, and not one that has been deleted.
func (a *app) editSelected() tea.Cmd {
	message, ok := a.selectedMessage()
	if !ok || !message.FromMe || message.Revoked {
		return nil
	}
	a.editing, a.replyTo = message.ID, ""
	a.input.SetValue(editableText(message.Body))
	a.input.CursorEnd()
	return a.enterComposer()
}

// composerLines are the rows at the foot of the conversation: what the message
// will answer or change, and the box itself.
func (a *app) composerLines(width int) []string {
	lines := make([]string, 0, 3)
	if a.preview.Title != "" || a.preview.URL != "" {
		shown := safeLine(a.preview.Title)
		if shown == "" {
			shown = safeLine(a.preview.URL)
		}
		lines = append(lines, styleMuted.Render(fitLine("link: "+shown, width)))
	}
	switch {
	case a.editing != "":
		lines = append(lines, styleMuted.Render(fitLine("changing: "+a.quoted(a.editing), width)))
	case a.replyTo != "":
		lines = append(lines, styleMuted.Render(fitLine("answering: "+a.quoted(a.replyTo), width)))
	}
	return append(lines, a.composerBar(width)...)
}

// The message bar is three rows of the desktop client's input bar: the
// paperclip and the face on the left, the words in the middle, and the
// arrow that sends them on the right once there are any.
const (
	composerHeight = 3
	barLeft        = 7 // " 📎 🙂 "
	barRight       = 4 // " ➤  "
)

// composerBar is the bar itself. One row of words sits on its middle row,
// beside the buttons; more than that fill it from the top.
func (a *app) composerBar(width int) []string {
	bar := lipgloss.NewStyle().Background(colourBar)
	room := max(4, width-barLeft-barRight)
	words := inputRows(a.input.Value(), a.input.Position(), room, composerHeight, a.input.Placeholder, a.composing, bar)
	top := 0
	if len(words) == 1 {
		top = 1
	}
	rows := make([]string, composerHeight)
	for row := range rows {
		left, middle, right := spread(bar, barLeft), spread(bar, room), spread(bar, barRight)
		if row == 1 {
			left = bar.Render(" 📎 🙂 ")
			if strings.TrimSpace(a.input.Value()) != "" {
				right = bar.Foreground(colourBrand).Bold(true).Render(" ➤") + spread(bar, barRight-2)
			}
		}
		if at := row - top; at >= 0 && at < len(words) {
			middle = words[at]
		}
		rows[row] = left + middle + right
	}
	return rows
}

// barButton is what a click at a column of the message bar presses: the
// paperclip, the face, the arrow, or the words.
func (a *app) barButton(column, width int) string {
	switch {
	case column < 4:
		return "attach"
	case column < barLeft:
		return "emoji"
	case column >= width-barRight && strings.TrimSpace(a.input.Value()) != "":
		return "send"
	}
	return "words"
}

// quoted names a message the way the box above it refers to one.
func (a *app) quoted(id string) string {
	at := a.indexOf(id)
	if at < 0 {
		return "a message"
	}
	message := a.messages[at]
	who := "You"
	if !message.FromMe {
		if who = safeLine(message.SenderName); who == "" {
			who = safeLine(shortJID(message.SenderJID))
		}
	}
	said := safeLine(messageBody(message))
	if said == "" {
		said = "a message"
	}
	return who + ": " + said
}
