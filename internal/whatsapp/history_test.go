package whatsapp

import (
	"context"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

func TestHistoryBackfillKeyIsPerConversation(t *testing.T) {
	if historyBackfillKey("a@lid") == historyBackfillKey("b@lid") {
		t.Fatal("two conversations share a completion marker")
	}
	if got, want := historyBackfillKey("a@lid"), historyBackfillMetadataPrefix+"a@lid"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
}

func TestOldestTimestampIgnoresEnvelopes(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	c := &Client{store: st}
	const chat = "alice@s.whatsapp.net"
	for _, message := range []model.Message{
		{ID: "envelope", ChatJID: chat, Timestamp: 1, Kind: "", Status: "received"},
		{ID: "real", ChatJID: chat, Timestamp: 9, Kind: "text", Body: "hi", Status: "received"},
	} {
		if err := st.UpsertMessage(ctx, message, "Alice", false); err != nil {
			t.Fatal(err)
		}
	}
	// Paging must anchor on a real message; asking for history older than a
	// transport envelope would return nothing and stop the collection early.
	got, err := c.oldestTimestamp(ctx, chat)
	if err != nil {
		t.Fatal(err)
	}
	if got != 9 {
		t.Fatalf("anchored at %d, want the oldest real message at 9", got)
	}
	if _, err := c.oldestTimestamp(ctx, "nobody@s.whatsapp.net"); err == nil {
		t.Fatal("a conversation with no messages should report no anchor")
	}
}

func TestCollectChatHistoryStopsWithoutAnAnchor(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c := &Client{store: st}
	// An empty conversation is complete: there is nothing to page back from,
	// and it must not be retried on every connection.
	added, exhausted := c.collectChatHistory(context.Background(), "empty@s.whatsapp.net")
	if added != 0 || !exhausted {
		t.Fatalf("added=%d exhausted=%v, want 0 and true", added, exhausted)
	}
}

func TestCollectHistorySkipsConversationsAlreadyDone(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	const chat = "alice@s.whatsapp.net"
	if err := st.UpsertMessage(ctx, model.Message{ID: "m", ChatJID: chat, Timestamp: 5, Kind: "text", Body: "hi", Status: "received"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	if err := st.SetMetadata(ctx, historyBackfillKey(chat), "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	// A finished conversation must not reach RequestHistory, which would
	// dereference the WhatsApp client this test does not have.
	c := &Client{store: st}
	c.collectHistory(ctx)
}

func TestCollectHistoryStopsWhenCancelled(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.UpsertMessage(context.Background(), model.Message{ID: "m", ChatJID: "a@lid", Timestamp: 5, Kind: "text", Body: "hi", Status: "received"}, "A", false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &Client{store: st}
	// A cancelled daemon must not start asking WhatsApp for anything.
	c.collectHistory(ctx)
}
