package model

// ViewOncePlaceholder keeps routing and receipt metadata, never protected
// text, thumbnails, media paths or credentials. Its content stays on the phone.
func ViewOncePlaceholder(m Message) Message {
	return Message{
		ID: m.ID, ChatJID: m.ChatJID, SenderJID: m.SenderJID, SenderName: m.SenderName,
		Timestamp: m.Timestamp, FromMe: m.FromMe, Status: m.Status, Kind: "view_once",
		DeliveredAt: m.DeliveredAt, ReadAt: m.ReadAt, PlayedAt: m.PlayedAt,
		Revoked: m.Revoked, Reactions: m.Reactions,
	}
}
