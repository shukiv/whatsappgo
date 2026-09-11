package store

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/shukiv/whatsappgo/internal/model"
)

func packMentions(mentions []model.Mention) string {
	if len(mentions) == 0 {
		return ""
	}
	data, _ := json.Marshal(mentions)
	return string(data)
}

func unpackMentions(data string) []model.Mention {
	var mentions []model.Mention
	_ = json.Unmarshal([]byte(data), &mentions)
	return mentions
}

// ResolveMentionNames uses local names and PN/LID aliases only. Old builds
// discarded mention metadata: for those rows a known identifier can be
// labelled at read time, without rewriting history or inventing outgoing tags.
func (s *Store) ResolveMentionNames(ctx context.Context, m model.Message) model.Message {
	if m.Revoked || m.Kind == "view_once" || !strings.HasSuffix(m.ChatJID, "@g.us") {
		m.Mentions = nil
		return m
	}
	names := map[string]string{}
	knownName := func(jid string) string {
		if name, ok := names[jid]; ok {
			return name
		}
		chat, err := s.GetChat(ctx, jid)
		name := strings.TrimSpace(chat.Title)
		if err == nil && name != "" && name != chat.JID && name != displayJID(chat.JID) && name != "+"+displayJID(chat.JID) {
			names[jid] = name
			return name
		}
		var sender string
		_ = s.db.QueryRowContext(ctx, `SELECT sender_name FROM messages WHERE chat_jid=? AND sender_jid=? AND TRIM(sender_name)<>'' ORDER BY timestamp DESC LIMIT 1`, m.ChatJID, jid).Scan(&sender)
		names[jid] = strings.TrimSpace(sender)
		return names[jid]
	}
	if len(m.Mentions) == 0 {
		seen := map[string]bool{}
		for _, span := range model.MentionTokens(m.Body) {
			id := m.Body[span[0]+1 : span[1]]
			if seen[id] || len(m.Mentions) >= 128 {
				continue
			}
			seen[id] = true
			lid, pn := id+"@lid", id+"@s.whatsapp.net"
			ln, pnName := knownName(lid), knownName(pn)
			// Do not choose between conflicting identities with the same digits.
			if ln != "" && (pnName == "" || pnName == ln) {
				m.Mentions = append(m.Mentions, model.Mention{JID: lid, Name: ln})
			} else if pnName != "" && ln == "" {
				m.Mentions = append(m.Mentions, model.Mention{JID: pn, Name: pnName})
			}
		}
	}
	for i := range m.Mentions {
		if name := knownName(m.Mentions[i].JID); name != "" {
			m.Mentions[i].Name = name
		}
	}
	return m
}
