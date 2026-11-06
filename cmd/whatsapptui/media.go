package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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
// played once it has been handed to the player, which is the moment it is. A
// voice note already playing is stopped instead.
func (a *app) openMedia(message model.Message) tea.Cmd {
	if message.Kind == "audio" && a.playing.command != nil && a.playing.id == message.ID {
		a.stopPlaying()
		return nil
	}
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

// handOverMedia is the file itself going to another program. A voice note is
// played here, by a player with no window of its own, when there is one.
func (a *app) handOverMedia(message model.Message, play bool) tea.Cmd {
	path := message.MediaPath
	hand := func() tea.Msg {
		if err := readableFile(path); err != nil {
			return failureMsg{err}
		}
		if err := handOver(path); err != nil {
			return failureMsg{err}
		}
		return handedOverMsg{what: filepath.Base(path)}
	}
	if player := quietPlayer(); message.Kind == "audio" && player != nil {
		hand = playHere(message.ID, path, player)
	}
	asks := []tea.Cmd{hand}
	if play && message.PlayedAt == 0 {
		asks = append(asks, a.quiet("message.played", map[string]any{
			"chat_jid": a.open.JID, "sender_jid": message.SenderJID,
			"message_id": message.ID, "timestamp": model.NowMillis(),
		}))
	}
	return tea.Batch(asks...)
}

// playback is the voice note playing, and the player playing it, which is how
// the same key stops it.
type playback struct {
	id      string
	command *exec.Cmd
}

// Answers from the player: it started, and it finished.
type (
	playingMsg struct {
		id      string
		command *exec.Cmd
	}
	playedOutMsg struct{ command *exec.Cmd }
)

// The players that play a voice note without a window, in the order they are
// tried, so that listening does not take the reader out of the terminal.
var quietPlayers = [][]string{
	{"mpv", "--no-video", "--really-quiet"},
	{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet"},
	{"cvlc", "--play-and-exit", "--quiet"},
	{"pw-play"},
	{"paplay"},
}

func quietPlayer() []string {
	if set := strings.TrimSpace(os.Getenv("WHATSAPPGO_PLAYER")); set != "" {
		return []string{set}
	}
	for _, player := range quietPlayers {
		if _, err := exec.LookPath(player[0]); err == nil {
			return player
		}
	}
	return nil
}

// playHere starts the player on the file. As with any other program here,
// the file is an argument and never part of a command line, and what the
// player writes goes nowhere.
func playHere(id, path string, player []string) tea.Cmd {
	return func() tea.Msg {
		if err := readableFile(path); err != nil {
			return failureMsg{err}
		}
		quiet, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			return failureMsg{err}
		}
		defer quiet.Close()
		command := exec.Command(player[0], append(player[1:], path)...)
		command.Stdin, command.Stdout, command.Stderr = quiet, quiet, quiet
		if err := command.Start(); err != nil {
			return failureMsg{err}
		}
		return playingMsg{id: id, command: command}
	}
}

// listen waits for the player to finish, so the note stops showing as
// playing when it ends on its own.
func listen(command *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		_ = command.Wait()
		return playedOutMsg{command: command}
	}
}

// stopPlaying stops the voice note that is playing, if one is.
func (a *app) stopPlaying() {
	if a.playing.command != nil && a.playing.command.Process != nil {
		_ = a.playing.command.Process.Kill()
	}
	a.playing = playback{}
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

// attachMenu is what can be put into the conversation besides words: the
// desktop client's attach menu, item for item and in its order. What the
// desktop client cannot do yet, it says so here too.
func (a *app) attachMenu() tea.Cmd {
	if a.open.JID == "" {
		return nil
	}
	notYet := func(what string) func() tea.Cmd {
		return func() tea.Cmd {
			a.failure = what + " is not supported yet"
			return nil
		}
	}
	return a.ask("Attach", "", 0,
		choice{label: "Document", run: a.sendFilePrompt(browsing{title: "Choose a document"}, "document", true)},
		choice{label: "Photos and videos", run: a.sendFilePrompt(browsing{title: "Choose a photo or a video",
			kind: "photos and videos", kinds: photoAndVideoFiles}, "", true)},
		choice{label: "Camera", run: notYet("Camera")},
		choice{label: "Audio", run: a.sendFilePrompt(browsing{title: "Choose an audio file",
			kind: "audio files", kinds: audioFiles}, "", false)},
		choice{label: "Contact", run: a.sendContactPrompt},
		choice{label: "Poll", run: a.pollPrompt},
		choice{label: "Event", run: notYet("Event")},
		choice{label: "New sticker", run: a.sendFilePrompt(browsing{title: "Choose a picture for the sticker",
			kind: "JPEG and PNG pictures", kinds: stickerPictures}, "sticker", false)})
}

// sendFilePrompt has a file chosen and sends it as the item it was asked for
// under: a document keeps its file, a sticker is made from a picture, and
// anything else goes as what it is. A document, a photo and a video can carry
// a caption.
func (a *app) sendFilePrompt(how browsing, as string, captioned bool) func() tea.Cmd {
	return func() tea.Cmd {
		chat, how := a.open.JID, how
		how.chosen = func(path string) tea.Cmd {
			if err := readableFile(path); err != nil {
				a.failure = safeLine(err.Error())
				return nil
			}
			send := func(caption string) tea.Cmd {
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
			}
			if !captioned {
				return send("")
			}
			return a.type_("A caption?", safeLine(filepath.Base(path)), "", send)
		}
		return a.browse(how, a.startFolder(), "")
	}
}

// pollPrompt asks for the question and then its options one at a time, as
// the desktop client's poll dialog does, then whether more than one may be
// chosen. WhatsApp takes two options to twelve, none twice.
func (a *app) pollPrompt() tea.Cmd {
	chat := a.open.JID
	return a.type_("Poll question", "", "", func(question string) tea.Cmd {
		if question == "" {
			return nil
		}
		return a.pollOption(chat, question, nil)
	})
}

const pollMost = 12

func (a *app) pollOption(chat, question string, options []string) tea.Cmd {
	hint := "an empty one when there are enough"
	if len(options) < 2 {
		hint = "a poll has two at the least"
	}
	return a.type_(fmt.Sprintf("Option %d", len(options)+1), hint, "", func(option string) tea.Cmd {
		switch {
		case option == "" && len(options) < 2:
			a.failure = "A poll needs at least two options."
			return nil
		case option == "":
			return a.pollAnswers(chat, question, options)
		case slices.Contains(options, option):
			a.failure = "That option is in the poll already."
			return a.pollOption(chat, question, options)
		}
		next := append(slices.Clone(options), option)
		if len(next) == pollMost {
			return a.pollAnswers(chat, question, next)
		}
		return a.pollOption(chat, question, next)
	})
}

func (a *app) pollAnswers(chat, question string, options []string) tea.Cmd {
	send := func(multiple bool) func() tea.Cmd {
		return func() tea.Cmd {
			return func() tea.Msg {
				// The daemon announces the poll it sent, and that announcement
				// is what puts it in the conversation.
				if err := a.daemon.call(a.ctx, "poll.create", map[string]any{
					"chat_jid": chat, "question": question, "options": options, "multiple": multiple}, nil); err != nil {
					return failureMsg{err}
				}
				return nil
			}
		}
	}
	return a.ask("How many answers may each person choose?", question, 0,
		choice{label: "One answer", run: send(false)},
		choice{label: "Several answers", run: send(true)})
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
