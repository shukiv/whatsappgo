package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/shukiv/whatsappgo/internal/model"
)

// Opening a picture hands the file to another program and never draws it. The
// opener here is a script that records what it was given, so the test can see
// the file go out without a desktop behind it.
func recorder(t *testing.T) (opener string, taken func() string) {
	t.Helper()
	home := t.TempDir()
	noted := filepath.Join(home, "handed-over")
	script := filepath.Join(home, "opener")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' \"$1\" > "+noted+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WHATSAPPGO_OPENER", script)
	// Whatever the program is handed, it runs with a home of its own and never
	// the account's.
	t.Setenv("HOME", t.TempDir())
	// The program is started and left running - nobody waits for it, so that
	// the reader goes on reading - which means this has to wait for it to say
	// what it was given.
	return script, func() string {
		for attempt := 0; attempt < 200; attempt++ {
			if given, err := os.ReadFile(noted); err == nil && len(given) > 0 {
				return string(given)
			}
			time.Sleep(10 * time.Millisecond)
		}
		return ""
	}
}

func withMedia(t *testing.T, message model.Message) (model.Message, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "balcony.jpg")
	if err := os.WriteFile(path, []byte("not really a picture"), 0o600); err != nil {
		t.Fatal(err)
	}
	message.MediaPath = path
	return message, path
}

// A picture already downloaded goes straight to the program that opens
// pictures.
func TestOpeningAPictureHandsTheFileToAnotherProgram(t *testing.T) {
	_, taken := recorder(t)
	picture, path := withMedia(t, model.Message{ID: "p1", ChatJID: alice, SenderJID: alice,
		Timestamp: noon, Kind: "image", MediaName: "balcony.jpg", MediaSize: 1_258_291})
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{picture}})
	current := openAlice(t, fake, model.MessagePage{})

	current = pump(t, current, press("o"))
	if given := taken(); given != path {
		t.Fatalf("the program was given %q, wanted %q", given, path)
	}
	// Nothing was downloaded: the daemon already had it.
	if asked := fake.calls("message.download"); len(asked) != 0 {
		t.Fatalf("it was downloaded again: %v", asked)
	}
	if !strings.Contains(current.View(), "Opened balcony.jpg") {
		t.Fatalf("the window does not say what was opened:\n%s", current.View())
	}
}

// One the daemon has not fetched yet is fetched first, and then handed over.
func TestOpeningAPictureNotYetFetchedFetchesItFirst(t *testing.T) {
	_, taken := recorder(t)
	fetched, path := withMedia(t, model.Message{ID: "p1", ChatJID: alice, SenderJID: alice,
		Timestamp: noon, Kind: "image"})
	fake := newFakeDaemon().
		answer("messages.list", model.MessagePage{Messages: []model.Message{
			{ID: "p1", ChatJID: alice, SenderJID: alice, Timestamp: noon, Kind: "image"}}}).
		answer("message.download", fetched)
	current := openAlice(t, fake, model.MessagePage{})

	current = pump(t, current, press("o"))
	asked := fake.only(t, "message.download")
	if asked.text("chat_jid") != alice || asked.text("message_id") != "p1" {
		t.Fatalf("it was fetched as %v", asked.params)
	}
	if given := taken(); given != path {
		t.Fatalf("the program was given %q, wanted %q", given, path)
	}
}

// A voice note somebody else sent is reported as played once it has been
// handed to the player, which is the moment it is played.
func TestOpeningAVoiceNoteReportsItPlayed(t *testing.T) {
	playerRecorder(t)
	note, _ := withMedia(t, model.Message{ID: "v1", ChatJID: alice, SenderJID: alice,
		Timestamp: noon, Kind: "audio", MediaDuration: 12})
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{note}})
	current := openAlice(t, fake, model.MessagePage{})
	t.Cleanup(current.stopPlaying)
	current = pumpPatiently(t, current, press("o"))

	played := fake.only(t, "message.played")
	if played.text("message_id") != "v1" || played.text("sender_jid") != alice {
		t.Fatalf("it was reported as %v", played.params)
	}

	// One we sent ourselves has nobody to report it to.
	ours, _ := withMedia(t, model.Message{ID: "v2", ChatJID: alice, FromMe: true,
		Timestamp: noon, Kind: "audio"})
	other := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{ours}})
	window := openAlice(t, other, model.MessagePage{})
	t.Cleanup(window.stopPlaying)
	window = pumpPatiently(t, window, press("o"))
	if played := other.calls("message.played"); len(played) != 0 {
		t.Fatalf("our own voice note was reported played: %v", played)
	}
}

// A name is a name. A file called something that would be a command if it were
// put on a command line is passed as an argument instead, so nothing in it
// runs.
//
// The commands in the name only ever leave a marker behind, in each of the
// three ways a shell would run them. If opening a file ever goes through a
// shell again, the worst that happens is a file this test can see. On
// 2026-11-05 a name here carried a real command, a mutation check put a shell
// in front of it, and it deleted a home directory.
func TestAFileNameIsNeverACommand(t *testing.T) {
	_, taken := recorder(t)
	// A shell would run them in the directory the program starts in.
	workdir := t.TempDir()
	t.Chdir(workdir)
	marker := filepath.Join(workdir, "pwned-marker")
	nasty := filepath.Join(t.TempDir(), "paid; touch pwned-marker; $(touch pwned-marker) `touch pwned-marker`.pdf")
	if err := os.WriteFile(nasty, []byte("a file"), 0o600); err != nil {
		t.Fatal(err)
	}
	message := model.Message{ID: "d1", ChatJID: alice, SenderJID: alice, Timestamp: noon,
		Kind: "document", MediaName: filepath.Base(nasty), MediaPath: nasty}
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{message}})

	pump(t, openAlice(t, fake, model.MessagePage{}), press("o"))
	if given := taken(); given != nasty {
		t.Fatalf("the program was given %q, wanted the name whole: %q", given, nasty)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a command in the file name ran: it left its marker behind")
	} else if !os.IsNotExist(err) {
		t.Fatalf("look for the marker: %v", err)
	}
}

// A message of words has nothing to open, and says so rather than doing
// something.
func TestThereIsNothingToOpenInAMessageOfWords(t *testing.T) {
	recorder(t)
	fake := newFakeDaemon()
	current := openAlice(t, fake, aliceMessages())
	current = pump(t, current, press("o"))
	if asked := fake.calls(""); len(asked) != 0 {
		t.Fatalf("opening a message of words asked for %v", asked)
	}
	if !strings.Contains(current.View(), "nothing to open") {
		t.Fatalf("the window does not say so:\n%s", current.View())
	}
}

// A link offers the page itself or the full preview the daemon can fetch.
func TestALinkOffersThePageAndTheFullPreview(t *testing.T) {
	_, taken := recorder(t)
	link := model.Message{ID: "l1", ChatJID: alice, SenderJID: alice, Timestamp: noon,
		Kind: "text", Body: "look at https://example.com", LinkURL: "https://example.com",
		LinkTitle: "Example"}
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{link}})
	current := openAlice(t, fake, model.MessagePage{})

	current = pump(t, current, press("o"))
	if current.chooser == nil || len(current.chooser.items) != 2 {
		t.Fatalf("the prompt offers %v", current.chooser)
	}
	current = pump(t, current, press("2"))
	refreshed := fake.only(t, "link.preview.refresh")
	if refreshed.text("message_id") != "l1" {
		t.Fatalf("the preview was asked for as %v", refreshed.params)
	}
	fake.forget()
	current = pump(t, current, press("o"), press("1"))
	if given := taken(); given != "https://example.com" {
		t.Fatalf("the program was given %q", given)
	}
}

// A link was written by whoever sent it, and the program that opens it picks
// what to run by the link's scheme. Only the schemes a conversation carries are
// followed - the same ones the window follows - so a message cannot ask this
// computer to open a file of its own, mount a share or start a game.
//
// None of these would do anything if they did get through: the hosts do not
// exist and the paths are not there.
func TestALinkIsOpenedOnlyIfItIsAWebAddress(t *testing.T) {
	script, _ := recorder(t)
	for _, address := range []string{
		"file:///nothing/here",
		"ssh://nobody@example.invalid",
		"javascript:void(0)",
		"smb://example.invalid/share",
		"steam://nothing",
		"/nothing/here",
		"example.com",
		"--version",
	} {
		link := model.Message{ID: "l1", ChatJID: alice, SenderJID: alice, Timestamp: noon,
			Kind: "text", Body: "look at " + address, LinkURL: address}
		fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{link}})
		current := openAlice(t, fake, model.MessagePage{})
		fake.forget()

		current = pump(t, current, press("o"))
		current = pump(t, current, chooseLabel(t, current, "open the page"))
		if !strings.Contains(current.View(), "not a web address") {
			t.Fatalf("opening %q does not say why nothing opened:\n%s", address, current.View())
		}
		if asked := fake.calls(""); len(asked) != 0 {
			t.Fatalf("opening %q asked the daemon for %v", address, asked)
		}
	}
	nothingHandedOver(t, script)
}

// The addresses a conversation does carry are followed, however the scheme is
// written, and nothing else is - the same two lists the window is held to.
func TestOnlyTheSchemesAConversationCarriesAreFollowed(t *testing.T) {
	for _, address := range []string{
		"https://example.com/path?q=1",
		"http://example.com",
		"HTTPS://example.com",
		"mailto:someone@example.com",
		"tel:+15551234567",
		"geo:32.08,34.78",
	} {
		if given, followed := followable(address); !followed || given != address {
			t.Errorf("%q is not followed (%q, %v)", address, given, followed)
		}
	}
	for _, address := range []string{
		"file:///nothing/here",
		" file:///nothing/here",
		"ssh://nobody@example.invalid",
		"javascript:void(0)",
		"smb://example.invalid/share",
		"steam://nothing",
		"/nothing/here",
		"example.com",
		"--version",
		"",
	} {
		if _, followed := followable(address); followed {
			t.Errorf("%q is followed", address)
		}
	}
}

// A place is opened as a place on a map, which is an address the window
// follows too.
func TestALocationOpensAsAPlaceOnAMap(t *testing.T) {
	_, taken := recorder(t)
	place := model.Message{ID: "g1", ChatJID: alice, SenderJID: alice, Timestamp: noon,
		Kind: "location", Latitude: 32.08, Longitude: 34.78}
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{place}})
	pump(t, openAlice(t, fake, model.MessagePage{}), press("o"))
	if given := taken(); given != "geo:32.080000,34.780000" {
		t.Fatalf("the program was given %q", given)
	}
}

// nothingHandedOver fails if the opener was started at all. A refused address
// never starts it, so a short wait is enough for one that wrongly did to have
// written down what it was given.
func nothingHandedOver(t *testing.T, script string) {
	t.Helper()
	time.Sleep(500 * time.Millisecond)
	noted := filepath.Join(filepath.Dir(script), "handed-over")
	if given, err := os.ReadFile(noted); err == nil {
		t.Fatalf("the program was given %q", given)
	} else if !os.IsNotExist(err) {
		t.Fatalf("look for what the program was given: %v", err)
	}
}

// A file is sent as the item of the attach menu it was chosen under, which is
// how the desktop client's attach menu sends it: a document keeps its file.
func TestAFileIsSentAsWhateverItIsBeingSentAs(t *testing.T) {
	path := filepath.Join(homeWith(t, "lease.pdf"), "lease.pdf")
	fake := newFakeDaemon().answer("message.send_media",
		model.Message{ID: "s1", ChatJID: alice, FromMe: true, Kind: "document", MediaName: "lease.pdf"})
	current := openAlice(t, fake, aliceMessages())

	current = pump(t, current, press("a"))
	current = pump(t, current, chooseLabel(t, current, "Document"))
	current = pump(t, current, chooseLabel(t, current, "lease.pdf"))
	current = pump(t, current, typed("the lease"), press("enter"))

	sent := fake.only(t, "message.send_media")
	if sent.text("path") != path || sent.params["document"] != true || sent.text("caption") != "the lease" {
		t.Fatalf("the file went as %v", sent.params)
	}
	if sent.params["voice"] != nil || sent.params["sticker"] != nil {
		t.Fatalf("it went as more than one kind: %v", sent.params)
	}
}

// A file that is not there is not sent, and the reader is told why.
func TestAFileThatIsNotThereIsNotSent(t *testing.T) {
	fake := newFakeDaemon()
	current := openAlice(t, fake, aliceMessages())
	current = pump(t, current, press("a"))
	current = pump(t, current, chooseLabel(t, current, "Photos and videos"))
	current = pump(t, current, chooseLabel(t, current, "Type a path"))
	current.prompt.input.SetValue("")
	current = pump(t, current, typed("/no/such/file.jpg"), press("enter"))
	if sent := fake.calls("message.send_media"); len(sent) != 0 {
		t.Fatalf("a file that is not there was sent: %v", sent)
	}
	if !strings.Contains(current.View(), "no such file") {
		t.Fatalf("the window does not say why:\n%s", current.View())
	}
}

// A sticker that has passed through the account is sent by reusing the
// original - the message it came in is what goes. It is found where the
// desktop client keeps it, beside the emoji.
func TestAStickerIsSentByReusingTheOriginal(t *testing.T) {
	fake := newFakeDaemon().answer("media.shared", model.SharedMessagePage{Messages: []model.Message{
		{ID: "k1", ChatJID: "972500000002@s.whatsapp.net", ChatTitle: "Bob Example",
			Timestamp: noon, Kind: "sticker"}}})
	current := openAlice(t, fake, aliceMessages())
	current = pump(t, current, press(":"))
	current = pump(t, current, tea.KeyMsg{Type: tea.KeyCtrlS})
	asked := fake.only(t, "media.shared")
	if asked.text("category") != "stickers" {
		t.Fatalf("the stickers were asked for as %v", asked.params)
	}
	current = pump(t, current, press("1"))
	sent := fake.only(t, "sticker.send")
	if sent.text("chat_jid") != "972500000002@s.whatsapp.net" || sent.text("message_id") != "k1" ||
		sent.text("to_chat_jid") != alice {
		t.Fatalf("the sticker went as %v", sent.params)
	}
}

// A card carries a name and a number somebody chose from the contacts that
// have one.
func TestAContactCardIsPickedFromTheContactsThatHaveANumber(t *testing.T) {
	fake := newFakeDaemon().answer("contacts.shareable", []model.Contact{
		{JID: "972500000002@s.whatsapp.net", Name: "Bob Example", Phone: "+972500000002"}}).
		answer("message.send_contact", model.Message{ID: "c1", ChatJID: alice, FromMe: true, Kind: "contact"})
	current := openAlice(t, fake, aliceMessages())
	current = pump(t, current, press("a"))
	current = pump(t, current, chooseLabel(t, current, "Contact"))
	current = pump(t, current, press("1"))
	sent := fake.only(t, "message.send_contact")
	contact, _ := sent.params["contact"].(map[string]any)
	if sent.text("chat_jid") != alice || contact["name"] != "Bob Example" || contact["phone"] != "+972500000002" {
		t.Fatalf("the card went as %v", sent.params)
	}
}

// A page named in a message being written is looked up once the typing
// settles, and travels with the message - but only while it is still named in
// it.
func TestAPageNamedInAMessageTravelsWithIt(t *testing.T) {
	fake := newFakeDaemon().
		answer("link.preview", model.LinkPreview{URL: "https://example.com", Title: "Example"}).
		answer("message.send", model.Message{ID: "m9", ChatJID: alice, FromMe: true, Kind: "text"})
	current := driveFake(t, fake, aliceChat(), press("enter"), typed("look at https://example.com"))
	current = pump(t, current, typingMsg(current.typingToken))

	asked := fake.only(t, "link.preview")
	if asked.text("text") != "https://example.com" {
		t.Fatalf("the page was asked about as %v", asked.params)
	}
	if !strings.Contains(current.View(), "link: Example") {
		t.Fatalf("the window does not show the preview:\n%s", current.View())
	}
	current = pump(t, current, press("enter"))
	sent := fake.only(t, "message.send")
	preview, _ := sent.params["link_preview"].(map[string]any)
	if preview["url"] != "https://example.com" {
		t.Fatalf("the message went without the preview: %v", sent.params)
	}
}

// A conversation's picture is a picture: handed over, not drawn.
func TestAConversationsPictureIsHandedOver(t *testing.T) {
	_, taken := recorder(t)
	path := filepath.Join(t.TempDir(), "alice.png")
	if err := os.WriteFile(path, []byte("a picture"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := newFakeDaemon().answer("chat.avatar", map[string]any{"avatar_path": path})
	current := menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example"})
	pump(t, current, chooseLabel(t, current, "open its picture"))
	asked := fake.only(t, "chat.avatar")
	if asked.text("chat_jid") != alice || asked.params["refresh"] != false {
		t.Fatalf("the picture was asked for as %v", asked.params)
	}
	if given := taken(); given != path {
		t.Fatalf("the program was given %q", given)
	}
}

// playerRecorder stands in for the player: it writes down the file it was
// given and then stays running, as a player does until the note ends, so
// that stopping it can be seen. It runs with a home of its own.
func playerRecorder(t *testing.T) (taken func() string) {
	t.Helper()
	home := t.TempDir()
	noted := filepath.Join(home, "played")
	script := filepath.Join(home, "player")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' \"$1\" > "+noted+"\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WHATSAPPGO_PLAYER", script)
	t.Setenv("HOME", t.TempDir())
	return func() string {
		for attempt := 0; attempt < 200; attempt++ {
			if given, err := os.ReadFile(noted); err == nil && len(given) > 0 {
				return string(given)
			}
			time.Sleep(10 * time.Millisecond)
		}
		return ""
	}
}

// homeWith makes a home folder holding these files, each holding its own
// name, and folders for the names that end in a slash.
func homeWith(t *testing.T, names ...string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, name := range names {
		path := filepath.Join(home, name)
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// A file is chosen the way the desktop client's dialog chooses one: from the
// home folder, folders first, only the kinds of file asked for unless every
// file is asked for, hidden ones only when asked, and backspace goes back up
// to the folder just left. The next file is chosen from the same folder.
func TestAFileIsChosenFromItsFolder(t *testing.T) {
	home := homeWith(t, "Music/", "readme.txt", "Pictures/beach.jpg", "Pictures/notes.txt", "Pictures/.secret.jpg")
	fake := newFakeDaemon().answer("message.send_media", model.Message{ID: "s1", ChatJID: alice, FromMe: true})
	current := pump(t, openAlice(t, fake, aliceMessages()), press("a"))
	current = pump(t, current, chooseLabel(t, current, "Photos and videos"))

	want := []string{"Type a path…", "📁 ..", "📁 Music/", "📁 Pictures/", "Show every file", "Show hidden files"}
	if got := menuLabels(current); !slices.Equal(got, want) {
		t.Fatalf("the home folder shows %v, not %v", got, want)
	}
	if !strings.Contains(current.chooser.hint, home) || !strings.Contains(current.chooser.hint, "no photos and videos here") {
		t.Fatalf("the list says %q", current.chooser.hint)
	}

	current = pump(t, current, chooseLabel(t, current, "Pictures/"))
	want = []string{"Type a path…", "📁 ..", "📄 beach.jpg  18 B", "Show every file", "Show hidden files"}
	if got := menuLabels(current); !slices.Equal(got, want) {
		t.Fatalf("the pictures show %v, not %v", got, want)
	}
	if current.chooser.cursor != 2 {
		t.Fatalf("a folder opens with the cursor on row %d, not on its first file", current.chooser.cursor)
	}
	current = pump(t, current, press("."))
	if got := menuLabels(current); !slices.Contains(got, "📄 .secret.jpg  20 B") {
		t.Fatalf("the hidden file is not shown when asked for: %v", got)
	}
	current = pump(t, current, chooseLabel(t, current, "Show every file"))
	if got := menuLabels(current); !slices.Contains(got, "📄 notes.txt  18 B") || !slices.Contains(got, "Show only photos and videos") {
		t.Fatalf("every file is not shown when asked for: %v", got)
	}

	current = pump(t, current, tea.KeyMsg{Type: tea.KeyBackspace})
	if at := current.chooser.cursor; current.chooser.items[at].label != "📁 Pictures/" {
		t.Fatalf("going up left the cursor on %q", current.chooser.items[at].label)
	}
	current = pump(t, current, press("enter"))
	current = pump(t, current, chooseLabel(t, current, "beach.jpg"))
	current = pump(t, current, press("enter"))
	sent := fake.only(t, "message.send_media")
	if want := filepath.Join(home, "Pictures", "beach.jpg"); sent.text("path") != want || sent.params["document"] != nil {
		t.Fatalf("the picture went as %v", sent.params)
	}

	current = pump(t, current, press("a"))
	current = pump(t, current, chooseLabel(t, current, "Photos and videos"))
	if !strings.HasPrefix(current.chooser.hint, filepath.Join(home, "Pictures")) {
		t.Fatalf("the next file is not chosen from the same folder: %q", current.chooser.hint)
	}
}

// A path can still be typed, from the folder being looked at, and a folder
// typed is opened rather than sent.
func TestAPathCanStillBeTyped(t *testing.T) {
	home := homeWith(t, "Pictures/beach.jpg")
	fake := newFakeDaemon().answer("message.send_media", model.Message{ID: "s1", ChatJID: alice, FromMe: true})
	current := pump(t, openAlice(t, fake, aliceMessages()), press("a"))
	current = pump(t, current, chooseLabel(t, current, "Photos and videos"))
	current = pump(t, current, chooseLabel(t, current, "Type a path"))
	if current.prompt == nil || current.prompt.input.Value() != home+"/" {
		t.Fatalf("the path does not start from the folder being looked at")
	}
	current = pump(t, current, typed("Pictures"), press("enter"))
	if current.prompt != nil || current.chooser == nil || !strings.HasPrefix(current.chooser.hint, filepath.Join(home, "Pictures")) {
		t.Fatal("a folder typed was not opened")
	}
	current = pump(t, current, chooseLabel(t, current, "Type a path"))
	current = pump(t, current, typed("beach.jpg"), press("enter"), press("enter"))
	if sent := fake.only(t, "message.send_media"); sent.text("path") != filepath.Join(home, "Pictures", "beach.jpg") {
		t.Fatalf("the path typed went as %v", sent.params)
	}
}

// A file's name is whatever somebody called it, so it is cleaned before the
// terminal is given it, while the file sent is the one named.
func TestAFileNameIsCleanedBeforeItIsShown(t *testing.T) {
	name := "evil\x1b]0;touch pwned-marker\x07.jpg"
	home := homeWith(t, "Pictures/")
	if err := os.WriteFile(filepath.Join(home, "Pictures", name), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := newFakeDaemon().answer("message.send_media", model.Message{ID: "s1", ChatJID: alice, FromMe: true})
	current := pump(t, openAlice(t, fake, aliceMessages()), press("a"))
	current = pump(t, current, chooseLabel(t, current, "Photos and videos"))
	current = pump(t, current, chooseLabel(t, current, "Pictures/"))
	if view := current.View(); strings.Contains(view, "\x1b]") || strings.Contains(view, "\x07") || !strings.Contains(view, "pwned-marker") {
		t.Fatalf("the name reached the terminal as it is: %q", view)
	}
	current = pump(t, current, chooseLabel(t, current, "pwned-marker"), press("enter"))
	if sent := fake.only(t, "message.send_media"); sent.text("path") != filepath.Join(home, "Pictures", name) {
		t.Fatalf("the file sent was %q", sent.text("path"))
	}
}

// A folder that cannot be read says why, and the list stays where it was.
func TestAFolderThatCannotBeReadSaysWhy(t *testing.T) {
	home := homeWith(t, "Locked/")
	locked := filepath.Join(home, "Locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("this user reads any folder")
	}
	current := pump(t, openAlice(t, newFakeDaemon(), aliceMessages()), press("a"))
	current = pump(t, current, chooseLabel(t, current, "Document"))
	current = pump(t, current, chooseLabel(t, current, "Locked/"))
	if !strings.Contains(current.failure, "permission denied") || current.chooser == nil ||
		!strings.HasPrefix(current.chooser.hint, home) || current.chooser.items[current.chooser.cursor].label != "📁 Locked/" {
		t.Fatalf("an unreadable folder left the list at %v with %q", current.chooser, current.failure)
	}
}

// A list of files reads from the left whatever its names are written in: the
// icon first and the size last, with only a Hebrew name itself turned round.
func TestAListOfFilesReadsFromTheLeft(t *testing.T) {
	homeWith(t, "חופשה.jpg")
	current := pump(t, openAlice(t, newFakeDaemon(), aliceMessages()), press("a"))
	current = pump(t, current, chooseLabel(t, current, "Photos and videos"))
	if view := ansi.Strip(current.View()); !strings.Contains(view, "📄 השפוח.jpg  14 B") {
		t.Fatalf("a Hebrew name turned its row round:\n%s", view)
	}
}

// A folder whose name would turn the words around it is not put in the box
// to be added to, and a name typed there is still taken from that folder.
func TestAFolderNameThatWouldNotShowAsItIsIsNotPutInTheBox(t *testing.T) {
	home := homeWith(t, "turn‮marker/beach.jpg")
	fake := newFakeDaemon().answer("message.send_media", model.Message{ID: "s1", ChatJID: alice, FromMe: true})
	current := pump(t, openAlice(t, fake, aliceMessages()), press("a"))
	current = pump(t, current, chooseLabel(t, current, "Photos and videos"))
	current = pump(t, current, chooseLabel(t, current, "marker/"))
	current = pump(t, current, chooseLabel(t, current, "Type a path"))
	if value := current.prompt.input.Value(); value != "" {
		t.Fatalf("the box was given %q", value)
	}
	current = pump(t, current, typed("beach.jpg"), press("enter"), press("enter"))
	if sent := fake.only(t, "message.send_media"); sent.text("path") != filepath.Join(home, "turn‮marker", "beach.jpg") {
		t.Fatalf("the name typed went as %q", sent.text("path"))
	}
}
