package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
)

// inviteKey pulls the code out of an invite link, accepting either the whole
// link or the code on its own. WhatsApp's own share sheets hand out the link,
// and asking someone to trim it by hand is a way to make them get it wrong.
func inviteKey(value, prefix string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("a link or code is required")
	}
	if at := strings.Index(trimmed, prefix); at >= 0 {
		trimmed = trimmed[at+len(prefix):]
	}
	if at := strings.IndexAny(trimmed, "?#/"); at >= 0 {
		trimmed = trimmed[:at]
	}
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return "", errors.New("that link carries no invite code")
	}
	return trimmed, nil
}

// CreateChannel makes a new channel. WhatsApp calls these newsletters.
func (c *Client) CreateChannel(ctx context.Context, name, description string) (model.Channel, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return model.Channel{}, errors.New("a channel name is required")
	}
	metadata, err := c.wa.CreateNewsletter(ctx, whatsmeow.CreateNewsletterParams{
		Name:        trimmed,
		Description: strings.TrimSpace(description),
	})
	if err != nil {
		return model.Channel{}, err
	}
	if metadata == nil {
		return model.Channel{}, errors.New("WhatsApp created the channel but described nothing")
	}
	channel := model.Channel{
		JID:         metadata.ID.String(),
		Name:        metadata.ThreadMeta.Name.Text,
		Description: metadata.ThreadMeta.Description.Text,
	}
	c.emit(gateway.Event{Name: "channel.updated", Data: map[string]any{"jid": channel.JID}})
	return channel, nil
}

// FollowChannelLink follows a channel from its invite link.
//
// There is no directory search in this protocol - the PWA's "Discover" is a
// server-side surface with no client API - so a link is how a channel is found
// from here. The invite is read first so the answer can name what was followed.
func (c *Client) FollowChannelLink(ctx context.Context, link string) (model.Channel, error) {
	key, err := inviteKey(link, "whatsapp.com/channel/")
	if err != nil {
		return model.Channel{}, err
	}
	metadata, err := c.wa.GetNewsletterInfoWithInvite(ctx, key)
	if err != nil {
		return model.Channel{}, fmt.Errorf("look up channel: %w", err)
	}
	if metadata == nil {
		return model.Channel{}, errors.New("that link matches no channel")
	}
	if err := c.wa.FollowNewsletter(ctx, metadata.ID); err != nil {
		return model.Channel{}, err
	}
	channel := model.Channel{
		JID:             metadata.ID.String(),
		Name:            metadata.ThreadMeta.Name.Text,
		Description:     metadata.ThreadMeta.Description.Text,
		SubscriberCount: metadata.ThreadMeta.SubscriberCount,
	}
	c.emit(gateway.Event{Name: "channel.updated", Data: map[string]any{"jid": channel.JID}})
	return channel, nil
}

// CreateCommunity makes a community. A community is a group with the parent
// flag set; WhatsApp creates its announcement group itself.
func (c *Client) CreateCommunity(ctx context.Context, name string) (model.Community, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return model.Community{}, errors.New("a community name is required")
	}
	info, err := c.wa.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:        trimmed,
		GroupParent: types.GroupParent{IsParent: true},
	})
	if err != nil {
		return model.Community{}, err
	}
	community := model.Community{JID: info.JID.String(), Name: info.Name}
	c.emit(gateway.Event{Name: "community.updated", Data: map[string]any{"jid": community.JID}})
	return community, nil
}

// JoinGroupLink joins a group from its invite link.
func (c *Client) JoinGroupLink(ctx context.Context, link string) (model.Chat, error) {
	key, err := inviteKey(link, "chat.whatsapp.com/")
	if err != nil {
		return model.Chat{}, err
	}
	jid, err := c.wa.JoinGroupWithLink(ctx, key)
	if err != nil {
		return model.Chat{}, err
	}
	chat := model.Chat{JID: jid.String(), Title: jid.User, IsGroup: true}
	if info, infoErr := c.wa.GetGroupInfo(ctx, jid); infoErr == nil && info != nil {
		chat.Title = info.Name
	}
	if err := c.store.UpsertChat(ctx, chat); err != nil {
		return model.Chat{}, err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chat.JID}})
	return chat, nil
}
