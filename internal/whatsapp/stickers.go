package whatsapp

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

const maxStickerBytes = 1 << 20

// The UI sees PNG first-frame thumbnails. Only the durable original WebP may
// be re-sent; relabelling that PNG as a sticker would corrupt the attachment.
func (c *Client) SendStoredSticker(ctx context.Context, fromChat, messageID, toChat, replyTo string) (model.Message, error) {
	return c.sendStoredSticker(ctx, fromChat, messageID, toChat, replyTo, 0)
}

func (c *Client) sendStoredSticker(ctx context.Context, fromChat, messageID, toChat, replyTo string, forwardingScore int) (model.Message, error) {
	msg, err := c.store.GetMessage(ctx, fromChat, messageID)
	if err != nil {
		return model.Message{}, err
	}
	if msg.Kind != "sticker" || msg.Revoked {
		return model.Message{}, errors.New("this sticker is no longer available")
	}
	tmp, err := os.CreateTemp(c.mediaDir, "sticker-*.webp")
	if err != nil {
		return model.Message{}, err
	}
	defer func() { tmp.Close(); os.Remove(tmp.Name()) }()
	ready := false
	if c.media != nil {
		identities, err := c.store.ChatIdentityJIDs(ctx, msg.ChatJID)
		if err != nil {
			return model.Message{}, err
		}
		for _, identity := range identities {
			info, found, err := c.media.Lookup(ctx, identity, msg.ID)
			if err != nil {
				return model.Message{}, err
			}
			if !found {
				continue
			}
			if info.Size > maxStickerBytes {
				return model.Message{}, errors.New("sticker exceeds 1 MiB")
			}
			if _, err := c.media.WriteTo(ctx, identity, msg.ID, tmp); err != nil {
				return model.Message{}, err
			}
			ready = true
			break
		}
	}
	if !ready && msg.MediaPath != "" {
		// Older installations can have the original cached beside the display
		// thumbnail, but never infer an arbitrary path by stripping suffixes.
		if source, err := os.Open(msg.MediaPath); err == nil {
			header := make([]byte, 12)
			if _, err := io.ReadFull(source, header); err == nil && string(header[:4]) == "RIFF" && string(header[8:]) == "WEBP" {
				_, _ = source.Seek(0, io.SeekStart)
				n, err := io.Copy(tmp, io.LimitReader(source, maxStickerBytes+1))
				ready = err == nil && n <= maxStickerBytes
			}
			source.Close()
		}
	}
	if !ready {
		payload, available, err := c.store.MediaPayload(ctx, msg.ChatJID, msg.ID)
		if err != nil {
			return model.Message{}, err
		}
		var raw waE2E.Message
		if !available || proto.Unmarshal(payload, &raw) != nil || raw.GetStickerMessage() == nil {
			return model.Message{}, errors.New("the original sticker is unavailable; download it in its conversation first")
		}
		sticker := raw.GetStickerMessage()
		if sticker.GetFileLength() > maxStickerBytes || c.wa == nil || !c.wa.IsConnected() {
			return model.Message{}, errors.New("cannot download this sticker; check the connection and file size")
		}
		data, err := c.wa.Download(ctx, sticker)
		if err != nil {
			return model.Message{}, err
		}
		if len(data) > maxStickerBytes {
			return model.Message{}, errors.New("sticker exceeds 1 MiB")
		}
		if err := tmp.Truncate(0); err != nil {
			return model.Message{}, err
		}
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return model.Message{}, err
		}
		if _, err := tmp.Write(data); err != nil {
			return model.Message{}, err
		}
	}
	if err := tmp.Close(); err != nil {
		return model.Message{}, err
	}
	return c.SendMedia(ctx, gateway.MediaRequest{ChatJID: toChat, Path: tmp.Name(), ReplyTo: replyTo, Sticker: true, ForwardingScore: forwardingScore})
}
