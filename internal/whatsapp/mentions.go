package whatsapp

import (
	"context"
	"strings"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// ResolveMessageMentions is an optional, local-only read-side enrichment. It
// also repairs presentation of old rows whose mention context was discarded.
func (c *Client) ResolveMessageMentions(ctx context.Context, m model.Message) model.Message {
	m = c.withMentionNames(ctx, m)
	if m.Revoked || m.Kind == "view_once" || !strings.HasSuffix(m.ChatJID, "@g.us") {
		return m
	}
	seen := map[string]bool{}
	for _, mention := range m.Mentions {
		seen[strings.SplitN(mention.JID, "@", 2)[0]] = true
	}
	for _, span := range model.MentionTokens(m.Body) {
		id := m.Body[span[0]+1 : span[1]]
		if seen[id] || len(m.Mentions) >= 128 {
			continue
		}
		seen[id] = true
		candidate := c.withMentionNames(ctx, model.Message{ChatJID: m.ChatJID, Body: m.Body,
			Mentions: []model.Mention{{JID: id + "@lid"}, {JID: id + "@s.whatsapp.net"}}})
		var found model.Mention
		ambiguous := false
		for _, mention := range candidate.Mentions {
			if mention.Name == "" {
				continue
			}
			if found.Name != "" && found.Name != mention.Name {
				ambiguous = true
				break
			}
			found = mention
		}
		if !ambiguous && found.Name != "" {
			m.Mentions = append(m.Mentions, found)
		}
	}
	return m
}

func (c *Client) withMentionNames(ctx context.Context, m model.Message) model.Message {
	if m.Revoked || m.Kind == "view_once" || !strings.HasSuffix(m.ChatJID, "@g.us") {
		m.Mentions = nil
		return m
	}
	if c.store != nil {
		m = c.store.ResolveMentionNames(ctx, m)
	}
	if c.wa == nil || c.wa.Store == nil {
		return m
	}
	for i := range m.Mentions {
		if m.Mentions[i].Name != "" {
			continue
		}
		jid, err := types.ParseJID(m.Mentions[i].JID)
		if err != nil {
			continue
		}
		if c.isOwnIdentity(jid) {
			m.Mentions[i].Name = firstNonEmpty(c.wa.Store.PushName, "You")
			continue
		}
		identities := []types.JID{jid}
		if c.wa.Store.LIDs != nil {
			var alias types.JID
			if jid.Server == types.HiddenUserServer {
				alias, _ = c.wa.Store.LIDs.GetPNForLID(ctx, jid)
			} else {
				alias, _ = c.wa.Store.LIDs.GetLIDForPN(ctx, jid)
			}
			if !alias.IsEmpty() {
				identities = append(identities, alias)
			}
		}
		if c.wa.Store.Contacts != nil {
			for _, identity := range identities {
				if contact, err := c.wa.Store.Contacts.GetContact(ctx, identity); err == nil {
					m.Mentions[i].Name = strings.TrimSpace(firstNonEmpty(contact.FullName, contact.FirstName, contact.BusinessName, contact.PushName))
					if m.Mentions[i].Name != "" {
						break
					}
				}
			}
		}
	}
	return m
}

func textMessagePayload(req gateway.TextRequest, contextInfo *waE2E.ContextInfo) *waE2E.Message {
	if contextInfo == nil && req.Preview.URL == "" && len(req.Mentions) == 0 {
		return &waE2E.Message{Conversation: proto.String(req.Text)}
	}
	if len(req.Mentions) > 0 {
		if contextInfo == nil {
			contextInfo = &waE2E.ContextInfo{}
		}
		for _, mention := range req.Mentions {
			contextInfo.MentionedJID = append(contextInfo.MentionedJID, mention.JID)
		}
	}
	extended := &waE2E.ExtendedTextMessage{Text: proto.String(req.Text), ContextInfo: contextInfo}
	if req.Preview.URL != "" {
		extended.MatchedText = proto.String(req.Preview.URL)
		extended.Title = proto.String(req.Preview.Title)
		extended.Description = proto.String(req.Preview.Description)
		extended.JPEGThumbnail = req.Preview.Thumbnail
		if len(req.Preview.Thumbnail) > 0 {
			extended.PreviewType = waE2E.ExtendedTextMessage_IMAGE.Enum()
		}
	}
	return &waE2E.Message{ExtendedTextMessage: extended}
}
