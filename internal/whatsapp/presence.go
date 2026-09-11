package whatsapp

import (
	"context"
	"strings"

	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// chatPresenceData resolves group typing names from local caches only. These
// frequent, transient events must not fetch group rosters or create chat rows.
func (c *Client) chatPresenceData(evt *waEvents.ChatPresence) map[string]any {
	ctx := context.Background()
	chat, sender := evt.Chat.ToNonAD(), evt.Sender.ToNonAD()
	chatJID, senderJID := chat.String(), sender.String()
	if c.store != nil {
		chatJID = c.store.CanonicalChatJID(ctx, chatJID)
		senderJID = c.store.CanonicalChatJID(ctx, senderJID)
	}
	data := map[string]any{"chat_jid": chatJID, "sender_jid": senderJID,
		"state": string(evt.State), "media": string(evt.Media)}
	if chat.Server != types.GroupServer {
		return data
	}
	if sender.IsEmpty() || (sender.Server != types.DefaultUserServer && sender.Server != types.HiddenUserServer) || c.isOwnIdentity(sender) {
		return nil
	}

	identities := []types.JID{sender}
	if canonical, err := types.ParseJID(senderJID); err == nil && canonical != sender {
		identities = append(identities, canonical)
	}
	// Normalize both PN and LID events to the same member, including paused
	// events. Whatsmeow's LID store is a local mapping, not a network lookup.
	if c.wa != nil && c.wa.Store != nil && c.wa.Store.LIDs != nil {
		var alias types.JID
		if sender.Server == types.HiddenUserServer {
			alias, _ = c.wa.Store.LIDs.GetPNForLID(ctx, sender)
		} else {
			alias, _ = c.wa.Store.LIDs.GetLIDForPN(ctx, sender)
		}
		if !alias.IsEmpty() {
			alias = alias.ToNonAD()
			identities = append(identities, alias)
			if sender.Server == types.HiddenUserServer {
				senderJID = alias.String()
			}
			if c.store != nil {
				senderJID = c.store.CanonicalChatJID(ctx, senderJID)
			}
			data["sender_jid"] = senderJID
		}
	}
	if evt.State != types.ChatPresenceComposing {
		return data
	}
	for _, jid := range identities {
		if c.store != nil {
			if known, err := c.store.GetChat(ctx, jid.String()); err == nil {
				name := strings.TrimSpace(known.Title)
				if name != "" && name != known.JID && name != displayJID(known.JID) && name != "+"+displayJID(known.JID) {
					data["sender_name"] = name
					return data
				}
			}
		}
	}
	if c.wa != nil && c.wa.Store != nil && c.wa.Store.Contacts != nil {
		for _, jid := range identities {
			if contact, err := c.wa.Store.Contacts.GetContact(ctx, jid); err == nil {
				name := strings.TrimSpace(firstNonEmpty(contact.FullName, contact.FirstName, contact.BusinessName, contact.PushName))
				if name != "" {
					data["sender_name"] = name
					return data
				}
			}
		}
	}
	for _, jid := range identities {
		if jid.Server == types.DefaultUserServer {
			data["sender_name"] = "+" + jid.User
			break
		}
	}
	// An opaque LID is not a phone number. Leave it unnamed so the desktop can
	// use its cached group roster or a clearly labelled member fallback.
	return data
}
