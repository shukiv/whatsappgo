package whatsapp

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
	"go.mau.fi/whatsmeow/proto/waE2E"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// Inspect protection flags only. Empty wrappers are meaningful too: the
// linked device may receive the envelope without the protected content.
func isViewOnce(raw *waE2E.Message) bool {
	for depth := 0; raw != nil && depth < 16; depth++ {
		if raw.ViewOnceMessage != nil || raw.ViewOnceMessageV2 != nil || raw.ViewOnceMessageV2Extension != nil ||
			raw.GetImageMessage().GetViewOnce() || raw.GetVideoMessage().GetViewOnce() ||
			raw.GetAudioMessage().GetViewOnce() || raw.GetExtendedTextMessage().GetViewOnce() {
			return true
		}
		switch {
		case raw.DeviceSentMessage != nil:
			raw = raw.GetDeviceSentMessage().GetMessage()
		case raw.EphemeralMessage != nil:
			raw = raw.GetEphemeralMessage().GetMessage()
		case raw.EditedMessage != nil:
			raw = raw.GetEditedMessage().GetMessage()
		default:
			return false
		}
	}
	return false
}

func eventIsViewOnce(evt *waEvents.Message) bool {
	return evt.IsViewOnce || evt.IsViewOnceV2 || evt.IsViewOnceV2Extension || isViewOnce(evt.Message) || isViewOnce(evt.RawMessage)
}

// Older builds retained some marked media payloads as ordinary photos. Repair
// their metadata before starting the client, so cached files cannot become an
// alternate way of opening protected content. No media is fetched or decoded.
func (c *Client) redactStoredViewOnce(ctx context.Context) error {
	const key = "view_once_metadata_repair_v1"
	if _, done, err := c.store.Metadata(ctx, key); err != nil || done {
		return err
	}
	cursor := localstore.MediaCursor{}
	for {
		items, err := c.store.MediaPayloadPage(ctx, cursor)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return c.store.SetMetadata(ctx, key, "done")
		}
		for _, item := range items {
			cursor = localstore.MediaCursor{ChatJID: item.ChatJID, MessageID: item.MessageID}
			var raw waE2E.Message
			if proto.Unmarshal(item.Payload, &raw) != nil || !isViewOnce(&raw) {
				continue
			}
			message, err := c.store.GetMessage(ctx, item.ChatJID, item.MessageID)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if err := c.store.UpsertMessage(ctx, model.ViewOncePlaceholder(message), "", false); err != nil {
				return err
			}
		}
	}
}
