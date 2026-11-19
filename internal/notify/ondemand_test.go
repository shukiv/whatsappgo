package notify

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// recordingNotifier stands in for the desktop notifier: it presents, and keeps
// what it was asked to post.
type recordingNotifier struct {
	mu     sync.Mutex
	posted []string
	closed bool
}

func (r *recordingNotifier) Notify(_ context.Context, msg Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.posted = append(r.posted, msg.Title)
	return nil
}

func (r *recordingNotifier) Presents() bool { return true }

func (r *recordingNotifier) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func (r *recordingNotifier) titles() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.posted...)
}

// eventually waits for something a hold's background release does.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A daemon started without notifications posts them only while a window holds
// it, and does not open the desktop notifier at all until one does.
func TestOnDemandPresentsOnlyWhileHeld(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	desktop := &recordingNotifier{}
	opens := 0
	onDemand := NewOnDemand(func() (Notifier, error) {
		opens++
		return desktop, nil
	})
	defer onDemand.Close()
	ctx := context.Background()

	if onDemand.Presents() {
		t.Fatal("presents before any window asked")
	}
	if err := onDemand.Notify(ctx, Message{Title: "before"}); err != nil {
		t.Fatal(err)
	}
	if opens != 0 {
		t.Fatalf("opened the desktop notifier %d times before anything held it", opens)
	}

	window, leave := context.WithCancel(ctx)
	defer leave()
	if err := onDemand.Hold(window); err != nil {
		t.Fatal(err)
	}
	if !onDemand.Presents() {
		t.Fatal("held, but does not say it presents: the window would show nothing")
	}
	if err := onDemand.Notify(ctx, Message{Title: "while held"}); err != nil {
		t.Fatal(err)
	}

	leave()
	eventually(t, "the hold to end with the window's connection", func() bool { return !onDemand.Presents() })
	if err := onDemand.Notify(ctx, Message{Title: "after"}); err != nil {
		t.Fatal(err)
	}
	if got := desktop.titles(); len(got) != 1 || got[0] != "while held" {
		t.Fatalf("posted %q, want only the notification that arrived while held", got)
	}
}

// Two windows on one daemon: the first to close must not take the second's
// notifications with it, and the notifier is opened once between them.
func TestOnDemandStaysHeldUntilEveryHolderLeaves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	opens := 0
	onDemand := NewOnDemand(func() (Notifier, error) {
		opens++
		return &recordingNotifier{}, nil
	})
	defer onDemand.Close()
	first, leaveFirst := context.WithCancel(context.Background())
	defer leaveFirst()
	second, leaveSecond := context.WithCancel(context.Background())
	defer leaveSecond()
	if err := onDemand.Hold(first); err != nil {
		t.Fatal(err)
	}
	if err := onDemand.Hold(second); err != nil {
		t.Fatal(err)
	}
	if opens != 1 {
		t.Fatalf("opened the desktop notifier %d times for two holds, want once", opens)
	}

	leaveFirst()
	time.Sleep(50 * time.Millisecond)
	if !onDemand.Presents() {
		t.Fatal("the first window to close ended the second window's notifications")
	}
	leaveSecond()
	eventually(t, "the last hold to end", func() bool { return !onDemand.Presents() })
}

// With no notification service to open, a hold fails and changes nothing: the
// window keeps presenting what the daemon leaves to it.
func TestOnDemandHoldFailsWhenNothingOpens(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for name, open := range map[string]func() (Notifier, error){
		"error":          func() (Notifier, error) { return nil, errors.New("no session bus") },
		"nothing opened": func() (Notifier, error) { return nil, nil },
	} {
		t.Run(name, func(t *testing.T) {
			onDemand := NewOnDemand(open)
			defer onDemand.Close()
			if err := onDemand.Hold(context.Background()); err == nil {
				t.Fatal("a hold with nothing to present through succeeded")
			}
			if onDemand.Presents() {
				t.Fatal("says it presents with nothing to present through")
			}
		})
	}
}

func TestOnDemandCloseClosesWhatItOpened(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	desktop := &recordingNotifier{}
	onDemand := NewOnDemand(func() (Notifier, error) { return desktop, nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := onDemand.Hold(ctx); err != nil {
		t.Fatal(err)
	}
	if err := onDemand.Close(); err != nil {
		t.Fatal(err)
	}
	desktop.mu.Lock()
	closed := desktop.closed
	desktop.mu.Unlock()
	if !closed {
		t.Fatal("closing left the desktop notifier's session bus connection open")
	}
	if onDemand.Presents() {
		t.Fatal("still presents after closing")
	}
}
