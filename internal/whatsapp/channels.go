package whatsapp

import (
	"context"
	"errors"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
)

// ChannelMessages reads a channel's recent posts and keeps them the way every
// other message is kept, so the conversation view draws them, their pictures
// download, and opening the channel again needs no network.
//
// A channel's posts are not delivered to a linked device the way a
// conversation's messages are. They sit where WhatsApp keeps them and nothing
// asks for them until a reader opens the channel, which is why a followed
// channel used to open onto nothing at all.
func (c *Client) ChannelMessages(ctx context.Context, channelJID string, count int) ([]model.Message, error) {
	jid, err := types.ParseJID(strings.TrimSpace(channelJID))
	if err != nil {
		return nil, err
	}
	if jid.Server != types.NewsletterServer {
		return nil, errors.New("that address is not a channel")
	}
	// A page big enough to fill the view and small enough that opening a
	// channel is not a download.
	if count <= 0 || count > 100 {
		count = 50
	}
	fetch := c.fetchNewsletterMessages
	if fetch == nil {
		if c.wa == nil {
			return nil, errors.New("WhatsApp is disconnected")
		}
		fetch = c.wa.GetNewsletterMessages
	}
	posts, err := fetch(ctx, jid, &whatsmeow.GetNewsletterMessagesParams{Count: count})
	if err != nil {
		return nil, err
	}
	// The channel itself is the conversation's name. Without it the chat row
	// this creates is titled with the bare numeric address.
	title := c.channelTitle(ctx, jid)
	stored := make([]model.Message, 0, len(posts))
	for _, post := range posts {
		if post == nil || post.Message == nil {
			continue
		}
		msg := messageFromEvent(&waEvents.Message{
			Info: types.MessageInfo{
				// A post has no author of its own: the channel wrote it, and
				// the conversation already carries that name.
				MessageSource: types.MessageSource{Chat: jid, Sender: jid},
				ID:            post.MessageID,
				Timestamp:     post.Timestamp,
				Type:          post.Type,
			},
			Message: post.Message,
		})
		if msg.ID == "" {
			continue
		}
		msg = c.withCachedThumbnail(msg, post.Message)
		msg = c.withCachedLinkPreview(msg, post.Message)
		if err := c.store.UpsertMessage(ctx, msg, title, false); err != nil {
			continue
		}
		// The keys to a post's picture live in the payload, and this is the
		// only moment it is held. Without this the picture can never be
		// fetched.
		c.rememberMediaPayload(msg, post.Message)
		stored = append(stored, msg)
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": jid.String()}})
	return stored, nil
}

// channelTitle answers with the channel's name, or empty when it cannot be
// read. An unnamed conversation is better than a wrongly named one.
func (c *Client) channelTitle(ctx context.Context, jid types.JID) string {
	info := c.newsletterInfo
	if info == nil {
		if c.wa == nil {
			return ""
		}
		info = c.wa.GetNewsletterInfo
	}
	meta, err := info(ctx, jid)
	if err != nil || meta == nil {
		return ""
	}
	return strings.TrimSpace(meta.ThreadMeta.Name.Text)
}
