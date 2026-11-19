package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/notify"
	"github.com/shukiv/whatsappgo/internal/rpc"
	"github.com/shukiv/whatsappgo/internal/store"
)

// presentingNotifier stands in for the desktop notifier a daemon opens.
type presentingNotifier struct{}

func (presentingNotifier) Notify(context.Context, notify.Message) error { return nil }
func (presentingNotifier) Presents() bool                               { return true }
func (presentingNotifier) Close() error                                 { return nil }

func newPresentService(t *testing.T) *Service {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	st, err := store.Open("file:" + name + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := New(st, &fakeGateway{}, events.New())
	t.Cleanup(svc.Close)
	return svc
}

func presenting(t *testing.T, raw json.RawMessage) bool {
	t.Helper()
	var answer struct {
		Presenting *bool `json:"presenting"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil || answer.Presenting == nil {
		t.Fatalf("notifications.present answered %s (%v), want {\"presenting\":bool}", raw, err)
	}
	return *answer.Presenting
}

// The window holds a daemon's notifications through its own connection, and
// they last exactly as long as that connection. This goes through the RPC
// server on a real socket because the server is what ends the hold: it cancels
// a connection's context when the client goes away. A context cancelled by
// hand would pass whether or not the server did that.
func TestPresentEndsWithTheConnection(t *testing.T) {
	svc := newPresentService(t)
	onDemand := notify.NewOnDemand(func() (notify.Notifier, error) { return presentingNotifier{}, nil })
	svc.UseNotifier(onDemand)
	broker := events.New()
	path := filepath.Join(t.TempDir(), "rpc.sock")
	server := rpc.NewServer(path, svc, broker)
	if err := server.Listen(); err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Serve(ctx)

	window, err := rpc.Dial(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := window.Call(ctx, "notifications.present", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !presenting(t, raw) || !onDemand.Presents() {
		t.Fatal("a window asked and the daemon still does not present notifications")
	}

	// Another client coming and going leaves the window's hold alone.
	other, err := rpc.Dial(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Call(ctx, "status.get", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	_ = other.Close()
	time.Sleep(50 * time.Millisecond)
	if !onDemand.Presents() {
		t.Fatal("another client leaving ended the window's notifications")
	}

	_ = window.Close()
	deadline := time.Now().Add(2 * time.Second)
	for onDemand.Presents() {
		if time.Now().After(deadline) {
			t.Fatal("the window closed and the daemon still presents notifications")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A daemon started with notifications on presents them without being asked,
// and one with nothing to post through says it does not; neither is an error,
// so a window can ask every daemon it meets.
func TestPresentAnswersForDaemonsThatDoNotHold(t *testing.T) {
	for name, tc := range map[string]struct {
		notifier notify.Notifier
		want     bool
	}{
		"started with notifications": {presentingNotifier{}, true},
		"no notification service":    {notify.Noop{}, false},
		"no notifier set":            {nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			svc := newPresentService(t)
			if tc.notifier != nil {
				svc.UseNotifier(tc.notifier)
			}
			result, err := svc.Handle(context.Background(), "notifications.present", nil)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if got := presenting(t, raw); got != tc.want {
				t.Fatalf("presenting = %v, want %v", got, tc.want)
			}
		})
	}
}
