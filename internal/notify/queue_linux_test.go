//go:build linux

package notify

import (
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
)

func newTestDesktop() *Desktop {
	return &Desktop{chatNotifications: make(map[string]uint32)}
}

// A notification server refuses everything once its queue is full, and from
// that moment the user silently stops receiving messages. Leaving more than
// maxLiveNotifications open is what fills it.
func TestLiveNotificationsAreBounded(t *testing.T) {
	// GNOME's notification-daemon refuses past roughly twenty. The bound has to
	// stay well under whatever the least generous server allows, so it is
	// asserted here rather than left to whatever the constant happens to say.
	if maxLiveNotifications > 16 {
		t.Fatalf("maxLiveNotifications=%d is too close to a server's own cap", maxLiveNotifications)
	}
	d := newTestDesktop()
	var closed []uint32
	for i := 1; i <= maxLiveNotifications+3; i++ {
		id := uint32(i)
		d.trackLiveLocked(id, "chat-"+string(rune('a'+i)))
		closed = append(closed, d.overflowingLiveLocked()...)
	}
	if len(d.liveNotifications) != maxLiveNotifications {
		t.Fatalf("kept %d notifications open, want %d", len(d.liveNotifications), maxLiveNotifications)
	}
	if len(closed) != 3 {
		t.Fatalf("closed %d notifications, want 3", len(closed))
	}
	// The oldest go first: a message from an hour ago is the one worth losing.
	for i, id := range closed {
		if id != uint32(i+1) {
			t.Fatalf("closed[%d]=%d, want %d - not oldest first", i, id, i+1)
		}
	}
}

// One busy group must not fill the queue on its own. Replacing a chat's own
// notification is also what WhatsApp does.
func TestOneNotificationPerChat(t *testing.T) {
	d := newTestDesktop()
	d.trackLiveLocked(1, "alice@s.whatsapp.net")
	d.trackLiveLocked(2, "alice@s.whatsapp.net")
	if len(d.liveNotifications) != 1 {
		t.Fatalf("chat holds %d notifications open, want 1", len(d.liveNotifications))
	}
	if got := d.chatNotifications["alice@s.whatsapp.net"]; got != 2 {
		t.Fatalf("chat points at notification %d, want 2", got)
	}
}

func TestClosedNotificationLeavesTheQueue(t *testing.T) {
	d := newTestDesktop()
	d.trackLiveLocked(7, "bob@s.whatsapp.net")
	d.forgetLiveLocked(7)
	if len(d.liveNotifications) != 0 {
		t.Fatal("a closed notification still counts against the queue")
	}
	if _, ok := d.chatNotifications["bob@s.whatsapp.net"]; ok {
		t.Fatal("a closed notification is still named as a chat's live one")
	}
}

// Recovery depends on recognising the error by name, not by message text.
func TestMaxNotificationsExceededIsRecognised(t *testing.T) {
	full := dbus.Error{Name: maxNotificationsExceeded}
	if !isMaxNotificationsExceeded(full) {
		t.Fatal("a full notification queue was not recognised")
	}
	if isMaxNotificationsExceeded(errors.New("connection closed")) {
		t.Fatal("an unrelated error was treated as a full queue")
	}
}
