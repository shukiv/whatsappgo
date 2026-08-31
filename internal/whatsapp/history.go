package whatsapp

import (
	"context"
	"fmt"
	"time"

	"github.com/shuki/whatsappgo/internal/gateway"
)

// WhatsApp hands a linked device only a recent window of each conversation.
// Everything older has to be asked for one page at a time, and the account
// this runs against is a real one: requesting pages as fast as possible is
// what unofficial clients get noticed for. The collector is therefore
// deliberately slow, bounded per run, and resumable.
const (
	historyBackfillMetadataPrefix = "history_backfill_v1:"
	// Time between two page requests.
	historyRequestInterval = 4 * time.Second
	// How long a requested page is given to arrive before the conversation is
	// treated as having nothing older.
	historyPageTimeout = 25 * time.Second
	// Pages per conversation per run, so one huge conversation cannot occupy
	// the collector forever. What is left is picked up on the next connection.
	historyRoundsPerChat = 60
	// Messages per page. WhatsApp caps this; 50 matches what the interface
	// asks for when the reader scrolls past the oldest local message.
	historyPageSize = 50
	// Conversations examined per run, newest first.
	historyChatsPerRun = 200
)

func historyBackfillKey(chatJID string) string {
	return historyBackfillMetadataPrefix + chatJID
}

// collectHistory walks the conversations and asks WhatsApp for older pages
// until each one runs out. It is safe to call on every connection: finished
// conversations are recorded and skipped, and an unfinished one resumes from
// wherever its oldest stored message now is.
func (c *Client) collectHistory(ctx context.Context) {
	chats, err := c.store.ListChats(ctx, historyChatsPerRun, 0, "")
	if err != nil {
		return
	}
	collected := 0
	for _, chat := range chats {
		if ctx.Err() != nil {
			return
		}
		if _, done, err := c.store.Metadata(ctx, historyBackfillKey(chat.JID)); err != nil || done {
			continue
		}
		added, exhausted := c.collectChatHistory(ctx, chat.JID)
		collected += added
		if exhausted {
			_ = c.store.SetMetadata(ctx, historyBackfillKey(chat.JID), time.Now().UTC().Format(time.RFC3339))
		}
		if added > 0 {
			c.emit(gateway.Event{Name: "history.collected", Data: map[string]any{
				"chat_jid": chat.JID, "messages": added, "complete": exhausted,
			}})
		}
	}
	if collected > 0 {
		c.emit(gateway.Event{Name: "history.synced", Data: map[string]int{"messages": collected}})
	}
}

// collectChatHistory pages one conversation backwards. It reports how many
// messages arrived and whether the conversation has no more history to give.
func (c *Client) collectChatHistory(ctx context.Context, chatJID string) (int, bool) {
	collected := 0
	for round := 0; round < historyRoundsPerChat; round++ {
		if ctx.Err() != nil {
			return collected, false
		}
		before, err := c.oldestTimestamp(ctx, chatJID)
		if err != nil {
			// With no local message there is nothing to ask "older than".
			return collected, true
		}
		if err := c.RequestHistory(ctx, chatJID, historyPageSize); err != nil {
			return collected, false
		}
		added, arrived := c.awaitOlderHistory(ctx, chatJID, before)
		collected += added
		if !arrived {
			// Nothing older came back, so this conversation is complete as far
			// as WhatsApp is willing to go.
			return collected, true
		}
		select {
		case <-ctx.Done():
			return collected, false
		case <-time.After(historyRequestInterval):
		}
	}
	return collected, false
}

// awaitOlderHistory waits for a requested page to be stored, reporting how
// many messages it added.
func (c *Client) awaitOlderHistory(ctx context.Context, chatJID string, before int64) (int, bool) {
	deadline := time.Now().Add(historyPageTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, false
		case <-time.After(time.Second):
		}
		after, err := c.oldestTimestamp(ctx, chatJID)
		if err != nil {
			continue
		}
		if after < before {
			page, err := c.store.ListMessages(ctx, chatJID, before, historyPageSize*2)
			if err != nil {
				return 1, true
			}
			return len(page.Messages), true
		}
	}
	return 0, false
}

func (c *Client) oldestTimestamp(ctx context.Context, chatJID string) (int64, error) {
	oldest, err := c.store.OldestMessage(ctx, chatJID)
	if err != nil {
		return 0, fmt.Errorf("oldest message: %w", err)
	}
	return oldest.Timestamp, nil
}
