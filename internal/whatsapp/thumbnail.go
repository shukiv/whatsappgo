package whatsapp

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	"github.com/shuki/whatsappgo/internal/gateway"
	"github.com/shuki/whatsappgo/internal/model"
	localstore "github.com/shuki/whatsappgo/internal/store"
)

// maxInlineThumbnail bounds what is written to the cache. WhatsApp's inline
// previews are a few kilobytes; anything larger is not a thumbnail.
const maxInlineThumbnail = 1 << 20

const thumbnailBackfillMetadataKey = "media_thumbnail_backfill_v1"

// thumbnailFromMessage returns the small preview picture WhatsApp embeds
// directly in a media message, together with the extension for its format.
// The preview arrives with the message itself, so a photo or video can be
// shown before the full file has been downloaded.
func thumbnailFromMessage(msg *waE2E.Message) ([]byte, string) {
	if msg == nil {
		return nil, ""
	}
	switch {
	case msg.GetImageMessage() != nil:
		return msg.GetImageMessage().GetJPEGThumbnail(), ".jpg"
	case msg.GetVideoMessage() != nil:
		return msg.GetVideoMessage().GetJPEGThumbnail(), ".jpg"
	case msg.GetStickerMessage() != nil:
		return msg.GetStickerMessage().GetPngThumbnail(), ".png"
	case msg.GetDocumentMessage() != nil:
		return msg.GetDocumentMessage().GetJPEGThumbnail(), ".jpg"
	}
	return nil, ""
}

// cacheThumbnail stores a message's inline preview and returns its path. An
// empty result means the message carries no usable preview.
func (c *Client) cacheThumbnail(msg model.Message, raw *waE2E.Message) string {
	data, ext := thumbnailFromMessage(raw)
	if len(data) == 0 || len(data) > maxInlineThumbnail {
		return ""
	}
	// A truncated or unrecognised payload would render as a broken picture.
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return ""
	}
	dir := filepath.Join(c.mediaDir, "thumbnails")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	path := filepath.Join(dir, safeName(msg.ChatJID+"-"+msg.ID)+ext)
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path
	}
	tmp, err := os.CreateTemp(dir, "thumb-*")
	if err != nil {
		return ""
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return ""
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return ""
	}
	if err := tmp.Close(); err != nil {
		return ""
	}
	if err := os.Rename(tmpName, path); err != nil {
		return ""
	}
	return path
}

// withCachedThumbnail attaches the message's inline preview so it is stored by
// the same write that stores the message.
func (c *Client) withCachedThumbnail(msg model.Message, raw *waE2E.Message) model.Message {
	if msg.MediaThumbnail != "" {
		return msg
	}
	if path := c.cacheThumbnail(msg, raw); path != "" {
		msg.MediaThumbnail = path
	}
	return msg
}

// backfillThumbnails extracts previews for media that was stored before
// thumbnails were cached, using the message payloads already on disk. It runs
// once per profile and never contacts WhatsApp.
func (c *Client) backfillThumbnails() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, done, err := c.store.Metadata(ctx, thumbnailBackfillMetadataKey); err != nil || done {
		return
	}
	updated := 0
	// Paging by a stable cursor rather than by "still missing a thumbnail"
	// keeps the scan moving past messages that carry no usable preview.
	cursor := localstore.MediaCursor{}
	for {
		pending, err := c.store.MessagesMissingThumbnails(ctx, cursor, 200)
		if err != nil || len(pending) == 0 {
			break
		}
		for _, item := range pending {
			var raw waE2E.Message
			if err := proto.Unmarshal(item.Payload, &raw); err == nil {
				if path := c.cacheThumbnail(model.Message{ChatJID: item.ChatJID, ID: item.MessageID}, &raw); path != "" {
					if err := c.store.UpdateMediaThumbnail(ctx, item.ChatJID, item.MessageID, path); err != nil {
						return
					}
					updated++
				}
			}
			cursor = localstore.MediaCursor{ChatJID: item.ChatJID, MessageID: item.MessageID}
		}
		if ctx.Err() != nil {
			return
		}
	}
	if err := c.store.SetMetadata(ctx, thumbnailBackfillMetadataKey, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return
	}
	if updated > 0 {
		c.emit(gateway.Event{Name: "chat.updated"})
	}
}
