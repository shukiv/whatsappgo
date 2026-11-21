//go:build linux

package notify

import (
	"context"
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"
)

// serverCall is one call a notification service received.
type serverCall struct {
	method   string
	replaces uint32
	id       uint32
}

// recordingServer stands in for org.freedesktop.Notifications: it hands out ids
// the way a server does, keeping the id it was asked to replace, and records
// every call.
type recordingServer struct {
	mu    sync.Mutex
	next  uint32
	calls []serverCall
}

func (s *recordingServer) Notify(_ context.Context, replaces uint32, _, _, _ string, _ []string, _ map[string]dbus.Variant) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := replaces
	if id == 0 {
		s.next++
		id = s.next
	}
	s.calls = append(s.calls, serverCall{method: "Notify", replaces: replaces, id: id})
	return id, nil
}

func (s *recordingServer) CloseNotification(_ context.Context, id uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, serverCall{method: "CloseNotification", id: id})
	return nil
}

func recordingDesktop(gnomeShell bool) (*Desktop, *recordingServer) {
	server := &recordingServer{}
	return &Desktop{
		server:            server,
		gnomeShell:        gnomeShell,
		actions:           make(map[uint32]string),
		chatNotifications: make(map[string]uint32),
	}, server
}

// Two messages from one chat, then one from each of five more: enough for the
// replacing and, past maxLiveNotifications, the closing.
var burst = []string{"alice@lid", "alice@lid", "bob@lid", "carol@lid", "dave@lid", "erin@lid", "frank@lid"}

func notifyBurst(t *testing.T, d *Desktop) {
	t.Helper()
	for _, chat := range burst {
		if err := d.Notify(context.Background(), Message{ChatJID: chat, Title: chat, Body: "hello"}); err != nil {
			t.Fatal(err)
		}
	}
}

// Replacing the notification GNOME Shell is showing can leave its message tray
// broken - "this._notification is null" - with no banner from any program
// until the shell restarts. So it is never asked to replace or close one, and
// a click on any of them still opens its chat.
func TestGNOMEShellIsNeverAskedToReplaceOrClose(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	d, server := recordingDesktop(true)
	notifyBurst(t, d)

	if len(server.calls) != len(burst) {
		t.Fatalf("GNOME Shell got %d calls for %d messages: %+v", len(server.calls), len(burst), server.calls)
	}
	for i, call := range server.calls {
		if call.method != "Notify" {
			t.Fatalf("call %d was %s; GNOME Shell must never be asked to close a notification", i, call.method)
		}
		if call.replaces != 0 {
			t.Fatalf("call %d (%s) asked GNOME Shell to replace notification %d", i, burst[i], call.replaces)
		}
		if got := d.actions[call.id]; got != burst[i] {
			t.Fatalf("a click on notification %d would open %q, want %q", call.id, got, burst[i])
		}
	}
	if len(d.liveNotifications) != 0 || len(d.chatNotifications) != 0 {
		t.Fatalf("kept %d live ids and %d chat ids for notifications that are never replaced or closed",
			len(d.liveNotifications), len(d.chatNotifications))
	}
}

// Any other server keeps one notification per chat and a bounded queue: it is
// the kind that refuses everything once its queue is full.
func TestOtherServersStillReplaceAndClose(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	d, server := recordingDesktop(false)
	notifyBurst(t, d)

	var posted []serverCall
	var closed []uint32
	for _, call := range server.calls {
		if call.method == "CloseNotification" {
			closed = append(closed, call.id)
		} else {
			posted = append(posted, call)
		}
	}
	if len(posted) != len(burst) {
		t.Fatalf("posted %d notifications for %d messages", len(posted), len(burst))
	}
	if posted[1].replaces != posted[0].id {
		t.Fatalf("alice's second message replaced %d, want her first notification %d", posted[1].replaces, posted[0].id)
	}
	// Seven messages over six chats: alice, bob, carol and dave fill the bound,
	// erin closes alice's and frank closes bob's.
	if len(closed) != 2 || closed[0] != posted[0].id || closed[1] != posted[2].id {
		t.Fatalf("closed %v, want the two oldest: [%d %d]", closed, posted[0].id, posted[2].id)
	}
}

func TestOnlyGNOMEShellIsTreatedAsGNOMEShell(t *testing.T) {
	for name, want := range map[string]bool{
		"gnome-shell":         true,
		"Notification Daemon": false, // GNOME's notification-daemon, which does refuse
		"GNOME":               false, // the vendor both of them give
		"":                    false,
	} {
		if got := isGNOMEShell(name); got != want {
			t.Errorf("isGNOMEShell(%q) = %v, want %v", name, got, want)
		}
	}
}
