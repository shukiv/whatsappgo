package store

import (
	"context"
	"strings"

	"github.com/shukiv/whatsappgo/internal/model"
)

// ShareableContacts includes recent and archived direct contacts, unlike the
// sidebar's no-conversation-only search. A LID is never treated as a phone.
// One bounded query avoids loading the address book or per-row message counts.
func (s *Store) ShareableContacts(ctx context.Context, query string) ([]model.Contact, error) {
	like := "%" + escapeLike(strings.TrimSpace(query)) + "%"
	phoneQuery := strings.TrimSpace(query)
	if strings.Trim(phoneQuery, "+0123456789 ().-") == "" {
		phoneQuery = strings.NewReplacer("+", "", " ", "", "(", "", ")", "", ".", "", "-", "").Replace(phoneQuery)
		if phoneQuery == "" && strings.TrimSpace(query) != "" {
			phoneQuery = query
		}
	}
	phoneLike := "%" + escapeLike(phoneQuery) + "%"
	rows, err := s.db.QueryContext(ctx, `WITH candidates AS (
	 SELECT c.jid, COALESCE(NULLIF(TRIM(c.local_title),''),c.title,'') AS name, c.avatar_path,
	 CASE WHEN c.jid LIKE '%@s.whatsapp.net' THEN c.jid ELSE COALESCE(
	  (SELECT a.alias_jid FROM chat_aliases a WHERE a.canonical_jid=c.jid
	   AND a.alias_jid LIKE '%@s.whatsapp.net' ORDER BY a.alias_jid LIMIT 1),'') END AS phone_jid
	 FROM chats c WHERE c.is_group=0 AND (c.jid LIKE '%@lid' OR c.jid LIKE '%@s.whatsapp.net')
	 AND NOT EXISTS (SELECT 1 FROM chat_aliases a WHERE a.alias_jid=c.jid)
	) SELECT jid,name,avatar_path,phone_jid FROM candidates
	WHERE phone_jid<>'' AND (name LIKE ? ESCAPE '\' OR phone_jid LIKE ? ESCAPE '\')
	ORDER BY name COLLATE NOCASE, jid LIMIT 100`, like, phoneLike)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.Contact, 0)
	for rows.Next() {
		var contact model.Contact
		var phoneJID string
		if err := rows.Scan(&contact.JID, &contact.Name, &contact.AvatarPath, &phoneJID); err != nil {
			return nil, err
		}
		contact.Phone = "+" + strings.TrimSuffix(phoneJID, "@s.whatsapp.net")
		if contact.Name == "" || contact.Name == contact.JID || contact.Name == strings.SplitN(contact.JID, "@", 2)[0] {
			contact.Name = contact.Phone
		}
		if _, err := model.NewContactCard(contact.Name, contact.Phone); err == nil {
			items = append(items, contact)
		}
	}
	return items, rows.Err()
}
