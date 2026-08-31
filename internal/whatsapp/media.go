package whatsapp

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/shuki/whatsappgo/internal/gateway"
	localstore "github.com/shuki/whatsappgo/internal/store"
)

// Collecting attachments is the slowest thing this daemon does and the most
// visible to WhatsApp, so it runs last, one file at a time, and remembers how
// far it reached. Everything it fetches is written to the attachment database,
// which is what makes the local copy survive clearing the media cache.
const (
	mediaCollectCursorKey = "media_backfill_cursor_v1"
	mediaRequestInterval  = 1500 * time.Millisecond
	mediaFilesPerRun      = 400
	mediaScanPageSize     = 50
	// Anything larger is left for the reader to ask for explicitly.
	mediaSizeCeiling = 50 << 20
)

func formatMediaCursor(cursor localstore.MessageCursor) string {
	return strconv.FormatInt(cursor.Timestamp, 10) + "|" + cursor.MessageID
}

func parseMediaCursor(value string) localstore.MessageCursor {
	timestamp, messageID, found := strings.Cut(value, "|")
	if !found {
		return localstore.MessageCursor{}
	}
	parsed, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return localstore.MessageCursor{}
	}
	return localstore.MessageCursor{Timestamp: parsed, MessageID: messageID}
}

func (c *Client) storedMediaCursor(ctx context.Context) localstore.MessageCursor {
	value, found, err := c.store.Metadata(ctx, mediaCollectCursorKey)
	if err != nil || !found {
		return localstore.MessageCursor{}
	}
	return parseMediaCursor(value)
}

// collectMedia downloads attachments that have no local file yet, newest
// first, and stores them in the attachment database.
func (c *Client) collectMedia(ctx context.Context) {
	cursor := c.storedMediaCursor(ctx)
	collected := 0
	for collected < mediaFilesPerRun {
		if ctx.Err() != nil {
			return
		}
		pending, err := c.store.MessagesMissingMedia(ctx, cursor, mediaScanPageSize)
		if err != nil {
			return
		}
		if len(pending) == 0 {
			// The scan reached the oldest message. Starting over next time
			// picks up whatever arrived, or failed, in the meantime.
			_ = c.store.SetMetadata(ctx, mediaCollectCursorKey, "")
			break
		}
		for _, item := range pending {
			if ctx.Err() != nil {
				return
			}
			cursor = localstore.MessageCursor{Timestamp: item.Timestamp, MessageID: item.MessageID}
			if item.Size > mediaSizeCeiling {
				continue
			}
			if _, err := c.DownloadMedia(ctx, item.ChatJID, item.MessageID); err != nil {
				// Attachments that WhatsApp no longer serves are common in old
				// history. That is expected and must not stop the scan.
				continue
			}
			collected++
			select {
			case <-ctx.Done():
				return
			case <-time.After(mediaRequestInterval):
			}
			if collected >= mediaFilesPerRun {
				break
			}
		}
		if err := c.store.SetMetadata(ctx, mediaCollectCursorKey, formatMediaCursor(cursor)); err != nil {
			return
		}
	}
	if collected > 0 {
		c.emit(gateway.Event{Name: "media.collected", Data: map[string]int{"files": collected}})
	}
}
