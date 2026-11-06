package main

import tea "github.com/charmbracelet/bubbletea"

// hit is what one row of the conversation pane showed when it was drawn: a
// message, an item of a list being offered, or the message box.
type hit struct {
	message  int
	item     int
	composer bool
	// One of the message bar's rows, whose buttons are told apart by column.
	bar bool
}

var nothing = hit{message: -1, item: -1}

// mouse does with a click what the keys would do with the thing clicked on,
// and turns the wheel into moving up and down. A click on a conversation opens
// it, a click on a message puts the keys on it, a click on an item of a list
// chooses it, and a click on the message box starts typing.
func (a *app) mouse(msg tea.MouseMsg) tea.Cmd {
	listWidth, paneWidth, paneHeight := a.layout()
	// The title is the first row of the window, so the panes start on the
	// second.
	row := msg.Y - 1
	if row < 0 || row >= paneHeight || a.prompt != nil {
		return nil
	}
	inList := msg.X < listWidth
	inPane := msg.X > listWidth
	// The emoji panel answers the mouse itself, by where it drew things.
	if a.picker != nil && a.chooser == nil {
		if !inPane {
			return nil
		}
		return a.pickerMouse(msg, row, msg.X-listWidth-1)
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		step := tea.KeyMsg{Type: tea.KeyDown}
		if msg.Button == tea.MouseButtonWheelUp {
			step = tea.KeyMsg{Type: tea.KeyUp}
		}
		switch {
		case a.chooser != nil:
			return a.chooserKey(step)
		case inList:
			return a.chatsKey(step)
		case inPane && a.open.JID != "":
			return a.conversationKey(step)
		}
		return nil
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return nil
		}
	default:
		return nil
	}

	if a.chooser != nil {
		if inPane && row < len(a.hits) && a.hits[row].item >= 0 {
			a.chooser.cursor = a.hits[row].item
			return a.chooseHere()
		}
		return nil
	}
	if inList {
		// The account is above the list and does nothing when clicked. Each
		// conversation is several rows tall, and a click on any of them opens
		// it; the rows left over below the last whole one are nobody's.
		account := a.accountRows(listWidth, paneHeight)
		shown := (row - account) / chatRowHeight
		at := a.chatTop + shown
		if row < account || shown >= chatsShown(paneHeight-account) || at >= len(a.chats) {
			return nil
		}
		a.selected = at
		var left tea.Cmd
		if a.composing {
			left = a.leaveComposer()
		}
		return tea.Batch(left, a.openChat(a.chats[at]), a.enterComposer())
	}
	if !inPane || row >= len(a.hits) || a.open.JID == "" {
		return nil
	}
	switch shown := a.hits[row]; {
	case shown.composer:
		if shown.bar {
			switch a.barButton(msg.X-listWidth-1, paneWidth) {
			case "attach":
				return a.attachMenu()
			case "emoji":
				return a.openEmoji()
			case "send":
				return a.send()
			}
		}
		return a.enterComposer()
	case shown.message >= 0 && shown.message < len(a.messages):
		a.cursor = shown.message
		a.focus = paneConversation
		var left tea.Cmd
		if a.composing {
			left = a.leaveComposer()
		}
		// A click on a voice note plays it, and another stops it.
		if a.messages[shown.message].Kind == "audio" {
			return tea.Batch(left, a.openSelected())
		}
		return left
	}
	return nil
}
