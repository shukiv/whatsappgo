package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shukiv/whatsappgo/internal/model"
)

// Nothing is drawn in the terminal. A picture, a video, a sticker, a voice
// note or a file is named where it sits in the conversation, and opening one
// hands the file to the program that handles its kind - which is what the
// reader's desktop already decides for every other program.
//
// The file is passed as an argument to that program and never through a shell,
// so a name holding a quote, a semicolon or a newline is a name and not a
// command. Nothing here builds a command line out of text from the account.
func opener() []string {
	if set := strings.TrimSpace(os.Getenv("WHATSAPPGO_OPENER")); set != "" {
		return []string{set}
	}
	switch runtime.GOOS {
	case "darwin":
		return []string{"open"}
	case "windows":
		return []string{"rundll32", "url.dll,FileProtocolHandler"}
	}
	return []string{"xdg-open"}
}

// handOver starts the program for one file or address and leaves it running.
// Its output goes nowhere: a program that wrote to this terminal would draw
// over the conversation.
func handOver(target string) error {
	if strings.TrimSpace(target) == "" {
		return errors.New("there is nothing here to open")
	}
	parts := opener()
	command := exec.Command(parts[0], append(parts[1:], target)...)
	quiet, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer quiet.Close()
	command.Stdin, command.Stdout, command.Stderr = quiet, quiet, quiet
	if err := command.Start(); err != nil {
		return err
	}
	// Nobody waits for it: the reader goes on reading while the picture opens.
	return command.Process.Release()
}

// What a file has to be before it is handed over.
func readableFile(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("the daemon gave a path that is not absolute: %q", path)
	}
	about, err := os.Stat(path)
	if err != nil {
		return err
	}
	if about.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	return nil
}

// Answers that arrive from opening something.
type (
	openedMsg struct {
		generation int
		chat       string
		message    model.Message
		play       bool
	}
	handedOverMsg struct{ what string }
)

// openSelected is what the reader asks for with o: whatever the message under
// the cursor holds, opened the way its kind is opened. A message of words has
// nothing to open.
func (a *app) openSelected() tea.Cmd {
	message, ok := a.selectedMessage()
	if !ok || message.Revoked {
		return nil
	}
	if message.LinkURL != "" && mediaPlaceholder(message) == "" {
		return a.linkMenu(message)
	}
	switch message.Kind {
	case "image", "video", "audio", "sticker", "document", "view_once":
		return a.openMedia(message)
	case "location":
		if message.Latitude == 0 && message.Longitude == 0 {
			return nil
		}
		return a.openAddress(fmt.Sprintf("geo:%.6f,%.6f", message.Latitude, message.Longitude))
	}
	if message.LinkURL != "" {
		return a.linkMenu(message)
	}
	a.failure = "There is nothing to open in that message."
	return nil
}

// openMedia hands the file over, downloading it first if the daemon has not
// got it yet. A voice note or a video that somebody else sent is reported as
// played once it has been handed to the player, which is the moment it is.
func (a *app) openMedia(message model.Message) tea.Cmd {
	play := !message.FromMe && (message.Kind == "audio" || message.Kind == "video")
	if message.MediaPath != "" {
		return a.handOverMedia(message, play)
	}
	chat, generation, id := a.open.JID, a.generation, message.ID
	return func() tea.Msg {
		var fetched model.Message
		if err := a.daemon.call(a.ctx, "message.download",
			map[string]any{"chat_jid": chat, "message_id": id}, &fetched); err != nil {
			return failureMsg{err}
		}
		return openedMsg{generation: generation, chat: chat, message: fetched, play: play}
	}
}

// handOverMedia is the file itself going to another program.
func (a *app) handOverMedia(message model.Message, play bool) tea.Cmd {
	path := message.MediaPath
	asks := []tea.Cmd{func() tea.Msg {
		if err := readableFile(path); err != nil {
			return failureMsg{err}
		}
		if err := handOver(path); err != nil {
			return failureMsg{err}
		}
		return handedOverMsg{what: filepath.Base(path)}
	}}
	if play && message.PlayedAt == 0 {
		asks = append(asks, a.quiet("message.played", map[string]any{
			"chat_jid": a.open.JID, "sender_jid": message.SenderJID,
			"message_id": message.ID, "timestamp": model.NowMillis(),
		}))
	}
	return tea.Batch(asks...)
}

// The schemes a conversation legitimately carries, which are the ones the
// window follows too. A link was written by whoever sent it, and the program
// it is handed to picks what to run by its scheme - a desktop can register a
// handler for almost anything - so any other scheme could start a program of
// the sender's choosing. A path on this computer is not in the list either.
var followedSchemes = map[string]bool{"http": true, "https": true, "mailto": true, "tel": true, "geo": true}

// followable is the address as it is handed over, and whether it may be.
func followable(address string) (string, bool) {
	address = strings.TrimSpace(address)
	target, err := url.Parse(address)
	if err != nil {
		return "", false
	}
	return address, followedSchemes[target.Scheme]
}

// openAddress refuses an address it would not follow before anything is
// started, so a refused one never reaches another program.
func (a *app) openAddress(address string) tea.Cmd {
	address, followed := followable(address)
	if !followed {
		a.failure = "That link is not a web address, so it was not opened."
		return nil
	}
	return func() tea.Msg {
		if err := handOver(address); err != nil {
			return failureMsg{err}
		}
		return handedOverMsg{what: address}
	}
}

// linkMenu is what to do with a message carrying a page: open it, or ask the
// daemon for the full preview where only a small cached one arrived.
func (a *app) linkMenu(message model.Message) tea.Cmd {
	address := message.LinkURL
	return a.ask("Open this link?", safeLine(address), 0,
		choice{label: "open the page", run: func() tea.Cmd { return a.openAddress(address) }},
		choice{label: "fetch the full preview", run: func() tea.Cmd {
			return a.quiet("link.preview.refresh", map[string]any{
				"chat_jid": a.open.JID, "message_id": message.ID})
		}})
}

// openAvatar hands a conversation's picture over, asking the daemon for it
// first. It is a picture like any other: named, not drawn.
func (a *app) openAvatar(chat model.Chat) tea.Cmd {
	return func() tea.Msg {
		var fetched struct {
			Path string `json:"avatar_path"`
		}
		if err := a.daemon.call(a.ctx, "chat.avatar",
			map[string]any{"chat_jid": chat.JID, "refresh": false}, &fetched); err != nil {
			return failureMsg{err}
		}
		if fetched.Path == "" {
			return failureMsg{errors.New("this conversation has no picture")}
		}
		if err := readableFile(fetched.Path); err != nil {
			return failureMsg{err}
		}
		if err := handOver(fetched.Path); err != nil {
			return failureMsg{err}
		}
		return handedOverMsg{what: filepath.Base(fetched.Path)}
	}
}

// attachMenu is what can be put into the conversation besides words.
func (a *app) attachMenu() tea.Cmd {
	if a.open.JID == "" {
		return nil
	}
	return a.ask("Put into this conversation", "", 0,
		choice{label: "a file", run: a.sendFilePrompt},
		choice{label: "a contact card", run: a.sendContactPrompt},
		choice{label: "a sticker sent before", run: a.stickerPrompt})
}

// sendFilePrompt asks for a file and then how to send it, because the same
// file is a picture, a document or a voice note depending on what it is being
// sent as - which is the choice the window's attach menu offers.
func (a *app) sendFilePrompt() tea.Cmd {
	chat := a.open.JID
	return a.type_("Which file?", "The whole path to it.", "", func(path string) tea.Cmd {
		if path == "" {
			return nil
		}
		if err := readableFile(path); err != nil {
			a.failure = safeLine(err.Error())
			return nil
		}
		send := func(as string) func() tea.Cmd {
			return func() tea.Cmd {
				return a.type_("A caption?", filepath.Base(path), "", func(caption string) tea.Cmd {
					params := map[string]any{"chat_jid": chat, "path": path, "caption": caption, "reply_to": a.replyTo}
					if as != "" {
						params[as] = true
					}
					a.replyTo = ""
					return func() tea.Msg {
						var sent model.Message
						if err := a.daemon.call(a.ctx, "message.send_media", params, &sent); err != nil {
							return failureMsg{err}
						}
						return oneMessageMsg{generation: a.generation, chat: chat, message: sent}
					}
				})
			}
		}
		return a.ask("Send it as what?", safeLine(filepath.Base(path)), 0,
			choice{label: "a picture or a video", run: send("")},
			choice{label: "a file", run: send("document")},
			choice{label: "a voice note", run: send("voice")},
			choice{label: "a moving picture", run: send("gif")},
			choice{label: "a sticker", run: send("sticker")})
	})
}

// sendContactPrompt picks somebody out of the conversations that have a number
// and sends their name and number as a card, which is what the window's
// contact picker does.
func (a *app) sendContactPrompt() tea.Cmd {
	chat := a.open.JID
	return func() tea.Msg {
		var people []model.Contact
		if err := a.daemon.call(a.ctx, "contacts.shareable", map[string]any{"query": ""}, &people); err != nil {
			return failureMsg{err}
		}
		return contactsMsg{chat: chat, people: people}
	}
}

type contactsMsg struct {
	chat   string
	people []model.Contact
}

func (a *app) showContacts(found contactsMsg) tea.Cmd {
	if len(found.people) == 0 {
		return a.ask("Whose card?", "No contact here has a number to send.", 0, choice{label: "close"})
	}
	items := make([]choice, 0, len(found.people))
	for _, person := range found.people {
		name, phone := safeLine(person.Name), safeLine(person.Phone)
		items = append(items, choice{label: name + "  " + phone, run: func() tea.Cmd {
			return func() tea.Msg {
				var sent model.Message
				if err := a.daemon.call(a.ctx, "message.send_contact", map[string]any{
					"chat_jid": found.chat, "reply_to": "",
					"contact": map[string]any{"name": name, "phone": phone}}, &sent); err != nil {
					return failureMsg{err}
				}
				return oneMessageMsg{generation: a.generation, chat: found.chat, message: sent}
			}
		}})
	}
	return a.ask("Whose card?", plural(len(found.people), "contact", "contacts"), 0, items...)
}

// stickerPrompt reuses a sticker this account has sent or received before,
// which is the only way to send one: the original is what goes, not a copy
// made here.
func (a *app) stickerPrompt() tea.Cmd {
	chat := a.open.JID
	return func() tea.Msg {
		var page model.SharedMessagePage
		if err := a.daemon.call(a.ctx, "media.shared",
			map[string]any{"category": "stickers", "offset": 0, "limit": pageSize}, &page); err != nil {
			return failureMsg{err}
		}
		return stickersMsg{chat: chat, page: page}
	}
}

type stickersMsg struct {
	chat string
	page model.SharedMessagePage
}

func (a *app) showStickers(found stickersMsg) tea.Cmd {
	if len(found.page.Messages) == 0 {
		return a.ask("Which sticker?", "No sticker has passed through this account yet.", 0,
			choice{label: "close"})
	}
	items := make([]choice, 0, len(found.page.Messages))
	for _, sticker := range found.page.Messages {
		from, id, replyTo := sticker.ChatJID, sticker.ID, a.replyTo
		label := stamp(sticker.Timestamp)
		if name := safeLine(sticker.MediaName); name != "" {
			label += "  " + name
		}
		if title := safeLine(sticker.ChatTitle); title != "" {
			label += "  from " + title
		}
		items = append(items, choice{label: label, run: func() tea.Cmd {
			a.replyTo = ""
			return a.quiet("sticker.send", map[string]any{
				"chat_jid": from, "message_id": id, "to_chat_jid": found.chat, "reply_to": replyTo})
		}})
	}
	return a.ask("Which sticker?", "the ones this account has", 0, items...)
}

// mediaLibrary is everything shared across every conversation, which the
// window keeps as its own panel.
func (a *app) mediaLibrary() tea.Cmd {
	items := make([]choice, 0, len(sharedCategories))
	for _, category := range sharedCategories {
		category := category
		items = append(items, choice{label: category, run: func() tea.Cmd {
			return func() tea.Msg {
				var page model.SharedMessagePage
				if err := a.daemon.call(a.ctx, "media.shared", map[string]any{
					"category": category, "offset": 0, "limit": pageSize}, &page); err != nil {
					return failureMsg{err}
				}
				return sharedMsg{category: "everywhere: " + category, page: page}
			}
		}})
	}
	return a.ask("Shared across every conversation", "", 0, items...)
}

// linkPreview resolves a page named in the message being written, so the
// message carries the same preview the window would send. The daemon fetches
// it; nothing here reaches the page.
func (a *app) linkPreview(text string) tea.Cmd {
	address := firstAddress(text)
	if address == "" || address == a.previewFor {
		return nil
	}
	a.previewFor = address
	return func() tea.Msg {
		var preview model.LinkPreview
		if err := a.daemon.call(a.ctx, "link.preview", map[string]any{"text": address}, &preview); err != nil {
			return nil
		}
		return previewMsg{address: address, preview: preview}
	}
}

type previewMsg struct {
	address string
	preview model.LinkPreview
}

// firstAddress is the first thing in what is being written that looks like a
// page, which is what the daemon is asked about.
func firstAddress(text string) string {
	for _, word := range strings.Fields(text) {
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			return word
		}
	}
	return ""
}
