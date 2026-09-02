package service

// MethodDescription is returned by rpc.discover so shell tools and bots can
// inspect the local API without scraping documentation.
type MethodDescription struct {
	Name     string         `json:"name"`
	Summary  string         `json:"summary"`
	Mutating bool           `json:"mutating"`
	Params   map[string]any `json:"params_example"`
}

func method(name, summary string, mutating bool, params map[string]any) MethodDescription {
	if params == nil {
		params = map[string]any{}
	}
	return MethodDescription{Name: name, Summary: summary, Mutating: mutating, Params: params}
}

var apiMethods = []MethodDescription{
	method("rpc.discover", "List protocol methods and event names", false, nil),
	method("status.get", "Get connection and login state", false, nil),
	method("connection.connect", "Connect the linked device", true, nil),
	method("connection.disconnect", "Disconnect without unlinking", true, nil),
	method("pairing.start", "Start QR-code pairing", true, nil),
	method("pairing.phone", "Request a phone pairing code", true, map[string]any{"phone": "15551234567"}),
	method("account.logout", "Unlink this profile and delete its device session", true, nil),
	method("chats.list", "List or search chats", false, map[string]any{"limit": 100, "offset": 0, "query": "", "archived": false}),
	method("chats.archived_count", "Count archived chats", false, nil),
	method("chats.unread_count", "Count unread messages in visible chats", false, nil),
	method("chat.info", "Get contact metadata and shared-content counts", false, map[string]any{"chat_jid": "123@lid"}),
	method("chat.shared", "Page through a chat's media, documents, or links", false, map[string]any{"chat_jid": "123@lid", "category": "media", "offset": 0, "limit": 60}),
	method("chat.pin", "Pin or unpin a chat", true, map[string]any{"chat_jid": "123@lid", "value": true}),
	method("chat.mute", "Mute or unmute a chat", true, map[string]any{"chat_jid": "123@lid", "value": true}),
	method("chat.archive", "Archive or restore a chat", true, map[string]any{"chat_jid": "123@lid", "value": true}),
	method("chat.set_read", "Mark a chat read or unread across linked devices", true, map[string]any{"chat_jid": "123@lid", "value": true}),
	method("chat.read", "Send receipts and clear the local unread count", true, map[string]any{"chat_jid": "123@lid", "sender_jid": "456@lid", "message_ids": []string{"MESSAGE_ID"}, "timestamp": 0}),
	method("chat.typing", "Set composing or paused presence", true, map[string]any{"chat_jid": "123@lid", "typing": true}),
	method("contact.presence.subscribe", "Subscribe to a contact's online and last-seen updates", true, map[string]any{"chat_jid": "123@lid"}),
	method("chat.avatar", "Fetch and cache a chat avatar", true, map[string]any{"chat_jid": "123@lid", "refresh": false}),
	method("statuses.list", "List active status stories grouped by sender", false, nil),
	method("calls.list", "List synchronized call records", false, nil),
	method("channels.list", "List followed channels", false, nil),
	method("communities.list", "List joined communities", false, nil),
	method("messages.list", "Page through one chat's messages", false, map[string]any{"chat_jid": "123@lid", "before": 0, "limit": 50}),
	method("messages.search", "Search local message text", false, map[string]any{"query": "invoice", "limit": 50}),
	method("link.preview", "Resolve Open Graph metadata for composer text", false, map[string]any{"text": "https://example.com"}),
	method("history.request", "Request an older history page", true, map[string]any{"chat_jid": "123@lid", "limit": 50}),
	method("history.refresh", "Request a recent history page", true, map[string]any{"chat_jid": "123@lid", "limit": 50}),
	method("message.download", "Download and cache message media", true, map[string]any{"chat_jid": "123@lid", "message_id": "MESSAGE_ID"}),
	method("message.send", "Send a text message", true, map[string]any{"chat_jid": "123@lid", "text": "Hello", "reply_to": "", "reply_chat_jid": "", "link_preview": map[string]any{}}),
	method("message.send_media", "Send a file, image, video, audio, or voice note", true, map[string]any{"chat_jid": "123@lid", "path": "/absolute/file", "caption": "", "reply_to": "", "voice": false}),
	method("message.react", "Add or remove a reaction", true, map[string]any{"chat_jid": "123@lid", "message_id": "MESSAGE_ID", "sender_jid": "", "emoji": "👍"}),
	method("message.pin", "Pin a message for 24 hours, 7 days, or 30 days", true, map[string]any{"chat_jid": "123@lid", "message_id": "MESSAGE_ID", "sender_jid": "", "duration_seconds": 604800}),
	method("message.unpin", "Remove the pinned message from a chat", true, map[string]any{"chat_jid": "123@lid", "message_id": "MESSAGE_ID", "sender_jid": ""}),
	method("message.edit", "Edit a sent text message", true, map[string]any{"chat_jid": "123@lid", "message_id": "MESSAGE_ID", "text": "Corrected"}),
	method("message.delete", "Delete a message for everyone", true, map[string]any{"chat_jid": "123@lid", "message_id": "MESSAGE_ID", "sender_jid": ""}),
	method("contact.resolve", "Resolve a phone number to a WhatsApp chat", true, map[string]any{"phone": "15551234567"}),
	method("contact.save", "Resolve a phone number and save a local WhatsAppGo label", true, map[string]any{"phone": "15551234567", "name": "Alice"}),
}

var apiEvents = []string{
	"call.upsert", "calls.synced", "chat.presence", "chat.updated",
	"connection.changed", "contact.presence", "daemon.error", "directory.synced",
	"history.collected", "history.synced", "media.collected", "message.edited",
	"message.pinned", "message.reaction", "message.receipt", "message.revoked", "message.upsert",
	"notification.received", "pairing.error", "pairing.qr", "pairing.state",
	"pairing.success",
}

func discoveryResult() map[string]any {
	return map[string]any{
		"protocol_version": 1,
		"transport":        "newline-delimited JSON over an owner-only Unix socket",
		"methods":          apiMethods,
		"events":           apiEvents,
	}
}
