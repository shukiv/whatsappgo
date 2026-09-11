package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/notify"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

type refusingNotifier struct {
	err      error
	attempts int
}

func (n *refusingNotifier) Notify(context.Context, notify.Message) error {
	n.attempts++
	return n.err
}

func (n *refusingNotifier) Presents() bool { return true }

func (n *refusingNotifier) Close() error { return nil }

// A notification server whose queue is full refuses every client. The daemon
// has already told the window that it presented the message, so a refusal that
// stayed in the log would leave the reader with nothing at all.
func TestAFailedAlertAsksTheWindowToPresentItInstead(t *testing.T) {
	ctx := context.Background()
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.UpsertMessage(ctx, model.Message{ID: "m1", ChatJID: "alice@lid", Timestamp: 1000, Kind: "text", Body: "hello", Status: "received"}, "Alice", true); err != nil {
		t.Fatal(err)
	}
	client := &Client{store: st, notifier: &refusingNotifier{err: errors.New("MaxNotificationsExceeded")}}
	client.subs = map[uint64]func(gateway.Event){}

	events := make(chan map[string]string, 4)
	client.Subscribe(func(evt gateway.Event) {
		if evt.Name != "notification.received" {
			return
		}
		data, ok := evt.Data.(map[string]string)
		if !ok {
			t.Errorf("notification event carried %T", evt.Data)
			return
		}
		events <- data
	})

	client.deliverAlert("alice@lid", "Alice", "Alice reacted 👍 to your message", false)

	first := waitForEvent(t, events)
	if first["handled"] != "1" {
		t.Fatalf("the daemon did not claim the alert first: %#v", first)
	}
	second := waitForEvent(t, events)
	if second["handled"] != "0" {
		t.Fatalf("a refused alert was not handed back to the window: %#v", second)
	}
	if second["body"] != first["body"] || second["chat_jid"] != "alice@lid" {
		t.Fatalf("the replacement event describes another notification: %#v", second)
	}
}

func waitForEvent(t *testing.T, events chan map[string]string) map[string]string {
	t.Helper()
	select {
	case data := <-events:
		return data
	case <-time.After(5 * time.Second):
		t.Fatal("no notification event arrived")
		return nil
	}
}

// The deduplication table bounds memory. It must not decide that an incoming
// call is not worth announcing because enough status updates arrived first.
func TestAFullAlertTableStillLetsACallThrough(t *testing.T) {
	client := &Client{}
	for i := 0; i < maxTrackedAlerts; i++ {
		if !client.claimAlert(fmt.Sprintf("status:%d@lid:msg%d", i, i)) {
			t.Fatalf("status alert %d was refused while filling the table", i)
		}
	}
	if !client.claimAlert("call:alice@lid:call-1") {
		t.Fatal("a full table refused an incoming call alert")
	}
	if len(client.callAlerts) > maxTrackedAlerts {
		t.Fatalf("the table grew past its bound: %d entries", len(client.callAlerts))
	}
	if client.claimAlert("call:alice@lid:call-1") {
		t.Fatal("the same call was announced twice")
	}
}

func TestTheOldestAlertIsTheOneEvicted(t *testing.T) {
	client := &Client{callAlerts: map[string]time.Time{}}
	now := time.Now()
	for i := 0; i < maxTrackedAlerts; i++ {
		// Recent enough to survive expiry, ordered oldest first.
		client.callAlerts[fmt.Sprintf("status:%d", i)] = now.Add(-time.Duration(maxTrackedAlerts-i) * time.Second)
	}
	if !client.claimAlert("security:alice@lid") {
		t.Fatal("a full table refused a security-code alert")
	}
	if _, kept := client.callAlerts["status:0"]; kept {
		t.Fatal("the newest entry was evicted instead of the oldest")
	}
	if _, kept := client.callAlerts[fmt.Sprintf("status:%d", maxTrackedAlerts-1)]; !kept {
		t.Fatal("a recent entry was evicted")
	}
}
