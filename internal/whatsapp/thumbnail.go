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

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/linkpreview"
	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

// maxInlineThumbnail bounds what is written to the cache. WhatsApp's inline
// previews are a few kilobytes; anything larger is not a thumbnail.
const maxInlineThumbnail = 1 << 20

const thumbnailBackfillMetadataKey = "media_thumbnail_backfill_v1"
const linkPreviewBackfillMetadataKey = "youtube_link_preview_backfill_v3"

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
	case msg.GetLocationMessage() != nil:
		return msg.GetLocationMessage().GetJPEGThumbnail(), ".jpg"
	}
	return nil, ""
}

// cacheThumbnail stores a message's inline preview and returns its path. An
// empty result means the message carries no usable preview.
func (c *Client) cacheThumbnail(msg model.Message, raw *waE2E.Message) string {
	data, ext := thumbnailFromMessage(raw)
	return c.writeThumbnail(msg.ChatJID+"-"+msg.ID, data, ext)
}

// linkThumbnailFromMessage returns the picture a sender's link preview carries.
func linkThumbnailFromMessage(msg *waE2E.Message) []byte {
	if extended := msg.GetExtendedTextMessage(); extended != nil {
		return extended.GetJPEGThumbnail()
	}
	return nil
}

// withCachedLinkPreview stores the picture that belongs to a link preview.
func (c *Client) withCachedLinkPreview(msg model.Message, raw *waE2E.Message) model.Message {
	if msg.LinkURL == "" || msg.LinkThumbnail != "" {
		return msg
	}
	if path := c.writeThumbnail(msg.ChatJID+"-"+msg.ID+"-link", linkThumbnailFromMessage(raw), ".jpg"); path != "" {
		msg.LinkThumbnail = path
	}
	return msg
}

func (c *Client) withCachedOutgoingLinkPreview(msg model.Message, preview model.LinkPreview) model.Message {
	if preview.URL == "" || len(preview.Thumbnail) == 0 {
		return msg
	}
	extension := ".jpg"
	if preview.ThumbnailMIME == "image/png" {
		extension = ".png"
	}
	if path := c.writeThumbnail(msg.ChatJID+"-"+msg.ID+"-link", preview.Thumbnail, extension); path != "" {
		msg.LinkThumbnail = path
	}
	return msg
}

func (c *Client) writeThumbnail(key string, data []byte, ext string) string {
	return c.writeThumbnailFile(key, data, ext, false)
}

func (c *Client) writeThumbnailReplacing(key string, data []byte, ext string) string {
	return c.writeThumbnailFile(key, data, ext, true)
}

func (c *Client) writeThumbnailFile(key string, data []byte, ext string, replace bool) string {
	if len(data) == 0 || len(data) > maxInlineThumbnail || ext == "" {
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
	path := filepath.Join(dir, safeName(key)+ext)
	if info, err := os.Stat(path); !replace && err == nil && info.Size() > 0 {
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

// backfillLinkPreviews repairs historical YouTube cards that WhatsApp synced
// without an inline picture. It is deliberately a one-time YouTube-only pass:
// arbitrary links in old private conversations are never contacted.
func (c *Client) backfillLinkPreviews() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, done, err := c.store.Metadata(ctx, linkPreviewBackfillMetadataKey); err != nil || done {
		return
	}
	resolver := c.resolveLinkPreview
	if resolver == nil {
		resolver = linkpreview.Resolve
	}
	cursor := localstore.MediaCursor{}
	updatedChats := make(map[string]struct{})
	updated := 0
	for {
		pending, err := c.store.MessagesForLinkPreviewBackfill(ctx, cursor, 100)
		if err != nil || len(pending) == 0 {
			break
		}
		for _, item := range pending {
			text := item.LinkURL
			if text == "" {
				text = item.Body
			}
			if linkpreview.IsYouTube(text) {
				if preview, err := resolver(ctx, text); err == nil {
					// Use a distinct cache key for the high-resolution migration.
					// Changing the URL makes an already-visible QML Image discard its
					// in-memory copy immediately when the message refresh event arrives.
					path := c.writeThumbnailReplacing(item.ChatJID+"-"+item.MessageID+"-link-hq", preview.Thumbnail, ".jpg")
					if path != "" {
						if err := c.store.UpdateLinkPreview(ctx, item.ChatJID, item.MessageID, preview.URL, preview.Title, preview.Description, path); err != nil {
							return
						}
						updated++
						updatedChats[item.ChatJID] = struct{}{}
					}
				}
			}
			cursor = localstore.MediaCursor{ChatJID: item.ChatJID, MessageID: item.MessageID}
		}
		if ctx.Err() != nil {
			return
		}
	}
	if err := c.store.SetMetadata(ctx, linkPreviewBackfillMetadataKey, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return
	}
	if updated > 0 {
		chatJIDs := make([]string, 0, len(updatedChats))
		for chatJID := range updatedChats {
			chatJIDs = append(chatJIDs, chatJID)
		}
		c.emit(gateway.Event{Name: "history.synced", Data: map[string]any{"messages": updated, "chat_jids": chatJIDs}})
	}
}
