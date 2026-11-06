//go:build !windows

// The player these tests start is a shell script, and that it has stopped is
// seen with signal 0; Windows has neither.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/shukiv/whatsappgo/internal/model"
)

func openVoiceNote(t *testing.T) (*app, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "note.ogg")
	if err := os.WriteFile(path, []byte("not really a voice note"), 0o600); err != nil {
		t.Fatal(err)
	}
	note := model.Message{ID: "v1", ChatJID: alice, SenderJID: alice, Timestamp: noon,
		Kind: "audio", MediaDuration: 7, MediaSize: 18_000, MediaPath: path}
	fake := newFakeDaemon().answer("messages.list", model.MessagePage{Messages: []model.Message{note}})
	current := openAlice(t, fake, model.MessagePage{})
	t.Cleanup(current.stopPlaying)
	return current, path
}

// A voice note plays here, in a player with no window, on o; it shows that
// it is playing; and the same key stops it.
func TestAVoiceNotePlaysHereAndTheSameKeyStopsIt(t *testing.T) {
	taken := playerRecorder(t)
	current, path := openVoiceNote(t)
	current.focus, current.composing = paneConversation, false
	if !strings.Contains(current.footer(), "o play") {
		t.Fatalf("the key help does not say o plays the voice note: %q", current.footer())
	}

	current = pump(t, current, press("o"))
	if given := taken(); given != path {
		t.Fatalf("the player was given %q, not the voice note %q", given, path)
	}
	if current.playing.command == nil || !strings.Contains(current.footer(), "o stop") ||
		!strings.Contains(ansi.Strip(current.View()), "■") {
		t.Fatalf("the voice note does not show as playing:\n%s", ansi.Strip(current.View()))
	}

	player := current.playing.command.Process.Pid
	current = pump(t, current, press("o"))
	if current.playing.command != nil || !strings.Contains(ansi.Strip(current.View()), "▶") {
		t.Fatal("the same key did not stop the voice note")
	}
	for attempt := 0; attempt < 200; attempt++ {
		if syscall.Kill(player, 0) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the player was left playing")
}

// A click on a voice note plays it.
func TestAClickOnAVoiceNotePlaysIt(t *testing.T) {
	taken := playerRecorder(t)
	current, path := openVoiceNote(t)
	listWidth, _, _ := current.layout()
	current = pump(t, current, click(listWidth+10, rowOf(t, current, "voice note")))
	if given := taken(); given != path || current.playing.command == nil {
		t.Fatalf("a click on the voice note did not play it (the player was given %q)", given)
	}
}
