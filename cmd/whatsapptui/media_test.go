package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	recorder(t)
	note, _ := withMedia(t, model.Message{ID: "v1", ChatJID: alice, SenderJID: alice,
		Timestamp: noon, Kind: "audio", MediaDuration: 12})
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{note}})
	current := openAlice(t, fake, model.MessagePage{})
	pump(t, current, press("o"))

	played := fake.only(t, "message.played")
	if played.text("message_id") != "v1" || played.text("sender_jid") != alice {
		t.Fatalf("it was reported as %v", played.params)
	}

	// One we sent ourselves has nobody to report it to.
	ours, _ := withMedia(t, model.Message{ID: "v2", ChatJID: alice, FromMe: true,
		Timestamp: noon, Kind: "audio"})
	other := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{ours}})
	pump(t, openAlice(t, other, model.MessagePage{}), press("o"))
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

// A file is sent as what it is being sent as, which is the choice the window's
// attach menu offers for the same file.
func TestAFileIsSentAsWhateverItIsBeingSentAs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lease.pdf")
	if err := os.WriteFile(path, []byte("a lease"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := newFakeDaemon().answer("message.send_media",
		model.Message{ID: "s1", ChatJID: alice, FromMe: true, Kind: "document", MediaName: "lease.pdf"})
	current := openAlice(t, fake, aliceMessages())

	current = pump(t, current, press("a"))
	current = pump(t, current, chooseLabel(t, current, "a file"))
	current = pump(t, current, typed(path), press("enter"))
	current = pump(t, current, chooseLabel(t, current, "a file"))
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
	current = pump(t, current, chooseLabel(t, current, "a file"))
	current = pump(t, current, typed("/no/such/file.jpg"), press("enter"))
	if sent := fake.calls("message.send_media"); len(sent) != 0 {
		t.Fatalf("a file that is not there was sent: %v", sent)
	}
	if !strings.Contains(current.View(), "no such file") {
		t.Fatalf("the window does not say why:\n%s", current.View())
	}
}

// A sticker is sent by reusing the original, which is the only way one can be
// sent: the message it came in is what goes.
func TestAStickerIsSentByReusingTheOriginal(t *testing.T) {
	fake := newFakeDaemon().answer("media.shared", model.SharedMessagePage{Messages: []model.Message{
		{ID: "k1", ChatJID: "972500000002@s.whatsapp.net", ChatTitle: "Bob Example",
			Timestamp: noon, Kind: "sticker"}}})
	current := openAlice(t, fake, aliceMessages())
	current = pump(t, current, press("a"))
	current = pump(t, current, chooseLabel(t, current, "a sticker"))
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
	current = pump(t, current, chooseLabel(t, current, "a contact card"))
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
