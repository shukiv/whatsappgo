package whatsapp

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"github.com/shuki/whatsappgo/internal/gateway"
)

// mutedForever is what WhatsApp stores when a conversation is muted with no
// end date. It is far enough ahead to mean "until this is undone".
const mutedForever = 100 * 365 * 24 * time.Hour

// Pinning, muting, archiving and marking a conversation read are app-state
// changes: they belong to the account, not to this device, so each one is sent
// to WhatsApp and applied locally so the list responds immediately.

func (c *Client) SetChatPinned(ctx context.Context, chatJID string, pinned bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	if err := c.wa.SendAppState(ctx, appstate.BuildPin(target, pinned)); err != nil {
		return err
	}
	if err := c.store.UpdateChatPinned(ctx, chatJID, pinned); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID, "pinned": pinned}})
	return nil
}

func (c *Client) SetChatMuted(ctx context.Context, chatJID string, muted bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	if err := c.wa.SendAppState(ctx, appstate.BuildMute(target, muted, mutedForever)); err != nil {
		return err
	}
	var until int64
	if muted {
		until = time.Now().Add(mutedForever).UnixMilli()
	}
	if err := c.store.UpdateChatMuted(ctx, chatJID, until); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID, "muted": muted}})
	return nil
}

func (c *Client) SetChatArchived(ctx context.Context, chatJID string, archived bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	timestamp, key := c.lastMessageAnchor(ctx, chatJID)
	if err := c.wa.SendAppState(ctx, appstate.BuildArchive(target, archived, timestamp, key)); err != nil {
		return err
	}
	if err := c.store.UpdateChatArchived(ctx, chatJID, archived); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID, "archived": archived}})
	return nil
}

func (c *Client) SetChatRead(ctx context.Context, chatJID string, read bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	timestamp, key := c.lastMessageAnchor(ctx, chatJID)
	if err := c.wa.SendAppState(ctx, appstate.BuildMarkChatAsRead(target, read, timestamp, key)); err != nil {
		return err
	}
	if read {
		if err := c.store.MarkChatRead(ctx, chatJID); err != nil {
			return err
		}
	} else if err := c.store.MarkChatUnread(ctx, chatJID); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID, "read": read}})
	return nil
}

// lastMessageAnchor describes the newest message of a conversation, which
// WhatsApp requires alongside an archive or read change.
func (c *Client) lastMessageAnchor(ctx context.Context, chatJID string) (time.Time, *waCommon.MessageKey) {
	newest, err := c.store.NewestMessage(ctx, chatJID)
	if err != nil || newest.ID == "" {
		return time.Now(), nil
	}
	return time.UnixMilli(newest.Timestamp), &waCommon.MessageKey{
		RemoteJID: proto.String(chatJID),
		FromMe:    proto.Bool(newest.FromMe),
		ID:        proto.String(newest.ID),
	}
}
