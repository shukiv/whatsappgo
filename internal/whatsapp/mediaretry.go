package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waMmsRetry"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/shukiv/whatsappgo/internal/model"
)

const mediaRetryTimeout = 30 * time.Second

type mediaRetryWaiter struct {
	mediaKey   []byte
	done       chan struct{}
	directPath string
	err        error
}

func isExpiredMediaDownload(err error) bool {
	return errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith403) ||
		errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith404) ||
		errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith410)
}

func setMediaDirectPath(raw *waE2E.Message, directPath string) bool {
	switch {
	case raw.GetImageMessage() != nil:
		raw.GetImageMessage().DirectPath = proto.String(directPath)
	case raw.GetVideoMessage() != nil:
		raw.GetVideoMessage().DirectPath = proto.String(directPath)
	case raw.GetAudioMessage() != nil:
		raw.GetAudioMessage().DirectPath = proto.String(directPath)
	case raw.GetDocumentMessage() != nil:
		raw.GetDocumentMessage().DirectPath = proto.String(directPath)
	case raw.GetStickerMessage() != nil:
		raw.GetStickerMessage().DirectPath = proto.String(directPath)
	default:
		return false
	}
	return true
}

func (c *Client) awaitMediaRetryPath(ctx context.Context, msg model.Message, media whatsmeow.DownloadableMessage) (string, error) {
	id := types.MessageID(msg.ID)
	c.mu.Lock()
	if c.mediaRetries == nil {
		c.mediaRetries = make(map[types.MessageID]*mediaRetryWaiter)
	}
	waiter, exists := c.mediaRetries[id]
	if !exists {
		waiter = &mediaRetryWaiter{mediaKey: append([]byte(nil), media.GetMediaKey()...), done: make(chan struct{})}
		c.mediaRetries[id] = waiter
	}
	c.mu.Unlock()

	if !exists {
		info, err := c.mediaRetryMessageInfo(msg)
		if err != nil {
			c.completeMediaRetry(id, waiter, "", err)
		} else if err = c.wa.SendMediaRetryReceipt(ctx, info, waiter.mediaKey); err != nil {
			c.completeMediaRetry(id, waiter, "", err)
		}
	}

	timer := time.NewTimer(mediaRetryTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		if !exists {
			c.completeMediaRetry(id, waiter, "", ctx.Err())
		}
		return "", ctx.Err()
	case <-timer.C:
		err := errors.New("timed out waiting for the linked phone to refresh media")
		c.completeMediaRetry(id, waiter, "", err)
		return "", err
	case <-waiter.done:
		return waiter.directPath, waiter.err
	}
}

func (c *Client) mediaRetryMessageInfo(msg model.Message) (*types.MessageInfo, error) {
	chat, err := types.ParseJID(msg.ChatJID)
	if err != nil {
		return nil, err
	}
	sender, err := types.ParseJID(msg.SenderJID)
	if err != nil || sender.IsEmpty() {
		if msg.FromMe {
			sender, err = types.ParseJID(c.selfJID())
		} else {
			sender, err = chat, nil
		}
	}
	if err != nil {
		return nil, err
	}
	return &types.MessageInfo{
		MessageSource: types.MessageSource{Chat: chat, Sender: sender, IsFromMe: msg.FromMe, IsGroup: chat.Server == types.GroupServer},
		ID:            types.MessageID(msg.ID),
		Timestamp:     time.UnixMilli(msg.Timestamp),
	}, nil
}

func (c *Client) handleMediaRetry(evt *waEvents.MediaRetry) {
	c.mu.RLock()
	waiter := c.mediaRetries[evt.MessageID]
	c.mu.RUnlock()
	if waiter == nil {
		return
	}
	notification, err := whatsmeow.DecryptMediaRetryNotification(evt, waiter.mediaKey)
	if err == nil && notification.GetResult() != waMmsRetry.MediaRetryNotification_SUCCESS {
		err = fmt.Errorf("phone could not refresh media: %s", notification.GetResult())
	}
	path := ""
	if err == nil {
		path = notification.GetDirectPath()
		if path == "" {
			err = errors.New("phone returned an empty refreshed media path")
		}
	}
	c.completeMediaRetry(evt.MessageID, waiter, path, err)
}

func (c *Client) completeMediaRetry(id types.MessageID, waiter *mediaRetryWaiter, directPath string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if current := c.mediaRetries[id]; current != waiter {
		return
	}
	delete(c.mediaRetries, id)
	waiter.directPath = directPath
	waiter.err = err
	close(waiter.done)
}
